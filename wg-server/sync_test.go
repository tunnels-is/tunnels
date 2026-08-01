package wgserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tunnels-is/tunnels/types"
)

func TestQueryPeer_OK(t *testing.T) {
	want := types.WGPeer{
		PublicKeyHex:  "deadbeef",
		DeviceID:      "dev-1",
		WireGuardIP:   "10.0.0.5",
		WireGuardIPv6: "fd00::5",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	}))
	defer srv.Close()

	cfg := &Config{ControllerURL: srv.URL, APIKey: "test-api-key"}
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
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		}))
		cfg := &Config{ControllerURL: srv.URL, APIKey: "k"}
		initSyncClient(cfg)

		res, got := queryPeer(cfg, "x")
		srv.Close()
		if res != authDenied {
			t.Fatalf("status %d: expected authDenied, got %v", code, res)
		}
		if got != nil {
			t.Fatalf("status %d: expected nil peer", code)
		}
	}
}

func TestQueryPeer_ServerErrorIsUnknown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := &Config{ControllerURL: srv.URL, APIKey: "k"}
	initSyncClient(cfg)

	if res, _ := queryPeer(cfg, "x"); res != authUnknown {
		t.Fatalf("5xx should be authUnknown, got %v", res)
	}
}

func TestQueryPeer_QueryEscaping(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.Query().Get("pubkey")
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	cfg := &Config{ControllerURL: srv.URL, APIKey: "k"}
	initSyncClient(cfg)

	raw := "abc/def+ghi=="
	if res, _ := queryPeer(cfg, raw); res != authDenied {
		t.Fatalf("404 should be authDenied, got %v", res)
	}
	if seen != raw {
		t.Fatalf("query decode mismatch: got %q want %q", seen, raw)
	}
}
