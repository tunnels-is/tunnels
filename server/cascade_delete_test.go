package main

import (
	"testing"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

// C3: deleting a user cascade-deletes their devices and the device indexes,
// freeing WG IPs and releasing the wgkey reservation.
func TestDeleteUser_CascadesDevices(t *testing.T) {
	setupTestDB(t)
	u := testUser("cascade@example.com", "")
	if err := BBolt_CreateUser(u); err != nil {
		t.Fatal(err)
	}
	dev := &types.Device{ID: uuid.New(), UserID: u.ID, ServerID: uuid.New(), WireGuardKey: "wgkey-cascade-user"}
	if err := BBolt_CreateDevice(dev); err != nil {
		t.Fatal(err)
	}

	if err := BBolt_DeleteUserByID(u.ID.String()); err != nil {
		t.Fatal(err)
	}

	devs, err := BBolt_GetDevicesByUserID(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(devs) != 0 {
		t.Fatalf("expected 0 devices after user delete, got %d", len(devs))
	}
	// The wgkey must be free for re-registration.
	if d, _ := BBolt_FindDeviceByWGKey("wgkey-cascade-user"); d != nil {
		t.Fatal("wgkey index still reserved after user delete")
	}
}

// C4: deleting a server cascade-deletes devices bound to it.
func TestDeleteServer_CascadesDevices(t *testing.T) {
	setupTestDB(t)
	srv := &types.Server{ID: uuid.New(), Tag: "s", APIKey: "srvkey"}
	if err := BBolt_CreateServer(srv); err != nil {
		t.Fatal(err)
	}
	owner := uuid.New()
	dev := &types.Device{ID: uuid.New(), UserID: owner, ServerID: srv.ID, WireGuardKey: "wgkey-cascade-srv"}
	if err := BBolt_CreateDevice(dev); err != nil {
		t.Fatal(err)
	}
	// A device on a different server must survive.
	other := &types.Device{ID: uuid.New(), UserID: owner, ServerID: uuid.New(), WireGuardKey: "wgkey-other"}
	if err := BBolt_CreateDevice(other); err != nil {
		t.Fatal(err)
	}

	if err := BBolt_DeleteServerByID(srv.ID.String()); err != nil {
		t.Fatal(err)
	}

	if d, _ := BBolt_FindDeviceByWGKey("wgkey-cascade-srv"); d != nil {
		t.Fatal("device bound to deleted server was not removed")
	}
	if d, _ := BBolt_FindDeviceByWGKey("wgkey-other"); d == nil {
		t.Fatal("device on a different server must survive")
	}
}

// C1: deleting a group scrubs its ID from every user and server, so access
// (hasSharedOrNoGroup) is actually revoked.
func TestDeleteGroup_ScrubsMembership(t *testing.T) {
	setupTestDB(t)
	g := &Group{ID: uuid.New(), Tag: "g"}
	if err := BBolt_CreateGroup(g); err != nil {
		t.Fatal(err)
	}
	u := &User{ID: uuid.New(), Email: "grp@example.com", Groups: []uuid.UUID{g.ID}}
	if err := BBolt_CreateUser(u); err != nil {
		t.Fatal(err)
	}
	srv := &types.Server{ID: uuid.New(), Tag: "gs", APIKey: "gsk", Groups: []uuid.UUID{g.ID}}
	if err := BBolt_CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	if err := BBolt_DeleteGroupByID(g.ID.String()); err != nil {
		t.Fatal(err)
	}

	gotU, _ := BBolt_findUserByID(u.ID.String())
	if gotU == nil {
		t.Fatal("user vanished")
	}
	for _, gid := range gotU.Groups {
		if gid == g.ID {
			t.Fatal("group ID still on user after group delete")
		}
	}
	gotS, _ := BBolt_FindServerByID(srv.ID.String())
	if gotS == nil {
		t.Fatal("server vanished")
	}
	for _, gid := range gotS.Groups {
		if gid == g.ID {
			t.Fatal("group ID still on server after group delete")
		}
	}
}
