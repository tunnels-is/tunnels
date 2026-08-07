package wgserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tunnels-is/tunnels/types"
)

func startTLSController(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func testControllerCfg(srv *httptest.Server, apiKey string) *Config {
	return &Config{
		ControllerURL:      srv.URL,
		APIKey:             apiKey,
		InsecureSkipVerify: true, // test TLS cert is self-signed
	}
}

func TestQueryPeer_OK(t *testing.T) {
	want := types.WGPeer{
		PublicKeyHex:  "deadbeef",
		DeviceID:      "dev-1",
		WireGuardIP:   "10.0.0.5",
		WireGuardIPv6: "fd00::5",
	}

	srv := startTLSController(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wg/peer" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("pubkey"); got != "abc==" {
			t.Errorf("unexpected pubkey: %q", got)
		}
		if got := r.Header.Get("X-WG-KEY"); got != "test-api-key" {
			t.Errorf("missing or wrong X-WG-KEY header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(want)
	})

	cfg := testControllerCfg(srv, "test-api-key")
	initSyncClient(cfg)

	res, got := queryPeer(cfg, "abc==")
	if res != authAllowed {
		t.Fatalf("expected authAllowed, got %v", res)
	}
	if got == nil || *got != want {
		t.Fatalf("mismatch: got %+v want %+v", got, want)
	}
}

func TestQueryPeer_Denied(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound} {
		srv := startTLSController(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		})
		cfg := testControllerCfg(srv, "k")
		initSyncClient(cfg)

		res, got := queryPeer(cfg, "x")
		if res != authDenied {
			t.Fatalf("status %d: expected authDenied, got %v", code, res)
		}
		if got != nil {
			t.Fatalf("status %d: expected nil peer", code)
		}
	}
}

func TestQueryPeer_ServerErrorIsUnknown(t *testing.T) {
	srv := startTLSController(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	cfg := testControllerCfg(srv, "k")
	initSyncClient(cfg)

	if res, _ := queryPeer(cfg, "x"); res != authUnknown {
		t.Fatalf("5xx should be authUnknown, got %v", res)
	}
}

func TestQueryPeer_QueryEscaping(t *testing.T) {
	var seen string
	srv := startTLSController(t, func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.Query().Get("pubkey")
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := testControllerCfg(srv, "k")
	initSyncClient(cfg)

	raw := "abc/def+ghi=="
	if res, _ := queryPeer(cfg, raw); res != authDenied {
		t.Fatalf("404 should be authDenied, got %v", res)
	}
	if seen != raw {
		t.Fatalf("query decode mismatch: got %q want %q", seen, raw)
	}
}

func TestQueryPeer_HTTPControllerURLRejected(t *testing.T) {
	cfg := &Config{ControllerURL: "http://127.0.0.1:1", APIKey: "k"}
	initSyncClient(cfg)
	if res, _ := queryPeer(cfg, "x"); res != authUnknown {
		t.Fatalf("http:// controller must be rejected, got %v", res)
	}
}

func TestQueryPeer_RedirectNotFollowed(t *testing.T) {
	var evilHits int
	evil := startTLSController(t, func(w http.ResponseWriter, r *http.Request) {
		evilHits++
		if r.Header.Get("X-WG-KEY") != "" {
			t.Error("X-WG-KEY must not be sent to redirect target")
		}
		w.WriteHeader(http.StatusOK)
	})

	good := startTLSController(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, evil.URL+"/wg/peer?pubkey=stolen", http.StatusFound)
	})

	cfg := testControllerCfg(good, "secret-key")
	initSyncClient(cfg)

	res, peer := queryPeer(cfg, "x")
	if res != authUnknown {
		t.Fatalf("redirect should fail closed as authUnknown, got %v peer=%v", res, peer)
	}
	if evilHits != 0 {
		t.Fatalf("redirect target was contacted %d times", evilHits)
	}
}
