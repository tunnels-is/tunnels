package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

func meshTestLogger() {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
}

func callWGMesh(t *testing.T, server *types.Server) (*httptest.ResponseRecorder, types.WGMeshResponse) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/wg/mesh", nil)
	req = req.WithContext(context.WithValue(req.Context(), contextKeyServer, server))
	w := httptest.NewRecorder()
	API_WGMesh(w, req)
	var resp types.WGMeshResponse
	if w.Code == http.StatusOK {
		_ = json.NewDecoder(w.Body).Decode(&resp)
	}
	return w, resp
}

func TestAPI_WGMesh_SiblingSelection(t *testing.T) {
	setupTestDB(t)
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	mg := uuid.New().String()

	caller := &types.Server{
		ID: uuid.New(), APIKey: uuid.NewString(), MeshGroupID: mg,
		WireGuardPubKey: makeWGKey(), IP: "10.0.0.1",
		WireGuardSubnet: "10.0.0.0/22", WireGuardPort: 51820,
	}
	// same group, fully provisioned → included
	siblingIn := &types.Server{
		ID: uuid.New(), APIKey: uuid.NewString(), MeshGroupID: mg,
		WireGuardPubKey: makeWGKey(), IP: "2.2.2.2",
		WireGuardSubnet: "10.3.0.0/16", WireGuardPort: 51820, // mesh port defaults to 51821
	}
	// same group, not yet provisioned (no pubkey) → excluded
	siblingUnprov := &types.Server{
		ID: uuid.New(), APIKey: uuid.NewString(), MeshGroupID: mg,
		IP: "3.3.3.3", WireGuardSubnet: "10.4.0.0/16", WireGuardPort: 51820,
	}
	// different group → excluded
	siblingOther := &types.Server{
		ID: uuid.New(), APIKey: uuid.NewString(), MeshGroupID: uuid.New().String(),
		WireGuardPubKey: makeWGKey(), IP: "4.4.4.4",
		WireGuardSubnet: "10.5.0.0/16", WireGuardPort: 51820,
	}
	for _, s := range []*types.Server{caller, siblingIn, siblingUnprov, siblingOther} {
		if err := BBolt_CreateServer(s); err != nil {
			t.Fatal(err)
		}
	}

	w, resp := callWGMesh(t, caller)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(resp.Peers) != 1 {
		t.Fatalf("expected exactly 1 mesh peer (siblingIn), got %d: %+v", len(resp.Peers), resp.Peers)
	}
	p := resp.Peers[0]
	if p.Endpoint != "2.2.2.2:51821" {
		t.Fatalf("endpoint = %q, want 2.2.2.2:51821 (mesh port defaults to WireGuardPort+1)", p.Endpoint)
	}
	if len(p.AllowedSubnets) != 1 || p.AllowedSubnets[0] != "10.3.0.0/16" {
		t.Fatalf("AllowedSubnets = %v, want [10.3.0.0/16]", p.AllowedSubnets)
	}
}

func TestAPI_WGMesh_NoMeshGroup(t *testing.T) {
	setupTestDB(t)
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	caller := &types.Server{ID: uuid.New(), APIKey: uuid.NewString()} // no MeshGroupID
	if err := BBolt_CreateServer(caller); err != nil {
		t.Fatal(err)
	}
	w, resp := callWGMesh(t, caller)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(resp.Peers) != 0 {
		t.Fatalf("a server with no mesh group should get no peers, got %d", len(resp.Peers))
	}
}

func TestValidateServerMesh(t *testing.T) {
	setupTestDB(t)
	meshTestLogger()

	// mesh port collides with client port
	if err := validateServerMesh(&types.Server{WireGuardPort: 51820, WireGuardMeshPort: 51820}); err == nil {
		t.Fatal("meshport == wgport should error")
	}
	// no mesh group → ok
	if err := validateServerMesh(&types.Server{WireGuardPort: 51820, WireGuardMeshPort: 51821}); err != nil {
		t.Fatalf("no mesh group should pass, got %v", err)
	}
	// references a non-existent mesh group
	if err := validateServerMesh(&types.Server{
		ID: uuid.New(), WireGuardPort: 51820, WireGuardMeshPort: 51821,
		MeshGroupID: uuid.New().String(), WireGuardSubnet: "10.0.0.0/22",
	}); err == nil {
		t.Fatal("non-existent mesh group should error")
	}

	// real group with a sibling on 10.3.0.0/16
	mg := &types.MeshGroup{ID: uuid.New(), Tag: "g"}
	if err := DB_CreateMeshGroup(mg); err != nil {
		t.Fatal(err)
	}
	sib := &types.Server{
		ID: uuid.New(), APIKey: uuid.NewString(), MeshGroupID: mg.ID.String(),
		WireGuardSubnet: "10.3.0.0/16", WireGuardPort: 51820,
	}
	if err := BBolt_CreateServer(sib); err != nil {
		t.Fatal(err)
	}

	// non-overlapping subnet → ok
	if err := validateServerMesh(&types.Server{
		ID: uuid.New(), MeshGroupID: mg.ID.String(), WireGuardPort: 51820,
		WireGuardMeshPort: 51821, WireGuardSubnet: "10.4.0.0/16",
	}); err != nil {
		t.Fatalf("non-overlapping subnet should pass, got %v", err)
	}
	// overlapping subnet → error
	if err := validateServerMesh(&types.Server{
		ID: uuid.New(), MeshGroupID: mg.ID.String(), WireGuardPort: 51820,
		WireGuardMeshPort: 51821, WireGuardSubnet: "10.3.0.0/24",
	}); err == nil {
		t.Fatal("subnet overlapping a mesh sibling should error")
	}
}

// Exercises both the delete-clears-members behavior (fix #3) and that
// BBolt_UpdateServer persists MeshGroupID changes (fix #1).
func TestMeshGroupDelete_ClearsMembers(t *testing.T) {
	setupTestDB(t)
	meshTestLogger()

	mg := &types.MeshGroup{ID: uuid.New(), Tag: "g"}
	if err := DB_CreateMeshGroup(mg); err != nil {
		t.Fatal(err)
	}
	s1 := &types.Server{ID: uuid.New(), APIKey: uuid.NewString(), MeshGroupID: mg.ID.String(), WireGuardSubnet: "10.3.0.0/16", WireGuardPort: 51820}
	s2 := &types.Server{ID: uuid.New(), APIKey: uuid.NewString(), MeshGroupID: mg.ID.String(), WireGuardSubnet: "10.4.0.0/16", WireGuardPort: 51820}
	if err := BBolt_CreateServer(s1); err != nil {
		t.Fatal(err)
	}
	if err := BBolt_CreateServer(s2); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/ui/meshgroup/delete",
		strings.NewReader(`{"MeshGroupID":"`+mg.ID.String()+`"}`))
	w := httptest.NewRecorder()
	API_AdminMeshGroupDelete(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("delete returned %d", w.Code)
	}

	for _, id := range []uuid.UUID{s1.ID, s2.ID} {
		got, err := DB_FindServerByID(id)
		if err != nil || got == nil {
			t.Fatalf("server %s lookup: %v", id, err)
		}
		if got.MeshGroupID != "" {
			t.Fatalf("server %s still has MeshGroupID %q after group delete", id, got.MeshGroupID)
		}
	}
}
