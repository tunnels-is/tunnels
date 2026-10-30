package client

import "testing"

// TestDirectKeyAllowed ...
func TestDirectKeyAllowed(t *testing.T) {
	var fresh, hub, punchKey [32]byte
	fresh[0] = 1
	hub[0] = 2
	punchKey[0] = 3
	ipc := "public_key=0200000000000000000000000000000000000000000000000000000000000000\nallowed_ip=0.0.0.0/0\npublic_key=0300000000000000000000000000000000000000000000000000000000000000\nallowed_ip=10.0.1.8/32\n"
	if !directKeyAllowed(ipc, false, [32]byte{}, fresh) {
		t.Fatal("a new key was refused")
	}
	if directKeyAllowed(ipc, false, [32]byte{}, hub) {
		t.Fatal("hub key was accepted")
	}
	if !directKeyAllowed(ipc, true, punchKey, punchKey) {
		t.Fatal("refresh of the punch key for this address was refused")
	}
	if directKeyAllowed(ipc, true, punchKey, hub) {
		t.Fatal("hub key was accepted over an existing punch peer")
	}
	if directKeyAllowed("public_key=0300000000000000000000000000000000000000000000000000000000000000\n", false, [32]byte{}, punchKey) {
		t.Fatal("an installed punch key was reused for a different address")
	}
}
