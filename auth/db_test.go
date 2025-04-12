package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/google/uuid"
)

func setupTestDBs(t *testing.T) (userDB, groupDB, serverDB, tokenDB, indexDB *badger.DB) {
	t.Helper() // Marks this as a test helper function

	// Configure slog for tests (can direct output if needed, nil logger for badger)
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})) // Keep test output clean unless errors
		slog.SetDefault(logger)
	}
	dbOpts := badger.DefaultOptions("").WithLogger(nil) // Disable badger's own verbose logging

	userDir := t.TempDir()
	userDB, err := badger.Open(dbOpts.WithDir(userDir).WithValueDir(userDir))
	if err != nil {
		t.Fatalf("Failed to open test user DB: %v", err)
	}

	groupDir := t.TempDir()
	groupDB, err = badger.Open(dbOpts.WithDir(groupDir).WithValueDir(groupDir))
	if err != nil {
		userDB.Close()
		t.Fatalf("Failed to open test group DB: %v", err)
	}

	serverDir := t.TempDir()
	serverDB, err = badger.Open(dbOpts.WithDir(serverDir).WithValueDir(serverDir))
	if err != nil {
		userDB.Close()
		groupDB.Close()
		t.Fatalf("Failed to open test server DB: %v", err)
	}

	tokenDir := t.TempDir()
	tokenDB, err = badger.Open(dbOpts.WithDir(tokenDir).WithValueDir(tokenDir))
	if err != nil {
		userDB.Close()
		groupDB.Close()
		serverDB.Close()
		t.Fatalf("Failed to open test token DB: %v", err)
	}

	indexDir := t.TempDir()
	indexDB, err = badger.Open(dbOpts.WithDir(indexDir).WithValueDir(indexDir))
	if err != nil {
		userDB.Close()
		groupDB.Close()
		serverDB.Close()
		tokenDB.Close()
		t.Fatalf("Failed to open test index DB: %v", err)
	}

	t.Cleanup(func() {
		indexDB.Close()
		tokenDB.Close()
		serverDB.Close()
		groupDB.Close()
		userDB.Close()
		// t.TempDir() handles directory removal
	})

	return userDB, groupDB, serverDB, tokenDB, indexDB
}

// Override global DB vars for test scope
func setupTestDBGlobals(t *testing.T) {
	t.Helper()
	userDB, groupDB, serverDB, tokenDB, indexDB = setupTestDBs(t)
}

// --- User Tests ---

func TestSaveAndGetUser(t *testing.T) {
	setupTestDBGlobals(t)

	testUser := &User{
		UUID:       uuid.NewString(),
		Username:   "testuser",
		GoogleID:   "google123",
		IsAdmin:    false,
		IsManager:  false,
		OTPEnabled: false,
		OTPSecret:  "secret", // Usually generated, mock here
	}

	err := saveUser(testUser)
	if err != nil {
		t.Fatalf("saveUser failed: %v", err)
	}

	retrievedUser, err := getUser(testUser.UUID)
	if err != nil {
		t.Fatalf("getUser failed: %v", err)
	}

	if !reflect.DeepEqual(testUser, retrievedUser) {
		t.Errorf("Retrieved user %+v does not match saved user %+v", retrievedUser, testUser)
	}

	_, err = getUser(uuid.NewString()) // Try getting non-existent user
	if err == nil {
		t.Errorf("Expected error when getting non-existent user, but got nil")
	} else if !errors.Is(err, ErrNotFound) && !errors.Is(err, badger.ErrKeyNotFound) {
		t.Errorf("Expected ErrNotFound or badger.ErrKeyNotFound, but got: %v", err)
	}

}

