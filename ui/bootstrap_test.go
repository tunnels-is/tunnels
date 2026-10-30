package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"github.com/tunnels-is/tunnels/client"
)

// TestSetUserLoadsAccountTunnels ...
func TestSetUserLoadsAccountTunnels(t *testing.T) {
	base := t.TempDir()
	accounts := filepath.Join(base, "accounts") + string(os.PathSeparator)

	s := client.STATE.Load()
	if s == nil {
		t.Skip("client state not initialised")
	}
	prevBase, prevAccounts := s.BasePath, s.AccountsPath
	t.Cleanup(func() {
		waitForAccountFile(t, accounts)
		st := client.STATE.Load()
		st.BasePath, st.AccountsPath = prevBase, prevAccounts
		client.STATE.Store(st)
	})
	s.BasePath = base + string(os.PathSeparator)
	s.AccountsPath = accounts
	client.STATE.Store(s)

	const userID = "6f2a91c4e8b7d3a501f9c62b"

	if err := client.ActivateAccount(userID); err != nil {
		t.Fatal(err)
	}
	tunnelsDir := client.STATE.Load().TunnelsPath
	meta := &client.TunnelMeta{Tag: "from-disk", IFName: "tunnels", MTU: 1420, ConfigFormat: ".json"}
	blob, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tunnelsDir, "from-disk.json"), blob, 0o600); err != nil {
		t.Fatal(err)
	}

	cold := client.STATE.Load()
	cold.ActiveAccountHash = ""
	cold.TunnelsPath = ""
	client.STATE.Store(cold)
	client.TunnelMetaMap.Range(func(k string, _ *client.TunnelMeta) bool {
		client.TunnelMetaMap.Delete(k)
		return true
	})

	fy := test.NewApp()
	a := &App{fyneApp: fy, win: fy.NewWindow("t"), toastBox: container.NewStack()}

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

func waitForAccountFile(t *testing.T, accounts string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		found := false
		_ = filepath.Walk(accounts, func(path string, info os.FileInfo, err error) error {
			if err == nil && info != nil && !info.IsDir() && filepath.Base(path) == "user" {
				found = true
			}
			return nil
		})
		if found {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Log("timed out waiting for the account file; the save goroutine may still be in flight")
}
