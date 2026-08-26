package client

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCanonicalizePprofAddr(t *testing.T) {
	got, err := canonicalizePprofAddr("")
	if err != nil || got != defaultPprofAddr {
		t.Fatalf("empty: got %q %v, want %s", got, err, defaultPprofAddr)
	}
	got, err = canonicalizePprofAddr("localhost:7090")
	if err != nil || got != "127.0.0.1:7090" {
		t.Fatalf("localhost: got %q %v", got, err)
	}
	got, err = canonicalizePprofAddr("[::1]:6060")
	if err != nil || got != "[::1]:6060" {
		t.Fatalf("ipv6 loopback: got %q %v", got, err)
	}
	if _, err := canonicalizePprofAddr("0.0.0.0:6060"); err == nil {
		t.Fatal("public bind must be rejected")
	}
	if _, err := canonicalizePprofAddr("192.168.1.9:6060"); err == nil {
		t.Fatal("lan bind must be rejected")
	}
}

func TestPprofMuxServesHeap(t *testing.T) {
	srv := httptest.NewServer(pprofMux())
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/debug/pprof/heap")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) < 16 {
		t.Fatalf("heap profile too short: %d bytes", len(body))
	}
}
