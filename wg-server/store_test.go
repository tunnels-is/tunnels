package wgserver

import "testing"

// W5: nextIP allocates the lowest free address, reusing addresses freed by
// revocation instead of always incrementing past the max (which would exhaust
// the subnet prematurely under churn).
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
	// Base+2 onward: .2, .3, .4
	if ipA != "10.0.0.2" || ipB != "10.0.0.3" || ipC != "10.0.0.4" {
		t.Fatalf("unexpected initial allocation: %s %s %s", ipA, ipB, ipC)
	}

	// Revoke the middle peer, freeing 10.0.0.3.
	ps.DeleteByPubKey("keyB")

	// The next allocation must reuse the freed .3, not jump to .5.
	ipD, _, err := ps.GetOrAssign("devD", "keyD")
	if err != nil {
		t.Fatal(err)
	}
	if ipD != "10.0.0.3" {
		t.Fatalf("expected freed address 10.0.0.3 to be reused, got %s", ipD)
	}
}
