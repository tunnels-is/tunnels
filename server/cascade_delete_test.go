package main

import (
	"testing"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

func TestDeleteUser_CascadesDevices(t *testing.T) {
	setupTestDB(t)
	u := testUser("cascade@example.com", "")
	if err := createUser(u); err != nil {
		t.Fatal(err)
	}
	dev := &types.Device{ID: uuid.New(), UserID: u.ID, ServerID: uuid.New(), WireGuardKey: "wgkey-cascade-user"}
	if err := createDevice(dev); err != nil {
		t.Fatal(err)
	}

	if err := deleteUserByID(u.ID); err != nil {
		t.Fatal(err)
	}

	devs, err := getDevicesByUserID(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(devs) != 0 {
		t.Fatalf("expected 0 devices after user delete, got %d", len(devs))
	}

	if d, _ := findDeviceByWGKey("wgkey-cascade-user"); d != nil {
		t.Fatal("wgkey index still reserved after user delete")
	}
}

func TestDeleteServer_CascadesDevices(t *testing.T) {
	setupTestDB(t)
	srv := &types.Server{ID: uuid.New(), Tag: "s", APIKey: "srvkey"}
	if err := createServer(srv); err != nil {
		t.Fatal(err)
	}
	owner := uuid.New()
	dev := &types.Device{ID: uuid.New(), UserID: owner, ServerID: srv.ID, WireGuardKey: "wgkey-cascade-srv"}
	if err := createDevice(dev); err != nil {
		t.Fatal(err)
	}

	other := &types.Device{ID: uuid.New(), UserID: owner, ServerID: uuid.New(), WireGuardKey: "wgkey-other"}
	if err := createDevice(other); err != nil {
		t.Fatal(err)
	}

	if err := deleteServerByID(srv.ID); err != nil {
		t.Fatal(err)
	}

	if d, _ := findDeviceByWGKey("wgkey-cascade-srv"); d != nil {
		t.Fatal("device bound to deleted server was not removed")
	}
	if d, _ := findDeviceByWGKey("wgkey-other"); d == nil {
		t.Fatal("device on a different server must survive")
	}
}

func TestDeleteGroup_ScrubsMembership(t *testing.T) {
	setupTestDB(t)
	g := &Group{ID: uuid.New(), Tag: "g"}
	if err := createGroup(g); err != nil {
		t.Fatal(err)
	}
	u := &User{ID: uuid.New(), Email: "grp@example.com", Groups: []uuid.UUID{g.ID}}
	if err := createUser(u); err != nil {
		t.Fatal(err)
	}
	srv := &types.Server{ID: uuid.New(), Tag: "gs", APIKey: "gsk", Groups: []uuid.UUID{g.ID}}
	if err := createServer(srv); err != nil {
		t.Fatal(err)
	}

	if err := deleteGroupByID(g.ID); err != nil {
		t.Fatal(err)
	}

	gotU, _ := findUserByID(u.ID)
	if gotU == nil {
		t.Fatal("user vanished")
	}
	for _, gid := range gotU.Groups {
		if gid == g.ID {
			t.Fatal("group ID still on user after group delete")
		}
	}
	gotS, _ := findServerByID(srv.ID)
	if gotS == nil {
		t.Fatal("server vanished")
	}
	for _, gid := range gotS.Groups {
		if gid == g.ID {
			t.Fatal("group ID still on server after group delete")
		}
	}
}
