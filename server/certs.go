package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackpal/gateway"
	"github.com/tunnels-is/tunnels/certs"
	"golang.org/x/crypto/acme"
)

const letsEncryptDirectoryURL = "https://acme-v02.api.letsencrypt.org/directory"

func executableDir() (string, error) {
	ep, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(ep), nil
}

func resolveInterfaceIP(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	ip, err := gateway.DiscoverInterface()
	if err != nil {
		return "", err
	}
	return ip.String(), nil
}

func generateSelfSignedCerts(ipOverride string) error {
	dir, err := executableDir()
	if err != nil {
		return err
	}
	ip, err := resolveInterfaceIP(ipOverride)
	if err != nil {
		return err
	}
	_, err = certs.MakeCertV2(
		certs.ECDSA,
		filepath.Join(dir, "cert.pem"),
		filepath.Join(dir, "key.pem"),
		[]string{ip},
		[]string{""},
		"",
		time.Time{},
		true,
	)
	return err
}

func generateLetsEncryptCerts(ctx context.Context, domain string) error {
	if !isValidDomain(domain) {
		return fmt.Errorf("invalid domain %q", domain)
	}
	dir, err := executableDir()
	if err != nil {
		return err
	}
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	accountKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate ACME account key: %w", err)
	}

	client := &acme.Client{
		Key:          accountKey,
		DirectoryURL: letsEncryptDirectoryURL,
	}

	if _, err := client.Register(ctx, &acme.Account{}, acme.AcceptTOS); err != nil {
		if !errors.Is(err, acme.ErrAccountAlreadyExists) {
			return fmt.Errorf("ACME account registration: %w", err)
		}
	}

	order, err := client.AuthorizeOrder(ctx, acme.DomainIDs(domain))
	if err != nil {
		return fmt.Errorf("authorize order: %w", err)
	}

	srv, errCh, err := startHTTP01Server(client, order)
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		select {
		case serveErr := <-errCh:
			if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
				logger.Warn("ACME challenge server stopped with error", slog.Any("err", serveErr))
			}
		default:
		}
	}()

	for _, authzURL := range order.AuthzURLs {
		if err := completeHTTP01Authorization(ctx, client, authzURL); err != nil {
			return err
		}
	}

	finalOrder, err := client.WaitOrder(ctx, order.URI)
	if err != nil {
		return fmt.Errorf("wait order ready: %w", err)
	}

	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate cert key: %w", err)
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:  pkix.Name{CommonName: domain},
		DNSNames: []string{domain},
	}, certKey)
	if err != nil {
		return fmt.Errorf("create CSR: %w", err)
	}

	chain, _, err := client.CreateOrderCert(ctx, finalOrder.FinalizeURL, csrDER, true)
	if err != nil {
		return fmt.Errorf("finalize order: %w", err)
	}

	if err := writeCertChain(certPath, chain); err != nil {
		return err
	}
	if err := writeECKey(keyPath, certKey); err != nil {
		return err
	}

	logger.Info("Let's Encrypt certificate issued", "domain", domain, "cert", certPath, "key", keyPath)
	return nil
}

func startHTTP01Server(client *acme.Client, order *acme.Order) (*http.Server, chan error, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/acme-challenge/", func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.URL.Path, "/.well-known/acme-challenge/")
		resp, err := client.HTTP01ChallengeResponse(token)
		if err != nil {
			http.Error(w, "challenge response error", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(resp))
	})

	ln, err := net.Listen("tcp", ":80")
	if err != nil {
		return nil, nil, fmt.Errorf("listen on :80 for ACME HTTP-01 challenge: %w", err)
	}

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()
	logger.Info("ACME HTTP-01 challenge server listening on :80")
	return srv, errCh, nil
}

func completeHTTP01Authorization(ctx context.Context, client *acme.Client, authzURL string) error {
	authz, err := client.GetAuthorization(ctx, authzURL)
	if err != nil {
		return fmt.Errorf("get authorization: %w", err)
	}
	if authz.Status == acme.StatusValid {
		return nil
	}
	var challenge *acme.Challenge
	for _, c := range authz.Challenges {
		if c.Type == "http-01" {
			challenge = c
			break
		}
	}
	if challenge == nil {
		return fmt.Errorf("no http-01 challenge offered for %s", authz.Identifier.Value)
	}
	if _, err := client.Accept(ctx, challenge); err != nil {
		return fmt.Errorf("accept challenge: %w", err)
	}
	if _, err := client.WaitAuthorization(ctx, authz.URI); err != nil {
		return fmt.Errorf("wait authorization: %w", err)
	}
	return nil
}

func writeCertChain(path string, chain [][]byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, der := range chain {
		if err := pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
			return fmt.Errorf("encode certificate: %w", err)
		}
	}
	return nil
}

func writeECKey(path string, key *ecdsa.PrivateKey) error {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	// 0600: a TLS private key must not be world-readable (os.Create → 0644).
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	return pem.Encode(f, &pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

func isValidDomain(s string) bool {
	if s == "" || len(s) > 253 {
		return false
	}
	if net.ParseIP(s) != nil {
		return false
	}
	if !strings.Contains(s, ".") {
		return false
	}
	for _, label := range strings.Split(s, ".") {
		if label == "" || len(label) > 63 {
			return false
		}
		for i, r := range label {
			isAlnum := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
			if !isAlnum && r != '-' {
				return false
			}
			if r == '-' && (i == 0 || i == len(label)-1) {
				return false
			}
		}
	}
	return true
}
