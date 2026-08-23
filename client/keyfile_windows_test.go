//go:build windows

package client

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/sys/windows"
)

func TestValidateUserKeyFile_WindowsUnixModeBitsAreNotInsecure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "user")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := secureSecretFile(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 == 0 {
		t.Skip("this Windows reports 0600; unix-bit false positive cannot be shown")
	}
	if err := validateUserKeyFile(path, info); err != nil {
		t.Fatalf("unix mode %o must not fail ACL validation: %v", info.Mode().Perm(), err)
	}
}

func TestWarnIfInsecureSecretFile_RepairsWorldReadableACL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "user")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := setFileSDDL(path, "D:P(A;;FA;;;WD)"); err != nil {
		t.Fatalf("set world DACL: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateUserKeyFile(path, info); err == nil {
		t.Fatal("Everyone:FA DACL must be rejected")
	}

	warnIfInsecureSecretFile(path)

	info, err = os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateUserKeyFile(path, info); err != nil {
		t.Fatalf("repair should remove world access: %v", err)
	}
}

func TestWriteSecretFile_WindowsRejectsEveryone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "user")
	if err := writeSecretFile(path, []byte("x")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateUserKeyFile(path, info); err != nil {
		t.Fatalf("writeSecretFile ACL: %v", err)
	}
}

func setFileSDDL(path, sddl string) error {
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	err = windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		dacl,
		nil,
	)
	runtime.KeepAlive(sd)
	return err
}
