package ui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/client"
	"github.com/tunnels-is/tunnels/types"
)

func TestDeviceView_LocalFirstThenNewestCreated(t *testing.T) {
	localID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	a := &App{
		localDevices: []client.LocalDeviceInfo{{ID: localID.String()}},
		devices: []types.Device{
			{ID: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Tag: "old-remote", CreatedAt: now.Add(-3 * time.Hour)},
			{ID: localID, Tag: "this-machine", CreatedAt: now.Add(-10 * time.Hour)},
			{ID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), Tag: "new-remote", CreatedAt: now.Add(-1 * time.Hour)},
		},
	}
	a.recomputeDeviceView()
	got := make([]string, len(a.deviceView))
	for i, d := range a.deviceView {
		got[i] = d.Tag
	}
	want := []string{"this-machine", "new-remote", "old-remote"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestDeviceView_MatchesLocalByPubkey(t *testing.T) {
	now := time.Now()
	a := &App{
		localDevices: []client.LocalDeviceInfo{{WireGuardPubKey: "pubkey-local"}},
		devices: []types.Device{
			{ID: uuid.New(), Tag: "remote", WireGuardKey: "other", CreatedAt: now},
			{ID: uuid.New(), Tag: "mine", WireGuardKey: "pubkey-local", CreatedAt: now.Add(-time.Hour)},
		},
	}
	a.recomputeDeviceView()
	if len(a.deviceView) != 2 || a.deviceView[0].Tag != "mine" {
		t.Fatalf("got %#v", a.deviceView)
	}
}

func TestDeviceView_SeveralLocalStayOnTop(t *testing.T) {
	aID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	bID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	now := time.Now()
	a := &App{
		localDevices: []client.LocalDeviceInfo{{ID: aID.String()}, {ID: bID.String()}},
		devices: []types.Device{
			{ID: uuid.New(), Tag: "remote", CreatedAt: now},
			{ID: aID, Tag: "older-local", CreatedAt: now.Add(-2 * time.Hour)},
			{ID: bID, Tag: "newer-local", CreatedAt: now.Add(-time.Hour)},
		},
	}
	a.recomputeDeviceView()
	got := []string{a.deviceView[0].Tag, a.deviceView[1].Tag, a.deviceView[2].Tag}
	want := []string{"newer-local", "older-local", "remote"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestDeviceStatus_ConnectedWinsOverLocal(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	d := types.Device{ID: id, WireGuardIP: "10.9.0.2", WireGuardKey: "pub"}
	ids := map[string]struct{}{id.String(): {}}
	pubs := map[string]struct{}{"pub": {}}
	conn := map[string]struct{}{"10.9.0.2": {}}

	label, got := deviceStatus(d, ids, pubs, conn)
	if label != "Connected" || got != toneSuccess {
		t.Fatalf("connected local = %q %v", label, got)
	}

	label, got = deviceStatus(d, ids, pubs, nil)
	if label != "This device" || got != tonePrimary {
		t.Fatalf("local = %q %v", label, got)
	}

	label, got = deviceStatus(d, nil, nil, nil)
	if label != "Remote" || got != toneNeutral {
		t.Fatalf("remote = %q %v", label, got)
	}
}

func TestDeviceStatus_EmptyIPIsNotConnected(t *testing.T) {
	d := types.Device{ID: uuid.New()}
	conn := map[string]struct{}{"": {}}
	label, got := deviceStatus(d, nil, nil, conn)
	if label != "Remote" || got != toneNeutral {
		t.Fatalf("blank ip = %q %v", label, got)
	}
}

func TestDevicesPage_BuildsCards(t *testing.T) {
	fy := test.NewApp()
	t.Cleanup(fy.Quit)
	applyZoomTokens(1)

	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	a := &App{
		devicesLoaded: true,
		devices: []types.Device{
			{ID: uuid.New(), Tag: "laptop", WireGuardIP: "10.9.0.2", WireGuardIPv6: "fd00::2", CreatedAt: now},
			{ID: uuid.New(), Tag: "phone", WireGuardIP: "10.9.0.3", CreatedAt: now.Add(-time.Hour)},
		},
	}
	page := a.devicesPage()
	if page == nil {
		t.Fatal("nil page")
	}
	if len(a.deviceView) != 2 {
		t.Fatalf("view = %d", len(a.deviceView))
	}
}

func TestDeviceView_FilterStillApplies(t *testing.T) {
	id := uuid.New()
	a := &App{
		filterDevices: "laptop",
		localDevices:  []client.LocalDeviceInfo{{ID: id.String()}},
		devices: []types.Device{
			{ID: id, Tag: "phone", CreatedAt: time.Now()},
			{ID: uuid.New(), Tag: "laptop", CreatedAt: time.Now().Add(-time.Hour)},
		},
	}
	a.recomputeDeviceView()
	if len(a.deviceView) != 1 || a.deviceView[0].Tag != "laptop" {
		t.Fatalf("got %#v", a.deviceView)
	}
}
