package wgserver

import "testing"

func TestNextIP_ReusesFreedAddress(t *testing.T) {
	ps := NewPeerStore("10.0.0.0/24", "")

	ipA, _, err := ps.GetOrAssign("devA", "keyA")
	if err != nil {
		t.Fatal(err)
	}
	ipB, _, err := ps.GetOrAssign("devB", "keyB")
	if err != nil {
		t.Fatal(err)
	}
	ipC, _, err := ps.GetOrAssign("devC", "keyC")
	if err != nil {
		t.Fatal(err)
	}

	if ipA != "10.0.0.2" || ipB != "10.0.0.3" || ipC != "10.0.0.4" {
		t.Fatalf("unexpected initial allocation: %s %s %s", ipA, ipB, ipC)
	}

	ps.DeleteByPubKey("keyB")

	ipD, _, err := ps.GetOrAssign("devD", "keyD")
	if err != nil {
		t.Fatal(err)
	}
	if ipD != "10.0.0.3" {
		t.Fatalf("expected freed address 10.0.0.3 to be reused, got %s", ipD)
	}
}
