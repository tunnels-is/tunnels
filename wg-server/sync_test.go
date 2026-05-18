package wgserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tunnels-is/tunnels/types"
)

func TestFetchPeerByPubKey_OK(t *testing.T) {
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

	got, err := fetchPeerByPubKey(cfg, "abc==")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected peer, got nil")
	}
	if *got != want {
		t.Fatalf("mismatch: got %+v want %+v", *got, want)
	}
}

func TestFetchPeerByPubKey_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	cfg := &Config{ControllerURL: srv.URL, APIKey: "k"}
	initSyncClient(cfg)

	got, err := fetchPeerByPubKey(cfg, "missing")
	if err != nil {
		t.Fatalf("404 should be (nil, nil), got err: %v", err)
	}
	if got != nil {
		t.Fatal("404 should produce nil peer")
	}
}

func TestFetchPeerByPubKey_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := &Config{ControllerURL: srv.URL, APIKey: "k"}
	initSyncClient(cfg)

	if _, err := fetchPeerByPubKey(cfg, "x"); err == nil {
		t.Fatal("expected error on 5xx")
	}
}

func TestFetchPeerByPubKey_QueryEscaping(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.Query().Get("pubkey")
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	cfg := &Config{ControllerURL: srv.URL, APIKey: "k"}
	initSyncClient(cfg)

	raw := "abc/def+ghi=="
	if _, err := fetchPeerByPubKey(cfg, raw); err != nil {
		t.Fatal(err)
	}
	if seen != raw {
		t.Fatalf("query decode mismatch: got %q want %q", seen, raw)
	}
}