func TestGetUserByGoogleID(t *testing.T) {
	setupTestDBGlobals(t)

	googleID := "google_unique_id_456"
	testUser := &User{UUID: uuid.NewString(), Username: "googleUser", GoogleID: googleID}
	otherUser := &User{UUID: uuid.NewString(), Username: "otherUser", GoogleID: "another_id_789"}

	err := saveUser(testUser)
	if err != nil {
		t.Fatalf("saveUser failed for testUser: %v", err)
	}
	err = saveUser(otherUser)
	if err != nil {
		t.Fatalf("saveUser failed for otherUser: %v", err)
	}

	retrievedUser, err := getUserByGoogleID(googleID)
	if err != nil {
		t.Fatalf("getUserByGoogleID failed for %s: %v", googleID, err)
	}

	if retrievedUser.UUID != testUser.UUID || retrievedUser.GoogleID != testUser.GoogleID {
		t.Errorf("Retrieved user %+v does not match expected user %+v", retrievedUser, testUser)
	}

	_, err = getUserByGoogleID("non_existent_google_id")
	if err == nil {
		t.Errorf("Expected error when getting non-existent Google ID, but got nil")
	} else if !errors.Is(err, ErrNotFound) { // Specific error expected here
		t.Errorf("Expected ErrNotFound, but got: %v", err)
	}
}

func TestDeleteUser(t *testing.T) {
	setupTestDBGlobals(t)

	testUser := &User{UUID: uuid.NewString(), Username: "deleteMe"}
	err := saveUser(testUser)
	if err != nil {
		t.Fatalf("saveUser failed: %v", err)
	}

	err = deleteUser(testUser.UUID)
	if err != nil {
		t.Fatalf("deleteUser failed: %v", err)
	}

	_, err = getUser(testUser.UUID)
	if err == nil {
		t.Errorf("getUser succeeded after deleteUser, expected error")
	} else if !errors.Is(err, ErrNotFound) && !errors.Is(err, badger.ErrKeyNotFound) {
		t.Errorf("Expected ErrNotFound or badger.ErrKeyNotFound, but got: %v", err)
	}
}

func TestListUsers(t *testing.T) {
	setupTestDBGlobals(t)

	user1 := &User{UUID: uuid.NewString(), Username: "listUser1"}
	user2 := &User{UUID: uuid.NewString(), Username: "listUser2"}

	err := saveUser(user1)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	err = saveUser(user2)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	users, err := listUsers()
	if err != nil {
		t.Fatalf("listUsers failed: %v", err)
	}

	if len(users) != 2 {
		t.Errorf("Expected 2 users, got %d", len(users))
	}

	// Simple check if usernames are present
	found1, found2 := false, false
	for _, u := range users {
		if u.Username == user1.Username {
			found1 = true
		}
		if u.Username == user2.Username {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Errorf("Did not find both users in the list: found1=%t, found2=%t", found1, found2)
	}
}

func TestUsernameIndex(t *testing.T) {
	setupTestDBGlobals(t)

	username := "indexUser"
	userUUID := uuid.NewString()

	err := setUsernameIndex(username, userUUID)
	if err != nil {
		t.Fatalf("setUsernameIndex failed: %v", err)
	}

	retrievedUUID, err := getUserUUIDByUsername(username)
	if err != nil {
		t.Fatalf("getUserUUIDByUsername failed: %v", err)
	}
	if retrievedUUID != userUUID {
		t.Errorf("Expected UUID %s, got %s", userUUID, retrievedUUID)
	}

	// Test overwrite (if allowed by implementation)
	newUserUUID := uuid.NewString()
	err = setUsernameIndex(username, newUserUUID)
	if err != nil {
		t.Fatalf("setUsernameIndex (overwrite) failed: %v", err)
	}
	retrievedUUID, err = getUserUUIDByUsername(username)
	if err != nil {
		t.Fatalf("getUserUUIDByUsername after overwrite failed: %v", err)
	}
	if retrievedUUID != newUserUUID {
		t.Errorf("Expected overwritten UUID %s, got %s", newUserUUID, retrievedUUID)
	}

	err = deleteUsernameIndex(username)
	if err != nil {
		t.Fatalf("deleteUsernameIndex failed: %v", err)
	}

	_, err = getUserUUIDByUsername(username)
	if err == nil {
		t.Errorf("getUserUUIDByUsername succeeded after delete, expected error")
	} else if !errors.Is(err, ErrNotFound) && !errors.Is(err, badger.ErrKeyNotFound) {
		t.Errorf("Expected ErrNotFound or badger.ErrKeyNotFound, but got: %v", err)
	}
}

// --- Group Tests ---

func TestSaveAndGetGroup(t *testing.T) {
	setupTestDBGlobals(t)

	testGroup := &Group{
		UUID:        uuid.NewString(),
		Name:        "TestGroup",
		UserUUIDs:   []string{uuid.NewString(), uuid.NewString()},
		ServerUUIDs: []string{uuid.NewString()},
	}

	err := saveGroup(testGroup)
	if err != nil {
		t.Fatalf("saveGroup failed: %v", err)
	}

	retrievedGroup, err := getGroup(testGroup.UUID)
	if err != nil {
		t.Fatalf("getGroup failed: %v", err)
	}

	if !reflect.DeepEqual(testGroup, retrievedGroup) {
		t.Errorf("Retrieved group %+v does not match saved group %+v", retrievedGroup, testGroup)
	}
}

func TestDeleteGroup(t *testing.T) {
	setupTestDBGlobals(t)

	testGroup := &Group{UUID: uuid.NewString(), Name: "DeleteGroup"}
	err := saveGroup(testGroup)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	err = deleteGroup(testGroup.UUID)
	if err != nil {
		t.Fatalf("deleteGroup failed: %v", err)
	}

	_, err = getGroup(testGroup.UUID)
	if err == nil {
		t.Errorf("getGroup succeeded after delete, expected error")
	} else if !errors.Is(err, ErrNotFound) && !errors.Is(err, badger.ErrKeyNotFound) {
		t.Errorf("Expected ErrNotFound or badger.ErrKeyNotFound, but got: %v", err)
	}
}

func TestListGroups(t *testing.T) {
	setupTestDBGlobals(t)
	group1 := &Group{UUID: uuid.NewString(), Name: "ListGroup1"}
	group2 := &Group{UUID: uuid.NewString(), Name: "ListGroup2"}

	err := saveGroup(group1)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	err = saveGroup(group2)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	groups, err := listGroups()
	if err != nil {
		t.Fatalf("listGroups failed: %v", err)
	}
	if len(groups) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(groups))
	}
}

