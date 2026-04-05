package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	err := ConnectToBBoltDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { BBoltDB.Close() })
}

func testUser(email, apiKey string) *User {
	return &User{
		ID:     uuid.New(),
		Email:  email,
		APIKey: apiKey,
	}
}

// ---------------------------------------------------------------------------
// networkKey
// ---------------------------------------------------------------------------

func TestNetworkKey(t *testing.T) {
	key, err := networkKey("10.0.0.0/24")
	if err != nil {
		t.Fatal(err)
	}
	if len(key) != 4 {
		t.Fatalf("expected 4-byte key, got %d", len(key))
	}
	if key[0] != 10 || key[1] != 0 || key[2] != 0 || key[3] != 0 {
		t.Fatalf("unexpected key bytes: %v", key)
	}

	key2, err := networkKey("192.168.1.0/24")
	if err != nil {
		t.Fatal(err)
	}
	if key2[0] != 192 || key2[1] != 168 || key2[2] != 1 || key2[3] != 0 {
		t.Fatalf("unexpected key bytes: %v", key2)
	}

	_, err = networkKey("not-a-cidr")
	if err == nil {
		t.Fatal("expected error for invalid CIDR")
	}

	key6, err := networkKey("fd00::/64")
	if err != nil {
		t.Fatal("IPv6 should be supported:", err)
	}
	if len(key6) != 16 {
		t.Fatalf("expected 16-byte key for IPv6, got %d", len(key6))
	}
}

// ---------------------------------------------------------------------------
// ConnectToBBoltDB
// ---------------------------------------------------------------------------

