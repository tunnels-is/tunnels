package wgserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tunnels-is/tunnels/types"
)

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

func TestFetchMesh(t *testing.T) {
	want := types.WGMeshResponse{Peers: []types.WGMeshPeer{
		{PublicKeyHex: "deadbeef", Endpoint: "2.2.2.2:51821", AllowedSubnets: []string{"10.3.0.0/16"}},
	}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wg/mesh" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("X-WG-KEY"); got != "k" {
			t.Errorf("missing X-WG-KEY, got %q", got)
		}
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	cfg := &Config{ControllerURL: srv.URL, APIKey: "k"}
	initSyncClient(cfg)

	got, err := fetchMesh(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Peers) != 1 || got.Peers[0].Endpoint != "2.2.2.2:51821" {
		t.Fatalf("unexpected mesh response: %+v", got)
	}
}