// --- Server Tests ---

func TestSaveAndGetServer(t *testing.T) {
	setupTestDBGlobals(t)
	testServer := &Server{
		UUID:      uuid.NewString(),
		Name:      "WebServer",
		Hostname:  "web01.example.com",
		IPAddress: "192.168.1.10",
	}
	err := saveServer(testServer)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	retrieved, err := getServer(testServer.UUID)
	if err != nil {
		t.Fatalf("getServer failed: %v", err)
	}

	if !reflect.DeepEqual(testServer, retrieved) {
		t.Errorf("Retrieved server %+v mismatch saved %+v", retrieved, testServer)
	}
}

func TestDeleteServer(t *testing.T) {
	setupTestDBGlobals(t)
	testServer := &Server{UUID: uuid.NewString(), Name: "DeleteServer"}
	err := saveServer(testServer)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	err = deleteServer(testServer.UUID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = getServer(testServer.UUID)
	if err == nil {
		t.Errorf("getServer succeeded after delete, expected error")
	} else if !errors.Is(err, ErrNotFound) && !errors.Is(err, badger.ErrKeyNotFound) {
		t.Errorf("Expected ErrNotFound or badger.ErrKeyNotFound, but got: %v", err)
	}
}

func TestListServers(t *testing.T) {
	setupTestDBGlobals(t)
	server1 := &Server{UUID: uuid.NewString(), Name: "ListServer1", Hostname: "s1"}
	server2 := &Server{UUID: uuid.NewString(), Name: "ListServer2", Hostname: "s2"}
	err := saveServer(server1)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	err = saveServer(server2)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	servers, err := listServers()
	if err != nil {
		t.Fatalf("listServers failed: %v", err)
	}
	if len(servers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(servers))
	}
}

// --- Token Tests ---

