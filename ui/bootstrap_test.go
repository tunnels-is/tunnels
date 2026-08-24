package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"github.com/tunnels-is/tunnels/client"
)

// TestSetUserLoadsAccountTunnels covers the startup path that left the Tunnels
// page empty: tunnels live in a per-account workspace, so selecting the user has
// to activate that workspace and re-read state before the page is built.
func TestSetUserLoadsAccountTunnels(t *testing.T) {
	base := t.TempDir()
	accounts := filepath.Join(base, "accounts") + string(os.PathSeparator)

	s := client.STATE.Load()
	if s == nil {
		t.Skip("client state not initialised")
	}
	prevBase, prevAccounts := s.BasePath, s.AccountsPath
	t.Cleanup(func() {
		st := client.STATE.Load()
		st.BasePath, st.AccountsPath = prevBase, prevAccounts
		client.STATE.Store(st)
	})
	s.BasePath = base + string(os.PathSeparator)
	s.AccountsPath = accounts
	client.STATE.Store(s)

	const userID = "6f2a91c4e8b7d3a501f9c62b"

	// Create the workspace and drop a tunnel file in it, as a previous session
	// would have left behind.
	if err := client.ActivateAccount(userID); err != nil {
		t.Fatal(err)
	}
	tunnelsDir := client.STATE.Load().TunnelsPath
	meta := &client.TunnelMETA{Tag: "from-disk", IFName: "tunnels", MTU: 1420, ConfigFormat: ".json"}
	blob, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tunnelsDir, "from-disk.json"), blob, 0o600); err != nil {
		t.Fatal(err)
	}

	// Model a fresh process: no workspace activated yet and nothing cached.
	// activateAccountByHash early-returns when the hash is already active, so
	// the hash has to be cleared as well as the map.
	cold := client.STATE.Load()
	cold.ActiveAccountHash = ""
	cold.TunnelsPath = ""
	client.STATE.Store(cold)
	client.TunnelMetaMap.Range(func(k string, _ *client.TunnelMETA) bool {
		client.TunnelMetaMap.Delete(k)
		return true
	})

	fy := test.NewApp()
	a := &App{fyneApp: fy, win: fy.NewWindow("t"), toastBox: container.NewStack()}

	// What bootstrap does: snapshot state, then pick the saved user.
	a.refreshState()
	if len(a.tunnels) != 0 {
		t.Fatalf("precondition: expected an empty cold-start snapshot, got %d", len(a.tunnels))
	}

	a.setUser(&client.User{ID: userID, Email: "sveinn@min.io"})

	if len(a.tunnels) == 0 {
		t.Fatal("setUser left a.tunnels empty; the Tunnels page would show nothing")
	}
	found := false
	for _, tn := range a.tunnels {
		if tn != nil && tn.Tag == "from-disk" {
			found = true
		}
	}
	if !found {
		t.Errorf("tunnel from disk not visible after setUser; got %d tunnels", len(a.tunnels))
	}
}
