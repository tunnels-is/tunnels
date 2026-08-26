package client

import (
	"errors"
	"sync"
	"testing"

	"github.com/tunnels-is/tunnels/types"
)

func setupAutoConnectHome(t *testing.T) *User {
	t.Helper()
	dir := t.TempDir()
	prevS := STATE.Load()
	prevC := CONFIG.Load()
	t.Cleanup(func() {
		STATE.Store(prevS)
		CONFIG.Store(prevC)
		clearTunnelMap()
		clearActiveTunnels()
	})
	clearTunnelMap()
	clearActiveTunnels()

	STATE.Store(&stateV2{
		BasePath:   dir + "/",
		TunnelType: string(types.DefaultTun),
	})
	if err := InitBaseFoldersAndPaths(); err != nil {
		t.Fatal(err)
	}
	cs := &ControlServer{ID: "tunnels", Host: "api.tunnels.is", Port: "443", ValidateCertificate: true}
	CONFIG.Store(&configV2{ControlServers: []*ControlServer{cs}})

	u := &User{
		ID:            "user-auto-connect-1",
		Email:         "auto@example.com",
		DeviceToken:   &DEVICE_TOKEN{DT: "device-token", N: "dev"},
		ControlServer: cs,
	}
	if err := saveUser(u); err != nil {
		t.Fatal(err)
	}
	return u
}

func clearActiveTunnels() {
	TunnelMap.Range(func(key string, _ *TUN) bool {
		TunnelMap.Delete(key)
		return true
	})
}

func TestAutoConnect_OnlyFlaggedTunnelsWithServerID(t *testing.T) {
	setupAutoConnectHome(t)

	def := FindTunnel(DefaultTunnelName)
	if def == nil {
		t.Fatal("expected default tunnel")
	}
	def.AutoConnect = true
	def.ServerID = "server-keep"
	if err := writeTunnelsToDisk(def.Tag); err != nil {
		t.Fatal(err)
	}

	skipFlag := createTunnel()
	skipFlag.Tag = "no-flag"
	skipFlag.IFName = "no-flag"
	skipFlag.AutoConnect = false
	skipFlag.ServerID = "server-other"
	TunnelMetaMap.Store(skipFlag.Tag, skipFlag)

	skipID := createTunnel()
	skipID.Tag = "no-server"
	skipID.IFName = "no-server"
	skipID.AutoConnect = true
	skipID.ServerID = ""
	TunnelMetaMap.Store(skipID.Tag, skipID)

	var mu sync.Mutex
	var got []*ConnectionRequest
	autoConnectWith(func(cr *ConnectionRequest) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		cp := *cr
		got = append(got, &cp)
		return 200, nil
	})

	if len(got) != 1 {
		t.Fatalf("connect calls = %d, want 1", len(got))
	}
	if got[0].Tag != DefaultTunnelName || got[0].ServerID != "server-keep" {
		t.Fatalf("got tag=%q server=%q", got[0].Tag, got[0].ServerID)
	}
	if got[0].UserID != "user-auto-connect-1" || got[0].DeviceToken != "device-token" {
		t.Fatalf("credentials: uid=%q token=%q", got[0].UserID, got[0].DeviceToken)
	}
}

func TestAutoConnect_SkipsBusyTunnel(t *testing.T) {
	setupAutoConnectHome(t)

	def := FindTunnel(DefaultTunnelName)
	def.AutoConnect = true
	def.ServerID = "server-keep"

	live := &TUN{ID: "live-1"}
	live.meta.Store(def)
	live.SetState(TUN_Connected)
	TunnelMap.Store(live.ID, live)

	called := 0
	autoConnectWith(func(*ConnectionRequest) (int, error) {
		called++
		return 200, nil
	})
	if called != 0 {
		t.Fatalf("called %d times, want 0", called)
	}
}

func TestAutoConnect_NoAccountIsNoop(t *testing.T) {
	dir := t.TempDir()
	prevS := STATE.Load()
	t.Cleanup(func() {
		STATE.Store(prevS)
		clearTunnelMap()
		clearActiveTunnels()
	})
	STATE.Store(&stateV2{BasePath: dir + "/"})
	if err := InitBaseFoldersAndPaths(); err != nil {
		t.Fatal(err)
	}
	clearTunnelMap()
	clearActiveTunnels()

	called := 0
	autoConnectWith(func(*ConnectionRequest) (int, error) {
		called++
		return 200, nil
	})
	if called != 0 {
		t.Fatalf("called %d times, want 0", called)
	}
}

func TestAutoConnect_ReportsConnectError(t *testing.T) {
	setupAutoConnectHome(t)
	def := FindTunnel(DefaultTunnelName)
	def.AutoConnect = true
	def.ServerID = "server-keep"

	autoConnectWith(func(*ConnectionRequest) (int, error) {
		return 502, errors.New("boom")
	})
}

func TestPersistTunnelServerID(t *testing.T) {
	setupAutoConnectHome(t)
	def := FindTunnel(DefaultTunnelName)
	if def == nil {
		t.Fatal("expected default tunnel")
	}
	if err := persistTunnelServerID(def, "server-xyz"); err != nil {
		t.Fatal(err)
	}
	if def.ServerID != "server-xyz" {
		t.Fatalf("ServerID = %q", def.ServerID)
	}
	if err := persistTunnelServerID(def, "server-xyz"); err != nil {
		t.Fatal(err)
	}

	clearTunnelMap()
	if err := loadTunnelsFromDisk(); err != nil {
		t.Fatal(err)
	}
	loaded, ok := TunnelMetaMap.Load(DefaultTunnelName)
	if !ok || loaded.ServerID != "server-xyz" {
		t.Fatalf("reloaded ServerID = %+v ok=%v", loaded, ok)
	}
}

func TestServerConnectUsesDefaultTunnel(t *testing.T) {
	cr := &ConnectionRequest{Tag: "should-be-replaced"}
	code, err := ServerConnect(cr)
	if cr.Tag != DefaultTunnelName {
		t.Fatalf("Tag = %q, want %q", cr.Tag, DefaultTunnelName)
	}
	if code != 400 || err == nil {
		t.Fatalf("empty ServerID: code=%d err=%v", code, err)
	}

	code, err = ServerConnect(nil)
	if code != 400 || err == nil {
		t.Fatalf("nil request: code=%d err=%v", code, err)
	}
}