func TestSaveAndGetToken(t *testing.T) {
	setupTestDBGlobals(t)
	testToken := &AuthToken{
		UserUUID:   uuid.NewString(),
		TokenUUID:  uuid.NewString(),
		CreatedAt:  time.Now().Truncate(time.Second), // Truncate for comparison
		DeviceName: "Test Device",
	}
	err := saveToken(testToken)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	retrieved, err := getToken(testToken.TokenUUID)
	if err != nil {
		t.Fatalf("getToken failed: %v", err)
	}

	// Truncate retrieved time too before comparison
	retrieved.CreatedAt = retrieved.CreatedAt.Truncate(time.Second)

	if !reflect.DeepEqual(testToken, retrieved) {
		savedJSON, _ := json.Marshal(testToken)
		retrievedJSON, _ := json.Marshal(retrieved)
		t.Errorf("Retrieved token %s mismatch saved %s", retrievedJSON, savedJSON)
	}
}

func TestDeleteToken(t *testing.T) {
	setupTestDBGlobals(t)
	testToken := &AuthToken{TokenUUID: uuid.NewString(), UserUUID: uuid.NewString()}
	err := saveToken(testToken)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	err = deleteToken(testToken.TokenUUID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = getToken(testToken.TokenUUID)
	if err == nil {
		t.Errorf("getToken succeeded after delete, expected error")
	} else if !errors.Is(err, ErrNotFound) && !errors.Is(err, badger.ErrKeyNotFound) {
		t.Errorf("Expected ErrNotFound or badger.ErrKeyNotFound, but got: %v", err)
	}
}

func TestDeleteAllUserTokens(t *testing.T) {
	setupTestDBGlobals(t)

	user1UUID := uuid.NewString()
	user2UUID := uuid.NewString()

	tok1a := &AuthToken{TokenUUID: uuid.NewString(), UserUUID: user1UUID, DeviceName: "dev1a"}
	tok1b := &AuthToken{TokenUUID: uuid.NewString(), UserUUID: user1UUID, DeviceName: "dev1b"}
	tok2a := &AuthToken{TokenUUID: uuid.NewString(), UserUUID: user2UUID, DeviceName: "dev2a"}

	err := saveToken(tok1a)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	err = saveToken(tok1b)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	err = saveToken(tok2a)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Delete all tokens for user1
	err = deleteAllUserTokens(user1UUID)
	if err != nil {
		t.Fatalf("deleteAllUserTokens failed for user1: %v", err)
	}

	// Verify user1 tokens are gone
	_, err = getToken(tok1a.TokenUUID)
	if err == nil || (!errors.Is(err, ErrNotFound) && !errors.Is(err, badger.ErrKeyNotFound)) {
		t.Errorf("Token %s for user1 should be deleted, but retrieval error was: %v", tok1a.TokenUUID, err)
	}
	_, err = getToken(tok1b.TokenUUID)
	if err == nil || (!errors.Is(err, ErrNotFound) && !errors.Is(err, badger.ErrKeyNotFound)) {
		t.Errorf("Token %s for user1 should be deleted, but retrieval error was: %v", tok1b.TokenUUID, err)
	}

	// Verify user2 token still exists
	retrieved2a, err := getToken(tok2a.TokenUUID)
	if err != nil {
		t.Errorf("Token %s for user2 should still exist, but got error: %v", tok2a.TokenUUID, err)
	}
	if retrieved2a != nil && retrieved2a.UserUUID != user2UUID {
		t.Errorf("Retrieved token has wrong user UUID, expected %s got %s", user2UUID, retrieved2a.UserUUID)
	}

}

// --- Generic Helper Tests ---

func TestGenericGetItemNotFound(t *testing.T) {
	setupTestDBGlobals(t)
	var dummy User // Using User as a sample struct
	err := getItem(userDB, []byte("non:existent:key"), &dummy)
	if err == nil {
		t.Errorf("Expected error for non-existent key, got nil")
	} else if !errors.Is(err, badger.ErrKeyNotFound) {
		t.Errorf("Expected badger.ErrKeyNotFound, got %v", err)
	}
}
