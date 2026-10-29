package setcap

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

type fakeInfo struct {
	mode os.FileMode
	uid  uint32
	sys  any
}

func (f fakeInfo) Name() string       { return "tunnels" }
func (f fakeInfo) Size() int64        { return 1 }
func (f fakeInfo) Mode() os.FileMode  { return f.mode }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeInfo) Sys() any {
	if f.sys != nil {
		return f.sys
	}
	return &syscall.Stat_t{Uid: f.uid}
}

func TestValidateCapabilityInfo(t *testing.T) {
	rootFile := fakeInfo{mode: 0o755, uid: 0}
	rootDir := fakeInfo{mode: os.ModeDir | 0o755, uid: 0}
	if err := validateCapabilityInfo(rootFile, rootDir); err != nil {
		t.Fatal(err)
	}
	if err := validateCapabilityInfo(fakeInfo{mode: 0o755, uid: 1000}, rootDir); err == nil {
		t.Fatal("user-owned binary was accepted")
	}
	if err := validateCapabilityInfo(fakeInfo{mode: 0o775, uid: 0}, rootDir); err == nil {
		t.Fatal("group-writable binary was accepted")
	}
	if err := validateCapabilityInfo(fakeInfo{mode: 0o777, uid: 0}, rootDir); err == nil {
		t.Fatal("world-writable binary was accepted")
	}
	if err := validateCapabilityInfo(rootFile, fakeInfo{mode: os.ModeDir | 0o777, uid: 0}); err == nil {
		t.Fatal("world-writable directory was accepted")
	}
	if err := validateCapabilityInfo(rootFile, fakeInfo{mode: os.ModeDir | 0o755, uid: 1000}); err == nil {
		t.Fatal("user-owned directory was accepted")
	}
	if err := validateCapabilityInfo(fakeInfo{mode: os.ModeSymlink | 0o755, uid: 0}, rootDir); err == nil {
		t.Fatal("symlink was accepted")
	}
	if err := validateCapabilityInfo(fakeInfo{mode: 0o755, sys: "no-stat"}, rootDir); err == nil {
		t.Fatal("missing owner was accepted")
	}
}

func TestValidateCapabilityTarget_UserOwnedTree(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tunnels")
	if err := os.WriteFile(path, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := validateCapabilityTarget(path); err == nil {
		t.Fatal("user-owned tree was accepted")
	}
}

func TestClearSudoTimestampRunsCommand(t *testing.T) {
	var got []string
	prev := runCommand
	runCommand = func(name string, args ...string) error {
		got = append([]string{name}, args...)
		return nil
	}
	t.Cleanup(func() { runCommand = prev })

	clearSudoTimestamp()
	if len(got) != 2 || got[0] != "sudo" || got[1] != "-kK" {
		t.Fatalf("command = %q", got)
	}
}

func TestQuotedPath(t *testing.T) {
	if quotedPath("") != "'[BINARY]'" {
		t.Fatal(quotedPath(""))
	}
	if quotedPath("/usr/bin/tunnels") != "'/usr/bin/tunnels'" {
		t.Fatal(quotedPath("/usr/bin/tunnels"))
	}
	if quotedPath("/tmp/it's") != `'/tmp/it'\''s'` {
		t.Fatal(quotedPath("/tmp/it's"))
	}
}
