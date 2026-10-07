package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppIDIsDesktopFileStem(t *testing.T) {
	if strings.HasSuffix(appID, ".desktop") {
		t.Fatalf("appID %q ends in .desktop; the compositor shows that word as the application name", appID)
	}
	if appID != "is.tunnels" {
		t.Fatalf("appID = %q, desktop file is is.tunnels.desktop", appID)
	}
}

func TestMigrateFyneConfigDir(t *testing.T) {
	root := t.TempDir()
	oldPath := filepath.Join(root, legacyAppID)
	newPath := filepath.Join(root, appID)
	if err := os.Mkdir(oldPath, 0o755); err != nil {
		t.Fatal(err)
	}
	pref := filepath.Join(oldPath, "preferences.json")
	if err := os.WriteFile(pref, []byte(`{"ui-theme":"dark"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := migrateFyneConfigDir(root, legacyAppID, appID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(newPath, "preferences.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old config dir still present: %v", err)
	}

	// A second launch must not move a directory back over settings that
	// already live under the new id.
	if err := os.Mkdir(oldPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldPath, "preferences.json"), []byte(`{"ui-theme":"stale"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := migrateFyneConfigDir(root, legacyAppID, appID); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(newPath, "preferences.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"ui-theme":"dark"}` {
		t.Fatalf("new settings overwritten: %s", got)
	}
}