func TestConnectToBBoltDB(t *testing.T) {
	setupTestDB(t)
	// Verify DB is usable.
	if err := BBolt_CreateGroup(&Group{ID: uuid.New(), Tag: "smoke"}); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// User CRUD
// ---------------------------------------------------------------------------

func TestBBolt_CreateUser(t *testing.T) {
	setupTestDB(t)
	u := testUser("test@example.com", "apikey123")
	if err := BBolt_CreateUser(u); err != nil {
		t.Fatal(err)
	}

	found, err := BBolt_findUserByID(u.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found.Email != "test@example.com" {
		t.Fatal("user not found by ID or email mismatch")
	}

	found, err = BBolt_findUserByEmail("test@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found.ID != u.ID {
		t.Fatal("user not found by email index")
	}

	found, err = BBolt_findUserByAPIKey("apikey123")
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found.ID != u.ID {
		t.Fatal("user not found by apikey index")
	}
}

func TestBBolt_CreateUser_NoAPIKey(t *testing.T) {
	setupTestDB(t)
	u := testUser("nokey@example.com", "")
	if err := BBolt_CreateUser(u); err != nil {
		t.Fatal(err)
	}

	found, _ := BBolt_findUserByAPIKey("")
	if found != nil {
		t.Fatal("empty apikey should not match")
	}
}

func TestBBolt_findUserByID_NotFound(t *testing.T) {
	setupTestDB(t)
	found, err := BBolt_findUserByID(uuid.New().String())
	if err != nil {
		t.Fatal(err)
	}
	if found != nil {
		t.Fatal("expected nil")
	}
}

func TestBBolt_findUserByEmail_NotFound(t *testing.T) {
	setupTestDB(t)
	found, err := BBolt_findUserByEmail("ghost@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if found != nil {
		t.Fatal("expected nil")
	}
}

func TestBBolt_findUserByAPIKey_NotFound(t *testing.T) {
	setupTestDB(t)
	found, err := BBolt_findUserByAPIKey("no-such-key")
	if err != nil {
		t.Fatal(err)
	}
	if found != nil {
		t.Fatal("expected nil")
	}
}

func TestBBolt_getUsers(t *testing.T) {
	setupTestDB(t)
	for i := 0; i < 5; i++ {
		if err := BBolt_CreateUser(testUser(fmt.Sprintf("u%d@example.com", i), "")); err != nil {
			t.Fatal(err)
		}
	}

	users, _ := BBolt_getUsers(10, 0)
	if len(users) != 5 {
		t.Fatalf("expected 5, got %d", len(users))
	}

	users, _ = BBolt_getUsers(3, 0)
	if len(users) != 3 {
		t.Fatalf("expected 3, got %d", len(users))
	}

	users, _ = BBolt_getUsers(10, 3)
	if len(users) != 2 {
		t.Fatalf("expected 2, got %d", len(users))
	}

	users, _ = BBolt_getUsers(10, 10)
	if len(users) != 0 {
		t.Fatalf("expected 0, got %d", len(users))
	}
}

func TestBBolt_updateUserDeviceTokens(t *testing.T) {
	setupTestDB(t)
	u := testUser("tokens@example.com", "")
	BBolt_CreateUser(u)

	tokens := []*DeviceToken{
		{DT: "t1", N: "d1", Created: time.Now()},
		{DT: "t2", N: "d2", Created: time.Now()},
	}
	if err := BBolt_updateUserDeviceTokens(&UPDATE_USER_TOKENS{ID: u.ID, Tokens: tokens}); err != nil {
		t.Fatal(err)
	}

	found, _ := BBolt_findUserByID(u.ID.String())
	if len(found.Tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(found.Tokens))
	}
}

func TestBBolt_updateUserDeviceTokens_NotFound(t *testing.T) {
	setupTestDB(t)
	err := BBolt_updateUserDeviceTokens(&UPDATE_USER_TOKENS{ID: uuid.New()})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBBolt_updateUserSubTime(t *testing.T) {
	setupTestDB(t)
	u := testUser("sub@example.com", "")
	BBolt_CreateUser(u)

	exp := time.Now().Add(30 * 24 * time.Hour).Truncate(time.Second)
	if err := BBolt_updateUserSubTime(&User{Email: "sub@example.com", SubExpiration: exp}); err != nil {
		t.Fatal(err)
	}

	found, _ := BBolt_findUserByID(u.ID.String())
	if !found.SubExpiration.Truncate(time.Second).Equal(exp) {
		t.Fatalf("expected %v, got %v", exp, found.SubExpiration)
	}
}

func TestBBolt_updateUserSubTime_NotFound(t *testing.T) {
	setupTestDB(t)
	err := BBolt_updateUserSubTime(&User{Email: "nobody@example.com"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBBolt_updateUser(t *testing.T) {
	setupTestDB(t)
	u := testUser("update@example.com", "oldkey")
	BBolt_CreateUser(u)

	if err := BBolt_updateUser(&USER_UPDATE_FORM{
		UID:                   u.ID,
		APIKey:                "newkey",
		AdditionalInformation: "info",
	}); err != nil {
		t.Fatal(err)
	}

	// Old key gone.
	found, _ := BBolt_findUserByAPIKey("oldkey")
	if found != nil {
		t.Fatal("old key should not resolve")
	}

	// New key works.
	found, _ = BBolt_findUserByAPIKey("newkey")
	if found == nil {
		t.Fatal("new key should resolve")
	}
	if found.AdditionalInformation != "info" {
		t.Fatalf("expected 'info', got '%s'", found.AdditionalInformation)
	}
}

func TestBBolt_updateUser_ClearAPIKey(t *testing.T) {
	setupTestDB(t)
	u := testUser("clear@example.com", "mykey")
	BBolt_CreateUser(u)

	BBolt_updateUser(&USER_UPDATE_FORM{UID: u.ID, APIKey: ""})

	found, _ := BBolt_findUserByAPIKey("mykey")
	if found != nil {
		t.Fatal("cleared key should not resolve")
	}
}

func TestBBolt_updateUser_NotFound(t *testing.T) {
	setupTestDB(t)
	err := BBolt_updateUser(&USER_UPDATE_FORM{UID: uuid.New()})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBBolt_updateUserAdmin(t *testing.T) {
	setupTestDB(t)
	u := testUser("admin@example.com", "")
	BBolt_CreateUser(u)

	exp := time.Now().Add(60 * 24 * time.Hour).Truncate(time.Second)
	if err := BBolt_updateUserAdmin(&USER_ADMIN_UPDATE_FORM{
		TargetUserID:  u.ID,
		Email:         "new@example.com",
		Disabled:      true,
		IsManager:     true,
		Trial:         true,
		SubExpiration: exp,
	}); err != nil {
		t.Fatal(err)
	}

	// Old email gone.
	found, _ := BBolt_findUserByEmail("admin@example.com")
	if found != nil {
		t.Fatal("old email should not resolve")
	}

	// New email works.
	found, _ = BBolt_findUserByEmail("new@example.com")
	if found == nil {
		t.Fatal("new email should resolve")
	}
	if !found.Disabled || !found.IsManager || !found.Trial {
		t.Fatal("flags not set")
	}
	if !found.SubExpiration.Truncate(time.Second).Equal(exp) {
		t.Fatal("sub expiration mismatch")
	}
}

func TestBBolt_updateUserAdmin_SameEmail(t *testing.T) {
	setupTestDB(t)
	u := testUser("same@example.com", "")
	BBolt_CreateUser(u)

	BBolt_updateUserAdmin(&USER_ADMIN_UPDATE_FORM{
		TargetUserID: u.ID,
		Email:        "same@example.com",
		Disabled:     true,
	})

	found, _ := BBolt_findUserByEmail("same@example.com")
	if found == nil {
		t.Fatal("email index should survive same-email update")
	}
}

func TestBBolt_updateUserAdmin_EmptyEmail(t *testing.T) {
	setupTestDB(t)
	u := testUser("keep@example.com", "")
	BBolt_CreateUser(u)

	BBolt_updateUserAdmin(&USER_ADMIN_UPDATE_FORM{
		TargetUserID: u.ID,
		Email:        "",
		Disabled:     true,
	})

	found, _ := BBolt_findUserByEmail("keep@example.com")
	if found == nil {
		t.Fatal("original email should survive empty-email update")
	}
	if !found.Disabled {
		t.Fatal("Disabled not set")
	}
}

func TestBBolt_updateUserAdmin_NotFound(t *testing.T) {
	setupTestDB(t)
	err := BBolt_updateUserAdmin(&USER_ADMIN_UPDATE_FORM{TargetUserID: uuid.New()})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBBolt_toggleUserSubscriptionStatus(t *testing.T) {
	setupTestDB(t)
	u := testUser("toggle@example.com", "")
	BBolt_CreateUser(u)

	if err := BBolt_toggleUserSubscriptionStatus(&USER_UPDATE_SUB_FORM{Email: "toggle@example.com"}); err != nil {
		t.Fatal(err)
	}

	found, _ := BBolt_findUserByEmail("toggle@example.com")
	if found == nil {
		t.Fatal("user should still exist")
	}
}

func TestBBolt_toggleUserSubscriptionStatus_NotFound(t *testing.T) {
	setupTestDB(t)
	err := BBolt_toggleUserSubscriptionStatus(&USER_UPDATE_SUB_FORM{Email: "nope@example.com"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBBolt_userUpdateTwoFactorCodes(t *testing.T) {
	setupTestDB(t)
	u := testUser("2fa@example.com", "")
	BBolt_CreateUser(u)

	if err := BBolt_userUpdateTwoFactorCodes(&TWO_FACTOR_DB_PACKAGE{
		UID:      u.ID,
		Code:     []byte("code"),
		Recovery: []byte("recovery"),
	}); err != nil {
		t.Fatal(err)
	}

	found, _ := BBolt_findUserByID(u.ID.String())
	if !found.TwoFactorEnabled {
		t.Fatal("TwoFactorEnabled not set")
	}
	if string(found.TwoFactorCode) != "code" {
		t.Fatalf("expected 'code', got '%s'", found.TwoFactorCode)
	}
	if string(found.RecoveryCodes) != "recovery" {
		t.Fatalf("expected 'recovery', got '%s'", found.RecoveryCodes)
	}
}

func TestBBolt_userUpdateTwoFactorCodes_NotFound(t *testing.T) {
	setupTestDB(t)
	err := BBolt_userUpdateTwoFactorCodes(&TWO_FACTOR_DB_PACKAGE{UID: uuid.New()})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBBolt_userResetPassword(t *testing.T) {
	setupTestDB(t)
	u := testUser("reset@example.com", "")
	u.Password = "old"
	u.Tokens = []*DeviceToken{{DT: "t", N: "n"}}
	BBolt_CreateUser(u)

	if err := BBolt_userResetPassword(&User{ID: u.ID, Password: "new"}); err != nil {
		t.Fatal(err)
	}

	found, _ := BBolt_findUserByID(u.ID.String())
	if found.Password != "new" {
		t.Fatalf("expected 'new', got '%s'", found.Password)
	}
	if len(found.Tokens) != 0 {
		t.Fatalf("expected 0 tokens, got %d", len(found.Tokens))
	}
}

func TestBBolt_userResetPassword_NotFound(t *testing.T) {
	setupTestDB(t)
	err := BBolt_userResetPassword(&User{ID: uuid.New()})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBBolt_WipeUserConfirmCode(t *testing.T) {
	setupTestDB(t)
	u := testUser("confirm@example.com", "")
	u.ConfirmCode = "abc123"
	BBolt_CreateUser(u)

	if err := BBolt_WipeUserConfirmCode(&USER_ENABLE_QUERY{Email: "confirm@example.com"}); err != nil {
		t.Fatal(err)
	}

	found, _ := BBolt_findUserByID(u.ID.String())
	if found.ConfirmCode != "" {
		t.Fatalf("expected empty, got '%s'", found.ConfirmCode)
	}
}

func TestBBolt_WipeUserConfirmCode_NotFound(t *testing.T) {
	setupTestDB(t)
	err := BBolt_WipeUserConfirmCode(&USER_ENABLE_QUERY{Email: "ghost@example.com"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBBolt_UserActivateKey(t *testing.T) {
	setupTestDB(t)
	u := testUser("activate@example.com", "")
	u.Disabled = true
	u.Trial = true
	BBolt_CreateUser(u)

	exp := time.Now().Add(365 * 24 * time.Hour).Truncate(time.Second)
	key := &LicenseKey{Created: time.Now(), Months: 12, Key: "LIC-KEY"}
	if err := BBolt_UserActivateKey(exp, key, u.ID.String()); err != nil {
		t.Fatal(err)
	}

	found, _ := BBolt_findUserByID(u.ID.String())
	if found.Disabled || found.Trial {
		t.Fatal("Disabled/Trial should be false")
	}
	if found.Key == nil || found.Key.Key != "LIC-KEY" {
		t.Fatal("license key not set")
	}
	if !found.SubExpiration.Truncate(time.Second).Equal(exp) {
		t.Fatal("sub expiration mismatch")
	}
}

func TestBBolt_UserActivateKey_NotFound(t *testing.T) {
	setupTestDB(t)
	err := BBolt_UserActivateKey(time.Now(), nil, uuid.New().String())
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Device CRUD
// ---------------------------------------------------------------------------

func TestBBolt_CreateDevice(t *testing.T) {
	setupTestDB(t)
	uid := uuid.New()
	d := &types.Device{ID: uuid.New(), Tag: "dev1", UserID: uid}
	if err := BBolt_CreateDevice(d); err != nil {
		t.Fatal(err)
	}

	found, _ := BBolt_FindDeviceByID(d.ID.String())
	if found == nil || found.Tag != "dev1" {
		t.Fatal("device not found or tag mismatch")
	}

	devices, _ := BBolt_GetDevicesByUserID(uid)
	if len(devices) != 1 || devices[0].ID != d.ID {
		t.Fatal("device not found via user ID index")
	}
}

func TestBBolt_FindDeviceByID_NotFound(t *testing.T) {
	setupTestDB(t)
	found, err := BBolt_FindDeviceByID(uuid.New().String())
	if err != nil {
		t.Fatal(err)
	}
	if found != nil {
		t.Fatal("expected nil")
	}
}

func TestBBolt_GetDevicesByUserID_Multiple(t *testing.T) {
	setupTestDB(t)
	u1, u2 := uuid.New(), uuid.New()

	for i := 0; i < 3; i++ {
		BBolt_CreateDevice(&types.Device{ID: uuid.New(), UserID: u1, Tag: fmt.Sprintf("u1-%d", i)})
	}
	for i := 0; i < 2; i++ {
		BBolt_CreateDevice(&types.Device{ID: uuid.New(), UserID: u2, Tag: fmt.Sprintf("u2-%d", i)})
	}

	d1, _ := BBolt_GetDevicesByUserID(u1)
	if len(d1) != 3 {
		t.Fatalf("expected 3, got %d", len(d1))
	}

	d2, _ := BBolt_GetDevicesByUserID(u2)
	if len(d2) != 2 {
		t.Fatalf("expected 2, got %d", len(d2))
	}

	d3, _ := BBolt_GetDevicesByUserID(uuid.New())
	if len(d3) != 0 {
		t.Fatalf("expected 0, got %d", len(d3))
	}
}

func TestBBolt_UpdateDevice(t *testing.T) {
	setupTestDB(t)
	uid := uuid.New()
	d := &types.Device{ID: uuid.New(), UserID: uid, Tag: "orig"}
	BBolt_CreateDevice(d)

	d.Tag = "updated"
	if err := BBolt_UpdateDevice(d); err != nil {
		t.Fatal(err)
	}

	found, _ := BBolt_FindDeviceByID(d.ID.String())
	if found.Tag != "updated" {
		t.Fatalf("expected 'updated', got '%s'", found.Tag)
	}

	devices, _ := BBolt_GetDevicesByUserID(uid)
	if len(devices) != 1 {
		t.Fatalf("expected 1, got %d", len(devices))
	}
}

func TestBBolt_UpdateDevice_ChangeUserID(t *testing.T) {
	setupTestDB(t)
	u1, u2 := uuid.New(), uuid.New()
	d := &types.Device{ID: uuid.New(), UserID: u1, Tag: "move"}
	BBolt_CreateDevice(d)

	d.UserID = u2
	BBolt_UpdateDevice(d)

	d1, _ := BBolt_GetDevicesByUserID(u1)
	if len(d1) != 0 {
		t.Fatalf("expected 0 for old user, got %d", len(d1))
	}

	d2, _ := BBolt_GetDevicesByUserID(u2)
	if len(d2) != 1 {
		t.Fatalf("expected 1 for new user, got %d", len(d2))
	}
}

func TestBBolt_DeleteDeviceByID(t *testing.T) {
	setupTestDB(t)
	uid := uuid.New()
	d := &types.Device{ID: uuid.New(), UserID: uid}
	BBolt_CreateDevice(d)

	if err := BBolt_DeleteDeviceByID(d.ID.String()); err != nil {
		t.Fatal(err)
	}

	found, _ := BBolt_FindDeviceByID(d.ID.String())
	if found != nil {
		t.Fatal("device should be deleted")
	}

	devices, _ := BBolt_GetDevicesByUserID(uid)
	if len(devices) != 0 {
		t.Fatalf("expected 0 after delete, got %d", len(devices))
	}
}

func TestBBolt_DeleteDeviceByID_NonExistent(t *testing.T) {
	setupTestDB(t)
	if err := BBolt_DeleteDeviceByID(uuid.New().String()); err != nil {
		t.Fatal(err)
	}
}

func TestBBolt_GetDevices(t *testing.T) {
	setupTestDB(t)
	for i := 0; i < 5; i++ {
		BBolt_CreateDevice(&types.Device{ID: uuid.New(), UserID: uuid.New()})
	}

	dl, _ := BBolt_GetDevices(10, 0)
	if len(dl) != 5 {
		t.Fatalf("expected 5, got %d", len(dl))
	}

	dl, _ = BBolt_GetDevices(3, 0)
	if len(dl) != 3 {
		t.Fatalf("expected 3, got %d", len(dl))
	}

	dl, _ = BBolt_GetDevices(10, 3)
	if len(dl) != 2 {
		t.Fatalf("expected 2, got %d", len(dl))
	}
}

// ---------------------------------------------------------------------------
// Server CRUD
// ---------------------------------------------------------------------------

func TestBBolt_CreateServer(t *testing.T) {
	setupTestDB(t)
	s := &types.Server{ID: uuid.New(), Tag: "srv", Country: "US", IP: "1.2.3.4", Port: "443"}
	if err := BBolt_CreateServer(s); err != nil {
		t.Fatal(err)
	}

	found, _ := BBolt_FindServerByID(s.ID.String())
	if found == nil || found.Tag != "srv" {
		t.Fatal("server not found or tag mismatch")
	}
}

func TestBBolt_FindServerByID_NotFound(t *testing.T) {
	setupTestDB(t)
	found, err := BBolt_FindServerByID(uuid.New().String())
	if err != nil {
		t.Fatal(err)
	}
	if found != nil {
		t.Fatal("expected nil")
	}
}

func TestBBolt_UpdateServer(t *testing.T) {
	setupTestDB(t)
	s := &types.Server{ID: uuid.New(), Tag: "orig", Country: "US", IP: "1.2.3.4", Port: "443"}
	BBolt_CreateServer(s)

	updated, err := BBolt_UpdateServer(&types.Server{
		ID:              s.ID,
		Tag:             "new",
		Country:         "UK",
		IP:              "5.6.7.8",
		Port:            "8443",
		WireGuardPort:   "51820",
		WireGuardPubKey: "pubkey-abc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Tag != "new" || updated.Country != "UK" || updated.IP != "5.6.7.8" || updated.Port != "8443" {
		t.Fatal("basic field update mismatch")
	}
	if updated.WireGuardPort != "51820" || updated.WireGuardPubKey != "pubkey-abc" {
		t.Fatalf("wireguard fields mismatch: port=%s pubkey=%s", updated.WireGuardPort, updated.WireGuardPubKey)
	}

	// Verify persisted.
	found, _ := BBolt_FindServerByID(s.ID.String())
	if found.WireGuardPort != "51820" || found.WireGuardPubKey != "pubkey-abc" {
		t.Fatal("wireguard fields not persisted")
	}
}

func TestBBolt_UpdateServer_NotFound(t *testing.T) {
	setupTestDB(t)
	_, err := BBolt_UpdateServer(&types.Server{ID: uuid.New()})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBBolt_FindAllServers(t *testing.T) {
	setupTestDB(t)
	for i := 0; i < 3; i++ {
		BBolt_CreateServer(&types.Server{ID: uuid.New(), Tag: fmt.Sprintf("s%d", i)})
	}

	servers, err := BBolt_FindAllServers()
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 3 {
		t.Fatalf("expected 3, got %d", len(servers))
	}
}

func TestBBolt_FindServersWithoutGroups(t *testing.T) {
	setupTestDB(t)
	for i := 0; i < 3; i++ {
		BBolt_CreateServer(&types.Server{ID: uuid.New(), Tag: fmt.Sprintf("no-groups-%d", i)})
	}
	BBolt_CreateServer(&types.Server{ID: uuid.New(), Tag: "has-groups", Groups: []uuid.UUID{uuid.New()}})

	servers, _ := BBolt_FindServersWithoutGroups(10, 0)
	if len(servers) != 3 {
		t.Fatalf("expected 3, got %d", len(servers))
	}

	// Limit.
	servers, _ = BBolt_FindServersWithoutGroups(2, 0)
	if len(servers) != 2 {
		t.Fatalf("expected 2 with limit, got %d", len(servers))
	}

	// Offset.
	servers, _ = BBolt_FindServersWithoutGroups(10, 2)
	if len(servers) != 1 {
		t.Fatalf("expected 1 with offset, got %d", len(servers))
	}
}

func TestBBolt_FindServersByGroups(t *testing.T) {
	setupTestDB(t)
	g1, g2 := uuid.New(), uuid.New()
	BBolt_CreateServer(&types.Server{ID: uuid.New(), Tag: "in-g1", Groups: []uuid.UUID{g1}})
	BBolt_CreateServer(&types.Server{ID: uuid.New(), Tag: "in-g2", Groups: []uuid.UUID{g2}})
	BBolt_CreateServer(&types.Server{ID: uuid.New(), Tag: "both", Groups: []uuid.UUID{g1, g2}})
	BBolt_CreateServer(&types.Server{ID: uuid.New(), Tag: "none"})

	s1, _ := BBolt_FindServersByGroups([]string{g1.String()}, 10, 0)
	if len(s1) != 2 {
		t.Fatalf("expected 2 in g1, got %d", len(s1))
	}

	s2, _ := BBolt_FindServersByGroups([]string{g2.String()}, 10, 0)
	if len(s2) != 2 {
		t.Fatalf("expected 2 in g2, got %d", len(s2))
	}

	sAll, _ := BBolt_FindServersByGroups([]string{g1.String(), g2.String()}, 10, 0)
	if len(sAll) != 3 {
		t.Fatalf("expected 3 in g1|g2, got %d", len(sAll))
	}

	// Pagination — use single-group search for deterministic skip behavior.
	// g1 matches 2 servers: in-g1 and both.
	sLim, _ := BBolt_FindServersByGroups([]string{g1.String()}, 1, 0)
	if len(sLim) != 1 {
		t.Fatalf("expected 1 with limit, got %d", len(sLim))
	}

	sOff, _ := BBolt_FindServersByGroups([]string{g1.String()}, 10, 1)
	if len(sOff) != 1 {
		t.Fatalf("expected 1 with offset, got %d", len(sOff))
	}
}

func TestBBolt_SetServerWGSubnet(t *testing.T) {
	setupTestDB(t)
	s := &types.Server{ID: uuid.New(), Tag: "wg"}
	BBolt_CreateServer(s)

	if err := BBolt_SetServerWGSubnet(s.ID.String(), "10.0.0.0/24"); err != nil {
		t.Fatal(err)
	}

	found, _ := BBolt_FindServerByID(s.ID.String())
	if found.WireGuardSubnet != "10.0.0.0/24" {
		t.Fatalf("expected '10.0.0.0/24', got '%s'", found.WireGuardSubnet)
	}
}

func TestBBolt_SetServerWGSubnet_NotFound(t *testing.T) {
	setupTestDB(t)
	err := BBolt_SetServerWGSubnet(uuid.New().String(), "10.0.0.0/24")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBBolt_SetServerWGConfigID(t *testing.T) {
	setupTestDB(t)
	s := &types.Server{ID: uuid.New(), Tag: "link"}
	BBolt_CreateServer(s)

	wgCfg := &types.WGServerConfig{ID: uuid.New(), WireGuardPort: 51820}
	if err := BBolt_SetServerWGConfigID(s.ID.String(), wgCfg, "pub123", "10.0.0.0/24", "fd00::/64"); err != nil {
		t.Fatal(err)
	}

	found, _ := BBolt_FindServerByID(s.ID.String())
	if found.WGConfigID != wgCfg.ID {
		t.Fatal("WGConfigID mismatch")
	}
	if found.WireGuardPubKey != "pub123" {
		t.Fatal("pub key mismatch")
	}
	if found.WireGuardSubnet != "10.0.0.0/24" {
		t.Fatal("subnet mismatch")
	}
	if found.WireGuardSubnet6 != "fd00::/64" {
		t.Fatalf("expected subnet6 'fd00::/64', got '%s'", found.WireGuardSubnet6)
	}
	if found.WireGuardPort != "51820" {
		t.Fatalf("expected port '51820', got '%s'", found.WireGuardPort)
	}
}

func TestBBolt_SetServerWGConfigID_NotFound(t *testing.T) {
	setupTestDB(t)
	wgCfg := &types.WGServerConfig{ID: uuid.New(), WireGuardPort: 51820}
	err := BBolt_SetServerWGConfigID(uuid.New().String(), wgCfg, "pk", "10.0.0.0/24", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Group CRUD
// ---------------------------------------------------------------------------

func TestBBolt_CreateGroup(t *testing.T) {
	setupTestDB(t)
	g := &Group{ID: uuid.New(), Tag: "grp", Description: "desc"}
	if err := BBolt_CreateGroup(g); err != nil {
		t.Fatal(err)
	}

	found, _ := BBolt_findGroupByID(g.ID.String())
	if found == nil || found.Tag != "grp" {
		t.Fatal("group not found or tag mismatch")
	}
}

func TestBBolt_findGroupByID_NotFound(t *testing.T) {
	setupTestDB(t)
	found, err := BBolt_findGroupByID(uuid.New().String())
	if err != nil {
		t.Fatal(err)
	}
	if found != nil {
		t.Fatal("expected nil")
	}
}

func TestBBolt_UpdateGroup(t *testing.T) {
	setupTestDB(t)
	g := &Group{ID: uuid.New(), Tag: "orig", Description: "old"}
	BBolt_CreateGroup(g)

	if err := BBolt_UpdateGroup(&Group{ID: g.ID, Tag: "new", Description: "fresh"}); err != nil {
		t.Fatal(err)
	}

	found, _ := BBolt_findGroupByID(g.ID.String())
	if found.Tag != "new" || found.Description != "fresh" {
		t.Fatal("update mismatch")
	}
}

func TestBBolt_UpdateGroup_NotFound(t *testing.T) {
	setupTestDB(t)
	err := BBolt_UpdateGroup(&Group{ID: uuid.New()})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBBolt_DeleteGroupByID(t *testing.T) {
	setupTestDB(t)
	g := &Group{ID: uuid.New(), Tag: "del"}
	BBolt_CreateGroup(g)

	if err := BBolt_DeleteGroupByID(g.ID.String()); err != nil {
		t.Fatal(err)
	}

	found, _ := BBolt_findGroupByID(g.ID.String())
	if found != nil {
		t.Fatal("group should be deleted")
	}
}

func TestBBolt_findGroups(t *testing.T) {
	setupTestDB(t)
	for i := 0; i < 3; i++ {
		BBolt_CreateGroup(&Group{ID: uuid.New(), Tag: fmt.Sprintf("g%d", i)})
	}

	groups, err := BBolt_findGroups()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 3 {
		t.Fatalf("expected 3, got %d", len(groups))
	}
}

func TestBBolt_AddToGroup_User(t *testing.T) {
	setupTestDB(t)
	g := &Group{ID: uuid.New(), Tag: "ug"}
	BBolt_CreateGroup(g)
	u := testUser("grp@example.com", "")
	BBolt_CreateUser(u)

	if err := BBolt_AddToGroup(g.ID.String(), u.ID.String(), "user"); err != nil {
		t.Fatal(err)
	}

	found, _ := BBolt_findUserByID(u.ID.String())
	if !slices.Contains(uuidSliceToString(found.Groups), g.ID.String()) {
		t.Fatal("user not in group")
	}

	// Idempotent.
	BBolt_AddToGroup(g.ID.String(), u.ID.String(), "user")
	found, _ = BBolt_findUserByID(u.ID.String())
	if len(found.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(found.Groups))
	}
}

func TestBBolt_AddToGroup_Server(t *testing.T) {
	setupTestDB(t)
	g := &Group{ID: uuid.New(), Tag: "sg"}
	BBolt_CreateGroup(g)
	s := &types.Server{ID: uuid.New(), Tag: "srv"}
	BBolt_CreateServer(s)

	if err := BBolt_AddToGroup(g.ID.String(), s.ID.String(), "server"); err != nil {
		t.Fatal(err)
	}

	found, _ := BBolt_FindServerByID(s.ID.String())
	if !slices.Contains(uuidSliceToString(found.Groups), g.ID.String()) {
		t.Fatal("server not in group")
	}
}

func TestBBolt_AddToGroup_Device(t *testing.T) {
	setupTestDB(t)
	g := &Group{ID: uuid.New(), Tag: "dg"}
	BBolt_CreateGroup(g)
	d := &types.Device{ID: uuid.New(), UserID: uuid.New()}
	BBolt_CreateDevice(d)

	if err := BBolt_AddToGroup(g.ID.String(), d.ID.String(), "device"); err != nil {
		t.Fatal(err)
	}

	found, _ := BBolt_FindDeviceByID(d.ID.String())
	if !slices.Contains(uuidSliceToString(found.Groups), g.ID.String()) {
		t.Fatal("device not in group")
	}
}

func TestBBolt_AddToGroup_InvalidType(t *testing.T) {
	setupTestDB(t)
	err := BBolt_AddToGroup(uuid.New().String(), uuid.New().String(), "invalid")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBBolt_AddToGroup_NotFound(t *testing.T) {
	setupTestDB(t)
	err := BBolt_AddToGroup(uuid.New().String(), uuid.New().String(), "user")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBBolt_RemoveFromGroup_User(t *testing.T) {
	setupTestDB(t)
	g := &Group{ID: uuid.New(), Tag: "rg"}
	BBolt_CreateGroup(g)
	u := testUser("rm@example.com", "")
	BBolt_CreateUser(u)
	BBolt_AddToGroup(g.ID.String(), u.ID.String(), "user")

	if err := BBolt_RemoveFromGroup(g.ID.String(), u.ID.String(), "user"); err != nil {
		t.Fatal(err)
	}

	found, _ := BBolt_findUserByID(u.ID.String())
	if len(found.Groups) != 0 {
		t.Fatal("user should have 0 groups")
	}
}

func TestBBolt_RemoveFromGroup_Server(t *testing.T) {
	setupTestDB(t)
	g := &Group{ID: uuid.New(), Tag: "rsg"}
	BBolt_CreateGroup(g)
	s := &types.Server{ID: uuid.New(), Tag: "srv"}
	BBolt_CreateServer(s)
	BBolt_AddToGroup(g.ID.String(), s.ID.String(), "server")

	BBolt_RemoveFromGroup(g.ID.String(), s.ID.String(), "server")

	found, _ := BBolt_FindServerByID(s.ID.String())
	if len(found.Groups) != 0 {
		t.Fatal("server should have 0 groups")
	}
}

func TestBBolt_RemoveFromGroup_Device(t *testing.T) {
	setupTestDB(t)
	g := &Group{ID: uuid.New(), Tag: "rdg"}
	BBolt_CreateGroup(g)
	d := &types.Device{ID: uuid.New(), UserID: uuid.New()}
	BBolt_CreateDevice(d)
	BBolt_AddToGroup(g.ID.String(), d.ID.String(), "device")

	BBolt_RemoveFromGroup(g.ID.String(), d.ID.String(), "device")

	found, _ := BBolt_FindDeviceByID(d.ID.String())
	if len(found.Groups) != 0 {
		t.Fatal("device should have 0 groups")
	}
}

func TestBBolt_RemoveFromGroup_InvalidType(t *testing.T) {
	setupTestDB(t)
	err := BBolt_RemoveFromGroup(uuid.New().String(), uuid.New().String(), "invalid")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBBolt_RemoveFromGroup_NotFound(t *testing.T) {
	setupTestDB(t)
	gid := uuid.New().String()
	for _, typ := range []string{"user", "server", "device"} {
		err := BBolt_RemoveFromGroup(gid, uuid.New().String(), typ)
		if err == nil {
			t.Fatalf("expected error for non-existent %s", typ)
		}
	}
}

func TestBBolt_FindEntitiesByGroupID(t *testing.T) {
	setupTestDB(t)
	g := &Group{ID: uuid.New(), Tag: "find"}
	BBolt_CreateGroup(g)

	u1 := testUser("e1@example.com", "")
	u2 := testUser("e2@example.com", "")
	BBolt_CreateUser(u1)
	BBolt_CreateUser(u2)
	BBolt_AddToGroup(g.ID.String(), u1.ID.String(), "user")
	BBolt_AddToGroup(g.ID.String(), u2.ID.String(), "user")

	s := &types.Server{ID: uuid.New(), Tag: "srv"}
	BBolt_CreateServer(s)
	BBolt_AddToGroup(g.ID.String(), s.ID.String(), "server")

	d := &types.Device{ID: uuid.New(), UserID: uuid.New()}
	BBolt_CreateDevice(d)
	BBolt_AddToGroup(g.ID.String(), d.ID.String(), "device")

	entities, _ := BBolt_FindEntitiesByGroupID(g.ID.String(), "user", 10, 0)
	if len(entities) != 2 {
		t.Fatalf("expected 2 users, got %d", len(entities))
	}

	entities, _ = BBolt_FindEntitiesByGroupID(g.ID.String(), "server", 10, 0)
	if len(entities) != 1 {
		t.Fatalf("expected 1 server, got %d", len(entities))
	}

	entities, _ = BBolt_FindEntitiesByGroupID(g.ID.String(), "device", 10, 0)
	if len(entities) != 1 {
		t.Fatalf("expected 1 device, got %d", len(entities))
	}

	// Pagination.
	entities, _ = BBolt_FindEntitiesByGroupID(g.ID.String(), "user", 1, 0)
	if len(entities) != 1 {
		t.Fatalf("expected 1 with limit, got %d", len(entities))
	}

	entities, _ = BBolt_FindEntitiesByGroupID(g.ID.String(), "user", 10, 1)
	if len(entities) != 1 {
		t.Fatalf("expected 1 with offset, got %d", len(entities))
	}

	// Invalid type.
	_, err := BBolt_FindEntitiesByGroupID(g.ID.String(), "invalid", 10, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// WGServerConfig CRUD
// ---------------------------------------------------------------------------

func TestBBolt_CreateWGServerConfig(t *testing.T) {
	setupTestDB(t)
	cfg := &types.WGServerConfig{
		ID:            uuid.New(),
		Tag:           "wg",
		APIKey:        "wg-key",
		WireGuardPort: 51820,
	}
	if err := BBolt_CreateWGServerConfig(cfg); err != nil {
		t.Fatal(err)
	}

	found, _ := BBolt_FindWGServerConfigByID(cfg.ID.String())
	if found == nil || found.Tag != "wg" {
		t.Fatal("config not found or tag mismatch")
	}

	found, _ = BBolt_FindWGServerConfigByAPIKey("wg-key")
	if found == nil || found.ID != cfg.ID {
		t.Fatal("config not found by apikey index")
	}
}

func TestBBolt_CreateWGServerConfig_NoAPIKey(t *testing.T) {
	setupTestDB(t)
	cfg := &types.WGServerConfig{ID: uuid.New(), Tag: "no-key", WireGuardPort: 51820}
	if err := BBolt_CreateWGServerConfig(cfg); err != nil {
		t.Fatal(err)
	}

	// Findable by ID.
	found, _ := BBolt_FindWGServerConfigByID(cfg.ID.String())
	if found == nil {
		t.Fatal("config should be findable by ID")
	}

	// Empty key should not match.
	found, _ = BBolt_FindWGServerConfigByAPIKey("")
	if found != nil {
		t.Fatal("empty apikey should not match")
	}
}

func TestBBolt_FindWGServerConfigByID_NotFound(t *testing.T) {
	setupTestDB(t)
	found, err := BBolt_FindWGServerConfigByID(uuid.New().String())
	if err != nil {
		t.Fatal(err)
	}
	if found != nil {
		t.Fatal("expected nil")
	}
}

func TestBBolt_FindWGServerConfigByAPIKey_NotFound(t *testing.T) {
	setupTestDB(t)
	found, err := BBolt_FindWGServerConfigByAPIKey("nope")
	if err != nil {
		t.Fatal(err)
	}
	if found != nil {
		t.Fatal("expected nil")
	}
}

func TestBBolt_UpdateWGServerConfig(t *testing.T) {
	setupTestDB(t)
	cfg := &types.WGServerConfig{ID: uuid.New(), Tag: "orig", APIKey: "old", WireGuardPort: 51820}
	BBolt_CreateWGServerConfig(cfg)

	cfg.Tag = "new"
	cfg.APIKey = "fresh"
	if err := BBolt_UpdateWGServerConfig(cfg); err != nil {
		t.Fatal(err)
	}

	found, _ := BBolt_FindWGServerConfigByAPIKey("old")
	if found != nil {
		t.Fatal("old key should not resolve")
	}

	found, _ = BBolt_FindWGServerConfigByAPIKey("fresh")
	if found == nil || found.Tag != "new" {
		t.Fatal("new key should resolve with updated tag")
	}
}

func TestBBolt_UpdateWGServerConfig_ClearAPIKey(t *testing.T) {
	setupTestDB(t)
	cfg := &types.WGServerConfig{ID: uuid.New(), APIKey: "clear"}
	BBolt_CreateWGServerConfig(cfg)

	cfg.APIKey = ""
	BBolt_UpdateWGServerConfig(cfg)

	found, _ := BBolt_FindWGServerConfigByAPIKey("clear")
	if found != nil {
		t.Fatal("cleared key should not resolve")
	}
}

func TestBBolt_ListWGServerConfigs(t *testing.T) {
	setupTestDB(t)
	for i := 0; i < 3; i++ {
		BBolt_CreateWGServerConfig(&types.WGServerConfig{ID: uuid.New(), Tag: fmt.Sprintf("c%d", i)})
	}

	configs, err := BBolt_ListWGServerConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 3 {
		t.Fatalf("expected 3, got %d", len(configs))
	}
}

// ---------------------------------------------------------------------------
// Network CRUD
// ---------------------------------------------------------------------------

func TestBBolt_CreateNetworksBatch(t *testing.T) {
	setupTestDB(t)
	nets := []*Network{
		{ID: uuid.New(), CIDR: "10.0.0.0/24", Tag: "n1"},
		{ID: uuid.New(), CIDR: "10.0.1.0/24", Tag: "n2"},
		{ID: uuid.New(), CIDR: "10.0.2.0/24", Tag: "n3"},
	}
	if err := BBolt_CreateNetworksBatch(nets); err != nil {
		t.Fatal(err)
	}

	for _, n := range nets {
		found, err := BBolt_FindNetworkByID(n.ID)
		if err != nil {
			t.Fatal(err)
		}
		if found == nil || found.Tag != n.Tag {
			t.Fatalf("network %s not found or tag mismatch", n.Tag)
		}
	}

	count, _ := BBolt_CountNetworks()
	if count != 3 {
		t.Fatalf("expected 3, got %d", count)
	}
}

func TestBBolt_FindNetworkByID_NotFound(t *testing.T) {
	setupTestDB(t)
	found, err := BBolt_FindNetworkByID(uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if found != nil {
		t.Fatal("expected nil")
	}
}

func TestBBolt_GetNetworks(t *testing.T) {
	setupTestDB(t)
	nets := make([]*Network, 5)
	for i := range nets {
		nets[i] = &Network{ID: uuid.New(), CIDR: fmt.Sprintf("10.0.%d.0/24", i)}
	}
	BBolt_CreateNetworksBatch(nets)

	nl, _ := BBolt_GetNetworks(10, 0)
	if len(nl) != 5 {
		t.Fatalf("expected 5, got %d", len(nl))
	}

	nl, _ = BBolt_GetNetworks(3, 0)
	if len(nl) != 3 {
		t.Fatalf("expected 3, got %d", len(nl))
	}

	nl, _ = BBolt_GetNetworks(10, 3)
	if len(nl) != 2 {
		t.Fatalf("expected 2, got %d", len(nl))
	}
}

func TestBBolt_UpdateNetwork(t *testing.T) {
	setupTestDB(t)
	n := &Network{ID: uuid.New(), CIDR: "10.0.0.0/24", Tag: "orig"}
	BBolt_CreateNetworksBatch([]*Network{n})

	n.Tag = "updated"
	n.Description = "new desc"
	if err := BBolt_UpdateNetwork(n); err != nil {
		t.Fatal(err)
	}

	found, _ := BBolt_FindNetworkByID(n.ID)
	if found.Tag != "updated" || found.Description != "new desc" {
		t.Fatal("update mismatch")
	}
}

func TestBBolt_CountNetworks_Empty(t *testing.T) {
	setupTestDB(t)
	count, err := BBolt_CountNetworks()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// Empty collection returns
// ---------------------------------------------------------------------------

func TestBBolt_getUsers_Empty(t *testing.T) {
	setupTestDB(t)
	users, err := BBolt_getUsers(10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 0 {
		t.Fatalf("expected 0, got %d", len(users))
	}
}

func TestBBolt_GetDevices_Empty(t *testing.T) {
	setupTestDB(t)
	dl, err := BBolt_GetDevices(10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(dl) != 0 {
		t.Fatalf("expected 0, got %d", len(dl))
	}
}

func TestBBolt_FindAllServers_Empty(t *testing.T) {
	setupTestDB(t)
	servers, err := BBolt_FindAllServers()
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 0 {
		t.Fatalf("expected 0, got %d", len(servers))
	}
}

func TestBBolt_findGroups_Empty(t *testing.T) {
	setupTestDB(t)
	groups, err := BBolt_findGroups()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 {
		t.Fatalf("expected 0, got %d", len(groups))
	}
}

func TestBBolt_ListWGServerConfigs_Empty(t *testing.T) {
	setupTestDB(t)
	configs, err := BBolt_ListWGServerConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 0 {
		t.Fatalf("expected 0, got %d", len(configs))
	}
}

// ---------------------------------------------------------------------------
// Index backfill on reopen
// ---------------------------------------------------------------------------

func TestConnectToBBoltDB_BackfillIndexes(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	if err := ConnectToBBoltDB(dbPath); err != nil {
		t.Fatal(err)
	}

	u := testUser("bf@example.com", "bf-key")
	BBolt_CreateUser(u)

	d := &types.Device{ID: uuid.New(), UserID: uuid.New(), Tag: "bf-dev"}
	BBolt_CreateDevice(d)

	cfg := &types.WGServerConfig{ID: uuid.New(), APIKey: "bf-wgkey"}
	BBolt_CreateWGServerConfig(cfg)

	n := &Network{ID: uuid.New(), CIDR: "172.16.0.0/24", Tag: "bf-net"}
	BBolt_CreateNetworksBatch([]*Network{n})

	BBoltDB.Close()

	// Reopen — backfill rebuilds all indexes.
	if err := ConnectToBBoltDB(dbPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { BBoltDB.Close() })

	if found, _ := BBolt_findUserByEmail("bf@example.com"); found == nil {
		t.Fatal("user email index not backfilled")
	}
	if found, _ := BBolt_findUserByAPIKey("bf-key"); found == nil {
		t.Fatal("user apikey index not backfilled")
	}
	if devs, _ := BBolt_GetDevicesByUserID(d.UserID); len(devs) != 1 {
		t.Fatal("device userid index not backfilled")
	}
	if found, _ := BBolt_FindWGServerConfigByAPIKey("bf-wgkey"); found == nil {
		t.Fatal("wg apikey index not backfilled")
	}
	if found, _ := BBolt_FindNetworkByID(n.ID); found == nil {
		t.Fatal("network id index not backfilled")
	}
}
