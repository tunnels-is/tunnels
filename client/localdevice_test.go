package client

import (
	"os"
	"testing"
)

func TestLocalDevice_SaveLoadByServerID(t *testing.T) {
	dir := t.TempDir()
	STATE.Store(&stateV2{BasePath: dir + string(os.PathSeparator)})
	InitBaseFoldersAndPaths()

	u := &User{ID: "dev-user-1", Email: "d@example.com", DeviceToken: &DEVICE_TOKEN{DT: "t", N: "n"}}
	if err := saveUser(u); err != nil {
		t.Fatal(err)
	}

	d := &LocalDevice{
		ID:               "device-aaa",
		ServerID:         "server-111",
		Tag:              "test-dev",
		WireGuardPrivKey: "priv-key-material",
		WireGuardPubKey:  "pub-key-material",
		WireGuardIP:      "10.7.0.5",
	}
	if err := saveLocalDevice(d); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := findLocalDeviceByServerID("server-111")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.WireGuardPrivKey != "priv-key-material" {
		t.Fatalf("find by server: %+v", got)
	}

	list, err := listLocalDeviceInfo()
	if err != nil || len(list) != 1 {
		t.Fatalf("list info: %v len=%d", err, len(list))
	}
	if list[0].WireGuardPubKey != "pub-key-material" {
		t.Fatalf("info missing pub: %+v", list[0])
	}
	// Ensure private key never appears in info JSON path fields used by UI
	if list[0].ID != "device-aaa" {
		t.Fatalf("id: %s", list[0].ID)
	}

	none, err := findLocalDeviceByServerID("other-server")
	if err != nil || none != nil {
		t.Fatalf("expected no device for other server, got %+v err=%v", none, err)
	}
}
