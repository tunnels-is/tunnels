package client

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAccountWorkspace_SaveLoadActivate(t *testing.T) {
	dir := t.TempDir()
	STATE.Store(&stateV2{BasePath: dir + string(os.PathSeparator)})
	InitBaseFoldersAndPaths()

	u := &User{
		ID:    "user-aaa-111",
		Email: "a@example.com",
		DeviceToken: &DEVICE_TOKEN{
			DT: "token-a",
			N:  "dev",
		},
	}
	if err := saveUser(u); err != nil {
		t.Fatalf("saveUser: %v", err)
	}
	if u.SaveFileHash == "" {
		t.Fatal("expected SaveFileHash")
	}

	s := STATE.Load()
	if s.ActiveAccountHash != u.SaveFileHash {
		t.Fatalf("active hash = %q, want %q", s.ActiveAccountHash, u.SaveFileHash)
	}
	userFile := accountUserFile(u.SaveFileHash)
	info, err := os.Stat(userFile)
	if err != nil {
		t.Fatalf("user file missing: %v", err)
	}
	if err := validateUserKeyFile(userFile, info); err != nil {
		t.Fatalf("account file permissions: %v", err)
	}
	if _, err := os.Stat(accountTunnelsPath(u.SaveFileHash)); err != nil {
		t.Fatalf("tunnels dir missing: %v", err)
	}
	if _, err := os.Stat(accountDevicesPath(u.SaveFileHash)); err != nil {
		t.Fatalf("devices dir missing: %v", err)
	}

	u2 := &User{
		ID:    "user-bbb-222",
		Email: "b@example.com",
		DeviceToken: &DEVICE_TOKEN{
			DT: "token-b",
			N:  "dev",
		},
	}
	if err := saveUser(u2); err != nil {
		t.Fatalf("saveUser 2: %v", err)
	}
	if u2.SaveFileHash == u.SaveFileHash {
		t.Fatal("accounts must have distinct hashes")
	}

	users, err := getUsers()
	if err != nil {
		t.Fatalf("getUsers: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("getUsers len = %d, want 2", len(users))
	}

	foundA := false
	for _, got := range users {
		if got.Email == "a@example.com" && got.DeviceToken != nil && got.DeviceToken.DT == "token-a" {
			foundA = true
		}
	}
	if !foundA {
		t.Fatal("did not decrypt account A via folder hash alone")
	}

	if _, err := os.Stat(filepath.Join(dir, "user.key")); err == nil {
		t.Fatal("user.key should not be created")
	}

	if err := activateAccountByUserID(u.ID); err != nil {
		t.Fatalf("activate first: %v", err)
	}
	s = STATE.Load()
	wantTunnels := accountTunnelsPath(u.SaveFileHash)
	if s.TunnelsPath != wantTunnels {
		t.Fatalf("TunnelsPath = %q, want %q", s.TunnelsPath, wantTunnels)
	}

	if _, ok := TunnelMetaMap.Load(DefaultTunnelName); !ok {
		t.Fatal("expected default tunnel in active account")
	}

	if err := delUser(u2.SaveFileHash); err != nil {
		t.Fatalf("delUser: %v", err)
	}
	if _, err := os.Stat(accountDir(u2.SaveFileHash)); !os.IsNotExist(err) {
		t.Fatalf("account dir should be gone, err=%v", err)
	}
	users, err = getUsers()
	if err != nil {
		t.Fatalf("getUsers after delete: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("getUsers len after delete = %d, want 1", len(users))
	}
}

func TestEncryptAccountBlob_UsesFolderHashOnly(t *testing.T) {
	hash := "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	if len(hash) != 64 {

	}

	h, err := userIDToAccountHash("test-user-for-blob")
	if err != nil {
		t.Fatal(err)
	}
	pt := []byte(`{"hello":"world","token":"secret"}`)
	blob, err := encryptAccountBlob(pt, h)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if blob[0] != accountCryptoVersion {
		t.Fatalf("version byte = %d", blob[0])
	}
	out, err := decryptAccountBlob(blob, h)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(out, pt) {
		t.Fatalf("round trip mismatch")
	}

	_, err = decryptAccountBlob(blob, "0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil {
		t.Fatal("expected decrypt failure with wrong hash")
	}
}
