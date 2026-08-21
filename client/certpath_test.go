package client

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tunnels-is/tunnels/certs"
)

func TestTLSConfigForController_SkipVerifyIgnoresPath(t *testing.T) {
	cfg, err := tlsConfigForController(false, "/no/such/cert.pem")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.InsecureSkipVerify {
		t.Fatal("ValidateCertificate=false must skip verify")
	}
	if cfg.RootCAs != nil {
		t.Fatal("skip-verify must not load RootCAs")
	}
}

func TestTLSConfigForController_SystemRootsWhenNoPath(t *testing.T) {
	cfg, err := tlsConfigForController(true, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.InsecureSkipVerify {
		t.Fatal("ValidateCertificate=true must verify")
	}
	if cfg.RootCAs != nil {
		t.Fatal("empty path should use system roots")
	}
}

func TestTLSConfigForController_MissingFileFailsClosed(t *testing.T) {
	_, err := tlsConfigForController(true, filepath.Join(t.TempDir(), "missing.pem"))
	if err == nil {
		t.Fatal("missing certificate path must fail")
	}
}

func TestTLSConfigForController_InvalidPEMFailsClosed(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(p, []byte("not a cert"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := tlsConfigForController(true, p)
	if err == nil {
		t.Fatal("invalid PEM must fail")
	}
}

func TestTLSConfigForController_CustomRootVerifiesSelfSigned(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if _, err := certs.MakeCertV2(certs.ECDSA, certPath, keyPath, []string{"127.0.0.1"}, []string{"localhost"}, "", time.Time{}, true); err != nil {
		t.Fatal(err)
	}
	pair, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{pair},
		MinVersion:   tls.VersionTLS13,
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	body, code, err := SendRequestToURL(nil, "GET", srv.URL, nil, 5000, true, certPath)
	if err != nil {
		t.Fatalf("verify with CertificatePath: %v", err)
	}
	if code != 200 {
		t.Fatalf("code=%d body=%s", code, body)
	}

	_, _, err = SendRequestToURL(nil, "GET", srv.URL, nil, 5000, true, "")
	if err == nil {
		t.Fatal("system roots must reject the self-signed controller cert")
	}
}
