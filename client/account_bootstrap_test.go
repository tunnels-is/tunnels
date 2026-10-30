package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tunnels-is/tunnels/types"
)

// TestBootstrapSeesTunnelsOnlyAfterActivation ...
func TestBootstrapSeesTunnelsOnlyAfterActivation(t *testing.T) {
	base := t.TempDir()

	prevState := STATE.Load()
	t.Cleanup(func() { STATE.Store(prevState) })

	accounts := filepath.Join(base, "accounts") + string(os.PathSeparator)

	s := &State{
		BasePath:     base + string(os.PathSeparator),
		AccountsPath: accounts,
		TunnelType:   string(types.DefaultTun),
	}
	STATE.Store(s)

	const userID = "6f2a91c4e8b7d3a501f9c62b"
	hash, err := userIDToAccountHash(userID)
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureAccountDirs(hash); err != nil {
		t.Fatal(err)
	}

	meta := &TunnelMeta{Tag: "default", IFName: "tunnels", MTU: 1420, ConfigFormat: ".json"}
	blob, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(accountTunnelsPath(hash), "default.json"), blob, 0o600); err != nil {
		t.Fatal(err)
	}

	clearTunnelMetaMap()

	if got := len(GetFullState().Tunnels); got != 0 {
		t.Fatalf("precondition: expected no tunnels before activation, got %d", got)
	}

	if err := ActivateAccount(userID); err != nil {
		t.Fatalf("ActivateAccount: %v", err)
	}
	tunnels := GetFullState().Tunnels
	if len(tunnels) == 0 {
		t.Fatal("no tunnels after ActivateAccount; the on-disk tunnel was not loaded")
	}
	found := false
	for _, tn := range tunnels {
		if tn != nil && tn.Tag == "default" {
			found = true
		}
	}
	if !found {
		t.Errorf("tunnel %q missing after activation; got %d tunnels", "default", len(tunnels))
	}
}
