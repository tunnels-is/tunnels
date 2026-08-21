package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tunnels-is/tunnels/certs"
	"github.com/tunnels-is/tunnels/crypt"
)

func securityReviewLogger() {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
}

func TestGenerateSelfSigned_MatchesAPILoader(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	ip := "10.99.7.2"

	// Same call generateSelfSignedCerts uses (empty DNS SAN, IP override).
	_, err := certs.MakeCertV2(
		certs.ECDSA,
		certPath,
		keyPath,
		[]string{ip},
		[]string{""},
		"",
		time.Time{},
		true,
	)
	if err != nil {
		t.Fatalf("MakeCertV2: %v", err)
	}

	_, privB, err := crypt.LoadPrivateKey(keyPath)
	if err != nil {
		t.Fatalf("LoadPrivateKey: %v", err)
	}
	_, pubB, err := crypt.LoadPublicKey(certPath)
	if err != nil {
		t.Fatalf("LoadPublicKey: %v", err)
	}
	pair, err := tls.X509KeyPair(pubB, privB)
	if err != nil {
		t.Fatalf("X509KeyPair (API load path): %v", err)
	}
	if len(pair.Certificate) == 0 {
		t.Fatal("empty certificate")
	}
	parsed, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range parsed.IPAddresses {
		if a.String() == ip {
			found = true
		}
	}
	if !found {
		t.Fatalf("cert SAN IPs = %v, want %s", parsed.IPAddresses, ip)
	}
	if parsed.PublicKeyAlgorithm != x509.ECDSA {
		t.Fatalf("algo = %s, want ECDSA", parsed.PublicKeyAlgorithm)
	}
}

func TestIsValidDomain_LetsEncryptGate(t *testing.T) {
	ok := []string{"example.com", "vpn.example.com", "a-b.example.co.uk"}
	for _, d := range ok {
		if !isValidDomain(d) {
			t.Errorf("%q should be valid", d)
		}
	}
	bad := []string{"", "selfsign", "localhost", "10.0.0.1", "no-dot", "-bad.com", "bad-.com", "has space.com"}
	for _, d := range bad {
		if isValidDomain(d) {
			t.Errorf("%q should be rejected", d)
		}
	}
}

func TestGenerateLetsEncrypt_RejectsInvalidDomain(t *testing.T) {
	securityReviewLogger()
	if err := generateLetsEncryptCerts(context.Background(), ""); err == nil {
		t.Fatal("empty domain must fail before ACME")
	}
	if err := generateLetsEncryptCerts(context.Background(), "selfsign"); err == nil {
		t.Fatal("selfsign must not be treated as a domain")
	}
}

func TestMakeCertV2_DoesNotOverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, []byte("keep-me"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := certs.MakeCertV2(certs.ECDSA, certPath, keyPath, []string{"127.0.0.1"}, nil, "", time.Time{}, true)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(certPath)
	if string(got) != "keep-me" {
		t.Fatal("MakeCertV2 overwrote an existing cert.pem")
	}
}
