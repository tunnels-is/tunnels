package wgserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tunnels-is/tunnels/types"
)

// TestSameMeshPeer ...
func TestSameMeshPeer(t *testing.T) {
	base := installedMeshPeer{
		PublicKeyHex: "aa",
		Endpoint:     "1.2.3.4:51821",
		Subnets:      []string{"10.3.0.0/16"},
	}
	want := types.WGMeshPeer{
		PublicKeyHex:   "aa",
		Endpoint:       "1.2.3.4:51821",
		AllowedSubnets: []string{"10.3.0.0/16"},
	}
	if !sameMeshPeer(base, want) {
		t.Fatal("identical peer should compare equal")
	}

	changedEndpoint := want
	changedEndpoint.Endpoint = "9.9.9.9:51821"
	if sameMeshPeer(base, changedEndpoint) {
		t.Fatal("endpoint change should not compare equal")
	}

	changedSubnets := want
	changedSubnets.AllowedSubnets = []string{"10.3.0.0/16", "fd00::/64"}
	if sameMeshPeer(base, changedSubnets) {
		t.Fatal("subnet change should not compare equal")
	}

	twoSubnets := installedMeshPeer{
		PublicKeyHex: "aa",
		Endpoint:     "1.2.3.4:51821",
		Subnets:      []string{"10.3.0.0/16", "10.4.0.0/16"},
	}
	reordered := types.WGMeshPeer{
		PublicKeyHex:   "aa",
		Endpoint:       "1.2.3.4:51821",
		AllowedSubnets: []string{"10.4.0.0/16", "10.3.0.0/16"},
	}
	if !sameMeshPeer(twoSubnets, reordered) {
		t.Fatal("reordered subnets should compare equal")
	}
	dupSubnet := types.WGMeshPeer{
		PublicKeyHex:   "aa",
		Endpoint:       "1.2.3.4:51821",
		AllowedSubnets: []string{"10.3.0.0/16", "10.3.0.0/16"},
	}
	if sameMeshPeer(twoSubnets, dupSubnet) {
		t.Fatal("[X,Y] vs [X,X] must not compare equal")
	}
}

// TestValidMeshSubnet ...
func TestValidMeshSubnet(t *testing.T) {
	cases := map[string]bool{
		"10.3.0.0/16": true,
		"10.4.0.5/32": true,
		"fd00::/64":   true,
		"0.0.0.0/0":   false,
		"::/0":        false,
		"not-a-cidr":  false,
		"10.0.0.1":    false,
		"":            false,
	}
	for in, want := range cases {
		if got := validMeshSubnet(in); got != want {
			t.Errorf("validMeshSubnet(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestFetchMesh ...
func TestFetchMesh(t *testing.T) {
	want := types.WGMeshResponse{Peers: []types.WGMeshPeer{
		{PublicKeyHex: "deadbeef", Endpoint: "2.2.2.2:51821", AllowedSubnets: []string{"10.3.0.0/16"}},
	}}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wg/mesh" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("X-WG-KEY"); got != "k" {
			t.Errorf("missing X-WG-KEY, got %q", got)
		}
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	cfg := &Config{ControllerURL: srv.URL, APIKey: "k", InsecureSkipVerify: true}
	initSyncClient(cfg)

	got, err := fetchMesh(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Peers) != 1 || got.Peers[0].Endpoint != "2.2.2.2:51821" {
		t.Fatalf("unexpected mesh response: %+v", got)
	}
}

// TestFetchMesh_HTTPRejected ...
func TestFetchMesh_HTTPRejected(t *testing.T) {
	cfg := &Config{ControllerURL: "http://127.0.0.1:1", APIKey: "k"}
	if _, err := fetchMesh(cfg); err == nil {
		t.Fatal("expected error for http:// controller URL")
	}
}

// TestValidMeshSubnet_SharedRules checks validMeshSubnet now enforces the
// shared types.ValidateMeshSubnet rules (A-14), not just "not /0".
func TestValidMeshSubnet_SharedRules(t *testing.T) {
	for _, in := range []string{
		"0.0.0.0/1", "128.0.0.0/1", "10.0.0.0/7", "2000::/3",
		"10.3.0.1/16", "127.0.0.0/8", "::ffff:10.0.0.0/104",
	} {
		if validMeshSubnet(in) {
			t.Errorf("validMeshSubnet(%q) = true, want false", in)
		}
	}
}

// TestDesiredMeshPeers covers the hub-side filtering of a controller mesh
// response: invalid/overbroad subnets, overlap with the hub's own subnets,
// and cross-peer overlap (both claims dropped).
func TestDesiredMeshPeers(t *testing.T) {
	own := []string{"10.0.0.1/22", "fd00::/64"}
	resp := &types.WGMeshResponse{Peers: []types.WGMeshPeer{
		{PublicKeyHex: "", Endpoint: "9.9.9.9:1", AllowedSubnets: []string{"10.9.0.0/16"}},
		{PublicKeyHex: "aa", Endpoint: "1.1.1.1:51821", AllowedSubnets: []string{"10.3.0.0/16", "fd00:3::/64"}},
		{PublicKeyHex: "bb", Endpoint: "2.2.2.2:51821", AllowedSubnets: []string{"0.0.0.0/0", "0.0.0.0/1"}},
		{PublicKeyHex: "cc", Endpoint: "3.3.3.3:51821", AllowedSubnets: []string{"10.0.1.0/24", "10.5.0.0/16"}},
		{PublicKeyHex: "dd", Endpoint: "4.4.4.4:51821", AllowedSubnets: []string{"fd00::/48"}},
		{PublicKeyHex: "ee", Endpoint: "5.5.5.5:51821", AllowedSubnets: []string{"10.6.0.0/16"}},
		{PublicKeyHex: "ff", Endpoint: "6.6.6.6:51821", AllowedSubnets: []string{"10.6.128.0/17", "10.7.0.0/16"}},
	}}
	got := desiredMeshPeers(resp, own)

	want := map[string][]string{
		"aa": {"10.3.0.0/16", "fd00:3::/64"},
		"cc": {"10.5.0.0/16"},
		"ff": {"10.7.0.0/16"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d peers %+v, want %v", len(got), got, want)
	}
	for k, subnets := range want {
		p, ok := got[k]
		if !ok {
			t.Fatalf("missing peer %s in %+v", k, got)
		}
		if len(p.AllowedSubnets) != len(subnets) {
			t.Fatalf("peer %s subnets %v, want %v", k, p.AllowedSubnets, subnets)
		}
		for i := range subnets {
			if p.AllowedSubnets[i] != subnets[i] {
				t.Fatalf("peer %s subnets %v, want %v", k, p.AllowedSubnets, subnets)
			}
		}
	}
}
