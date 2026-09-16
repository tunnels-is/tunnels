package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
	gobolt "go.etcd.io/bbolt"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	err := openDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
}

func testUser(email, apiKey string) *User {
	return &User{
		ID:     uuid.New(),
		Email:  email,
		APIKey: apiKey,
	}
}

func TestOpenDB(t *testing.T) {
	setupTestDB(t)

	if err := createGroup(&Group{ID: uuid.New(), Tag: "smoke"}); err != nil {
		t.Fatal(err)
	}
}

func TestCreateUser(t *testing.T) {
	setupTestDB(t)
	u := testUser("test@example.com", "apikey123")
	if err := createUser(u); err != nil {
		t.Fatal(err)
	}

	found, err := findUserByID(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found.Email != "test@example.com" {
		t.Fatal("user not found by ID or email mismatch")
	}

	found, err = findUserByEmail("test@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found.ID != u.ID {
		t.Fatal("user not found by email index")
	}

	if err := db.View(func(tx *gobolt.Tx) error {
		uid := tx.Bucket([]byte(bucketUsersAPIKeyIndex)).Get([]byte("apikey123"))
		if uid == nil {
			t.Fatal("apikey index missing")
		}
		if string(uid) != u.ID.String() {
			t.Fatalf("apikey index uid = %s, want %s", uid, u.ID)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCreateUser_NoAPIKey(t *testing.T) {
	setupTestDB(t)
	u := testUser("nokey@example.com", "")
	if err := createUser(u); err != nil {
		t.Fatal(err)
	}

	if err := db.View(func(tx *gobolt.Tx) error {
		if v := tx.Bucket([]byte(bucketUsersAPIKeyIndex)).Get([]byte("")); v != nil {
			t.Fatal("empty apikey should not be indexed")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestFindUserByID_NotFound(t *testing.T) {
	setupTestDB(t)
	found, err := findUserByID(uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if found != nil {
		t.Fatal("expected nil")
	}
}

func TestFindUserByEmail_NotFound(t *testing.T) {
	setupTestDB(t)
	found, err := findUserByEmail("ghost@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if found != nil {
		t.Fatal("expected nil")
	}
}

func TestGetUsers(t *testing.T) {
	setupTestDB(t)
	for i := 0; i < 5; i++ {
		if err := createUser(testUser(fmt.Sprintf("u%d@example.com", i), "")); err != nil {
			t.Fatal(err)
		}
	}

	users, _ := getUsers(10, 0)
	if len(users) != 5 {
		t.Fatalf("expected 5, got %d", len(users))
	}

	users, _ = getUsers(3, 0)
	if len(users) != 3 {
		t.Fatalf("expected 3, got %d", len(users))
	}

	users, _ = getUsers(10, 3)
	if len(users) != 2 {
		t.Fatalf("expected 2, got %d", len(users))
	}

	users, _ = getUsers(10, 10)
	if len(users) != 0 {
		t.Fatalf("expected 0, got %d", len(users))
	}
}

func TestUpdateUserDeviceTokens(t *testing.T) {
	setupTestDB(t)
	u := testUser("tokens@example.com", "")
	createUser(u)

	tokens := []*DeviceToken{
		{DT: "t1", N: "d1", Created: time.Now()},
		{DT: "t2", N: "d2", Created: time.Now()},
	}
	if err := updateUserDeviceTokens(&userTokensUpdate{ID: u.ID, Tokens: tokens}); err != nil {
		t.Fatal(err)
	}

	found, _ := findUserByID(u.ID)
	if len(found.Tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(found.Tokens))
	}
}

func TestUpdateUserDeviceTokens_NotFound(t *testing.T) {
	setupTestDB(t)
	err := updateUserDeviceTokens(&userTokensUpdate{ID: uuid.New()})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateUserSubTime(t *testing.T) {
	setupTestDB(t)
	u := testUser("sub@example.com", "")
	createUser(u)

	exp := time.Now().Add(30 * 24 * time.Hour).Truncate(time.Second)
	if err := updateUserSubTime(&User{Email: "sub@example.com", SubExpiration: exp}); err != nil {
		t.Fatal(err)
	}

	found, _ := findUserByID(u.ID)
	if !found.SubExpiration.Truncate(time.Second).Equal(exp) {
		t.Fatalf("expected %v, got %v", exp, found.SubExpiration)
	}
}

func TestUpdateUserSubTime_NotFound(t *testing.T) {
	setupTestDB(t)
	err := updateUserSubTime(&User{Email: "nobody@example.com"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateUser(t *testing.T) {
	setupTestDB(t)
	u := testUser("update@example.com", "oldkey")
	createUser(u)

	if err := updateUser(&userUpdateRequest{
		UID:                   u.ID,
		APIKey:                "newkey",
		AdditionalInformation: "info",
	}); err != nil {
		t.Fatal(err)
	}

	if err := db.View(func(tx *gobolt.Tx) error {
		if tx.Bucket([]byte(bucketUsersAPIKeyIndex)).Get([]byte("oldkey")) != nil {
			t.Fatal("old key should not resolve")
		}
		if tx.Bucket([]byte(bucketUsersAPIKeyIndex)).Get([]byte("newkey")) == nil {
			t.Fatal("new key should resolve")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateUser_ClearAPIKey(t *testing.T) {
	setupTestDB(t)
	u := testUser("clear@example.com", "mykey")
	createUser(u)

	updateUser(&userUpdateRequest{UID: u.ID, APIKey: ""})

	if err := db.View(func(tx *gobolt.Tx) error {
		if tx.Bucket([]byte(bucketUsersAPIKeyIndex)).Get([]byte("mykey")) != nil {
			t.Fatal("cleared key should not resolve")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateUser_NotFound(t *testing.T) {
	setupTestDB(t)
	err := updateUser(&userUpdateRequest{UID: uuid.New()})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateUserAdmin(t *testing.T) {
	setupTestDB(t)
	u := testUser("admin@example.com", "")
	createUser(u)

	exp := time.Now().Add(60 * 24 * time.Hour).Truncate(time.Second)
	if err := updateUserAdmin(&adminUserUpdateRequest{
		TargetUserID:  u.ID,
		Email:         "new@example.com",
		Disabled:      true,
		Trial:         true,
		SubExpiration: exp,
	}); err != nil {
		t.Fatal(err)
	}

	found, _ := findUserByEmail("admin@example.com")
	if found != nil {
		t.Fatal("old email should not resolve")
	}

	found, _ = findUserByEmail("new@example.com")
	if found == nil {
		t.Fatal("new email should resolve")
	}
	if !found.Disabled || !found.Trial {
		t.Fatal("flags not set")
	}
	if !found.SubExpiration.Truncate(time.Second).Equal(exp) {
		t.Fatal("sub expiration mismatch")
	}
}

func TestUpdateUserAdmin_SameEmail(t *testing.T) {
	setupTestDB(t)
	u := testUser("same@example.com", "")
	createUser(u)

	updateUserAdmin(&adminUserUpdateRequest{
		TargetUserID: u.ID,
		Email:        "same@example.com",
		Disabled:     true,
	})

	found, _ := findUserByEmail("same@example.com")
	if found == nil {
		t.Fatal("email index should survive same-email update")
	}
}

func TestUpdateUserAdmin_EmptyEmail(t *testing.T) {
	setupTestDB(t)
	u := testUser("keep@example.com", "")
	createUser(u)

	updateUserAdmin(&adminUserUpdateRequest{
		TargetUserID: u.ID,
		Email:        "",
		Disabled:     true,
	})

	found, _ := findUserByEmail("keep@example.com")
	if found == nil {
		t.Fatal("original email should survive empty-email update")
	}
	if !found.Disabled {
		t.Fatal("Disabled not set")
	}
}

func TestUpdateUserAdmin_NotFound(t *testing.T) {
	setupTestDB(t)
	err := updateUserAdmin(&adminUserUpdateRequest{TargetUserID: uuid.New()})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateUserTwoFactorCodes(t *testing.T) {
	setupTestDB(t)
	u := testUser("2fa@example.com", "")
	createUser(u)

	if err := updateUserTwoFactorCodes(&twoFactorUpdate{
		UID:      u.ID,
		Code:     []byte("code"),
		Recovery: []byte("recovery"),
	}); err != nil {
		t.Fatal(err)
	}

	found, _ := findUserByID(u.ID)
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

func TestUpdateUserTwoFactorCodes_NotFound(t *testing.T) {
	setupTestDB(t)
	err := updateUserTwoFactorCodes(&twoFactorUpdate{UID: uuid.New()})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResetUserPassword(t *testing.T) {
	setupTestDB(t)
	u := testUser("reset@example.com", "")
	u.Password = "old"
	u.Tokens = []*DeviceToken{{DT: "t", N: "n"}}
	createUser(u)

	if err := resetUserPassword(&User{ID: u.ID, Password: "new"}); err != nil {
		t.Fatal(err)
	}

	found, _ := findUserByID(u.ID)
	if found.Password != "new" {
		t.Fatalf("expected 'new', got '%s'", found.Password)
	}
	if len(found.Tokens) != 0 {
		t.Fatalf("expected 0 tokens, got %d", len(found.Tokens))
	}
}

func TestResetUserPassword_NotFound(t *testing.T) {
	setupTestDB(t)
	err := resetUserPassword(&User{ID: uuid.New()})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestActivateUserKey(t *testing.T) {
	setupTestDB(t)
	u := testUser("activate@example.com", "")
	u.Disabled = true
	u.Trial = true
	createUser(u)

	exp := time.Now().Add(365 * 24 * time.Hour).Truncate(time.Second)
	key := &LicenseKey{Created: time.Now(), Months: 12, Key: "LIC-KEY"}
	if err := activateUserKey(exp, key, u.ID); err != nil {
		t.Fatal(err)
	}

	found, _ := findUserByID(u.ID)
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

func TestActivateUserKey_NotFound(t *testing.T) {
	setupTestDB(t)
	err := activateUserKey(time.Now(), nil, uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateDevice(t *testing.T) {
	setupTestDB(t)
	uid := uuid.New()
	d := &types.Device{ID: uuid.New(), Tag: "dev1", UserID: uid}
	if err := createDevice(d); err != nil {
		t.Fatal(err)
	}

	found, _ := findDeviceByID(d.ID)
	if found == nil || found.Tag != "dev1" {
		t.Fatal("device not found or tag mismatch")
	}

	devices, _ := getDevicesByUserID(uid)
	if len(devices) != 1 || devices[0].ID != d.ID {
		t.Fatal("device not found via user ID index")
	}
}

func TestFindDeviceByID_NotFound(t *testing.T) {
	setupTestDB(t)
	found, err := findDeviceByID(uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if found != nil {
		t.Fatal("expected nil")
	}
}

func TestGetDevicesByUserID_Multiple(t *testing.T) {
	setupTestDB(t)
	u1, u2 := uuid.New(), uuid.New()

	for i := 0; i < 3; i++ {
		createDevice(&types.Device{ID: uuid.New(), UserID: u1, Tag: fmt.Sprintf("u1-%d", i)})
	}
	for i := 0; i < 2; i++ {
		createDevice(&types.Device{ID: uuid.New(), UserID: u2, Tag: fmt.Sprintf("u2-%d", i)})
	}

	d1, _ := getDevicesByUserID(u1)
	if len(d1) != 3 {
		t.Fatalf("expected 3, got %d", len(d1))
	}

	d2, _ := getDevicesByUserID(u2)
	if len(d2) != 2 {
		t.Fatalf("expected 2, got %d", len(d2))
	}

	d3, _ := getDevicesByUserID(uuid.New())
	if len(d3) != 0 {
		t.Fatalf("expected 0, got %d", len(d3))
	}
}

func TestUpdateDevice(t *testing.T) {
	setupTestDB(t)
	uid := uuid.New()
	d := &types.Device{ID: uuid.New(), UserID: uid, Tag: "orig"}
	createDevice(d)

	d.Tag = "updated"
	if err := updateDevice(d); err != nil {
		t.Fatal(err)
	}

	found, _ := findDeviceByID(d.ID)
	if found.Tag != "updated" {
		t.Fatalf("expected 'updated', got '%s'", found.Tag)
	}

	devices, _ := getDevicesByUserID(uid)
	if len(devices) != 1 {
		t.Fatalf("expected 1, got %d", len(devices))
	}
}

func TestUpdateDevice_ChangeUserID(t *testing.T) {
	setupTestDB(t)
	u1, u2 := uuid.New(), uuid.New()
	d := &types.Device{ID: uuid.New(), UserID: u1, Tag: "move"}
	createDevice(d)

	d.UserID = u2
	updateDevice(d)

	d1, _ := getDevicesByUserID(u1)
	if len(d1) != 0 {
		t.Fatalf("expected 0 for old user, got %d", len(d1))
	}

	d2, _ := getDevicesByUserID(u2)
	if len(d2) != 1 {
		t.Fatalf("expected 1 for new user, got %d", len(d2))
	}
}

func TestDeleteDeviceByID(t *testing.T) {
	setupTestDB(t)
	uid := uuid.New()
	d := &types.Device{ID: uuid.New(), UserID: uid}
	createDevice(d)

	if err := deleteDeviceByID(d.ID); err != nil {
		t.Fatal(err)
	}

	found, _ := findDeviceByID(d.ID)
	if found != nil {
		t.Fatal("device should be deleted")
	}

	devices, _ := getDevicesByUserID(uid)
	if len(devices) != 0 {
		t.Fatalf("expected 0 after delete, got %d", len(devices))
	}
}

func TestDeleteDeviceByID_NonExistent(t *testing.T) {
	setupTestDB(t)
	if err := deleteDeviceByID(uuid.New()); err != nil {
		t.Fatal(err)
	}
}

func TestFindDeviceByWGKey(t *testing.T) {
	setupTestDB(t)
	d := &types.Device{ID: uuid.New(), UserID: uuid.New(), WireGuardKey: "wgkey-a"}
	if err := createDevice(d); err != nil {
		t.Fatal(err)
	}

	found, err := findDeviceByWGKey("wgkey-a")
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found.ID != d.ID {
		t.Fatal("expected device, got nil or wrong ID")
	}

	missing, err := findDeviceByWGKey("wgkey-does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	if missing != nil {
		t.Fatal("expected nil for missing key")
	}
}

func TestCreateDevice_DuplicateWGKey(t *testing.T) {
	setupTestDB(t)
	first := &types.Device{ID: uuid.New(), UserID: uuid.New(), WireGuardKey: "shared"}
	if err := createDevice(first); err != nil {
		t.Fatal(err)
	}

	second := &types.Device{ID: uuid.New(), UserID: uuid.New(), WireGuardKey: "shared"}
	if err := createDevice(second); err == nil {
		t.Fatal("expected uniqueness error, got nil")
	}

	found, _ := findDeviceByWGKey("shared")
	if found == nil || found.ID != first.ID {
		t.Fatal("first device should still own the wg key")
	}

	if got, _ := findDeviceByID(second.ID); got != nil {
		t.Fatal("second device should not have been written")
	}
}

func TestCreateDevice_EmptyWGKeyAllowsMany(t *testing.T) {
	setupTestDB(t)
	for i := 0; i < 3; i++ {
		if err := createDevice(&types.Device{ID: uuid.New(), UserID: uuid.New()}); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	if found, _ := findDeviceByWGKey(""); found != nil {
		t.Fatal("empty wg key should not be indexed")
	}
}

func TestUpdateDevice_ChangeWGKey(t *testing.T) {
	setupTestDB(t)
	d := &types.Device{ID: uuid.New(), UserID: uuid.New(), WireGuardKey: "old"}
	if err := createDevice(d); err != nil {
		t.Fatal(err)
	}

	d.WireGuardKey = "new"
	if err := updateDevice(d); err != nil {
		t.Fatal(err)
	}

	if old, _ := findDeviceByWGKey("old"); old != nil {
		t.Fatal("stale old-key index entry should be removed")
	}
	found, _ := findDeviceByWGKey("new")
	if found == nil || found.ID != d.ID {
		t.Fatal("new-key index entry missing")
	}
}

func TestUpdateDevice_DuplicateWGKey(t *testing.T) {
	setupTestDB(t)
	a := &types.Device{ID: uuid.New(), UserID: uuid.New(), WireGuardKey: "key-a"}
	b := &types.Device{ID: uuid.New(), UserID: uuid.New(), WireGuardKey: "key-b"}
	if err := createDevice(a); err != nil {
		t.Fatal(err)
	}
	if err := createDevice(b); err != nil {
		t.Fatal(err)
	}

	b.WireGuardKey = "key-a"
	if err := updateDevice(b); err == nil {
		t.Fatal("expected uniqueness error on update")
	}

	found, _ := findDeviceByWGKey("key-a")
	if found == nil || found.ID != a.ID {
		t.Fatal("key-a should still belong to device a")
	}
}

func TestDeleteDeviceByID_RemovesWGKeyIndex(t *testing.T) {
	setupTestDB(t)
	d := &types.Device{ID: uuid.New(), UserID: uuid.New(), WireGuardKey: "gone"}
	if err := createDevice(d); err != nil {
		t.Fatal(err)
	}

	if err := deleteDeviceByID(d.ID); err != nil {
		t.Fatal(err)
	}

	if found, _ := findDeviceByWGKey("gone"); found != nil {
		t.Fatal("wg key index entry should be removed on delete")
	}
}

func TestGetDevices(t *testing.T) {
	setupTestDB(t)
	for i := 0; i < 5; i++ {
		createDevice(&types.Device{ID: uuid.New(), UserID: uuid.New()})
	}

	dl, _ := getDevices(10, 0)
	if len(dl) != 5 {
		t.Fatalf("expected 5, got %d", len(dl))
	}

	dl, _ = getDevices(3, 0)
	if len(dl) != 3 {
		t.Fatalf("expected 3, got %d", len(dl))
	}

	dl, _ = getDevices(10, 3)
	if len(dl) != 2 {
		t.Fatalf("expected 2, got %d", len(dl))
	}
}

func TestGetDevices_PaginationWalksAllOnce(t *testing.T) {
	setupTestDB(t)
	const total = 13
	created := make(map[string]struct{}, total)
	for i := 0; i < total; i++ {
		d := &types.Device{ID: uuid.New(), UserID: uuid.New()}
		if err := createDevice(d); err != nil {
			t.Fatal(err)
		}
		created[d.ID.String()] = struct{}{}
	}

	const pageSize = int64(4)
	seen := make(map[string]int, total)
	var offset int64
	pages := 0
	for {
		pages++
		if pages > 10 {
			t.Fatal("pagination did not terminate")
		}
		page, err := getDevices(pageSize, offset)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range page {
			id := d.ID.String()
			if _, dup := seen[id]; dup {
				t.Fatalf("device %s returned on multiple pages", id)
			}
			seen[id] = pages
		}
		if int64(len(page)) < pageSize {
			break
		}
		offset += pageSize
	}

	if len(seen) != total {
		t.Fatalf("walk visited %d devices, expected %d", len(seen), total)
	}
	for id := range created {
		if _, ok := seen[id]; !ok {
			t.Fatalf("device %s was never returned by pagination", id)
		}
	}
}

func TestGetDevices_OffsetBeyondEnd(t *testing.T) {
	setupTestDB(t)
	for i := 0; i < 3; i++ {
		createDevice(&types.Device{ID: uuid.New(), UserID: uuid.New()})
	}

	dl, err := getDevices(10, 99)
	if err != nil {
		t.Fatal(err)
	}
	if len(dl) != 0 {
		t.Fatalf("offset past end should return empty, got %d", len(dl))
	}
}

func TestGetDevices_StableOrder(t *testing.T) {
	setupTestDB(t)
	for i := 0; i < 8; i++ {
		createDevice(&types.Device{ID: uuid.New(), UserID: uuid.New()})
	}

	first, _ := getDevices(8, 0)
	second, _ := getDevices(8, 0)
	if len(first) != len(second) {
		t.Fatalf("length mismatch: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i].ID != second[i].ID {
			t.Fatalf("order differs at index %d: %s vs %s", i, first[i].ID, second[i].ID)
		}
	}

	full, _ := getDevices(8, 0)
	var walked []*types.Device
	for off := int64(0); ; off += 3 {
		page, _ := getDevices(3, off)
		walked = append(walked, page...)
		if int64(len(page)) < 3 {
			break
		}
	}
	if len(walked) != len(full) {
		t.Fatalf("walked %d, full %d", len(walked), len(full))
	}
	for i := range full {
		if full[i].ID != walked[i].ID {
			t.Fatalf("paginated order differs from full read at %d", i)
		}
	}
}

func TestCreateServer(t *testing.T) {
	setupTestDB(t)
	s := &types.Server{ID: uuid.New(), Tag: "srv", Country: "US", IP: "1.2.3.4", Port: "443"}
	if err := createServer(s); err != nil {
		t.Fatal(err)
	}

	found, _ := findServerByID(s.ID)
	if found == nil || found.Tag != "srv" {
		t.Fatal("server not found or tag mismatch")
	}
}

func TestFindServerByID_NotFound(t *testing.T) {
	setupTestDB(t)
	found, err := findServerByID(uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if found != nil {
		t.Fatal("expected nil")
	}
}

func TestUpdateServer(t *testing.T) {
	setupTestDB(t)
	s := &types.Server{ID: uuid.New(), Tag: "orig", Country: "US", IP: "1.2.3.4", Port: "443"}
	createServer(s)

	updated, err := updateServer(&types.Server{
		ID:              s.ID,
		Tag:             "new",
		Country:         "UK",
		IP:              "5.6.7.8",
		Port:            "8443",
		WireGuardPort:   51820,
		WireGuardPubKey: "pubkey-abc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Tag != "new" || updated.Country != "UK" || updated.IP != "5.6.7.8" || updated.Port != "8443" {
		t.Fatal("basic field update mismatch")
	}
	if updated.WireGuardPort != 51820 {
		t.Fatalf("wireguard port mismatch: port=%d", updated.WireGuardPort)
	}
	if updated.WireGuardPubKey != "" {
		t.Fatal("pubkey must not be set via admin update; only config fetch pins it")
	}

	found, _ := findServerByID(s.ID)
	if found.WireGuardPort != 51820 {
		t.Fatal("wireguard port not persisted")
	}
}

func TestUpdateServer_NotFound(t *testing.T) {
	setupTestDB(t)
	_, err := updateServer(&types.Server{ID: uuid.New()})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFindAllServers(t *testing.T) {
	setupTestDB(t)
	for i := 0; i < 3; i++ {
		createServer(&types.Server{ID: uuid.New(), Tag: fmt.Sprintf("s%d", i)})
	}

	servers, err := findAllServers(100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 3 {
		t.Fatalf("expected 3, got %d", len(servers))
	}

	servers, _ = findAllServers(2, 0)
	if len(servers) != 2 {
		t.Fatalf("expected 2 with limit, got %d", len(servers))
	}

	servers, _ = findAllServers(10, 2)
	if len(servers) != 1 {
		t.Fatalf("expected 1 with offset, got %d", len(servers))
	}
}

func TestFindServersWithoutGroups(t *testing.T) {
	setupTestDB(t)
	for i := 0; i < 3; i++ {
		createServer(&types.Server{ID: uuid.New(), Tag: fmt.Sprintf("no-groups-%d", i)})
	}
	createServer(&types.Server{ID: uuid.New(), Tag: "has-groups", Groups: []uuid.UUID{uuid.New()}})

	servers, _ := findServersWithoutGroups(10, 0)
	if len(servers) != 3 {
		t.Fatalf("expected 3, got %d", len(servers))
	}

	servers, _ = findServersWithoutGroups(2, 0)
	if len(servers) != 2 {
		t.Fatalf("expected 2 with limit, got %d", len(servers))
	}

	servers, _ = findServersWithoutGroups(10, 2)
	if len(servers) != 1 {
		t.Fatalf("expected 1 with offset, got %d", len(servers))
	}
}

func TestFindServersByGroups(t *testing.T) {
	setupTestDB(t)
	g1, g2 := uuid.New(), uuid.New()
	createServer(&types.Server{ID: uuid.New(), Tag: "in-g1", Groups: []uuid.UUID{g1}})
	createServer(&types.Server{ID: uuid.New(), Tag: "in-g2", Groups: []uuid.UUID{g2}})
	createServer(&types.Server{ID: uuid.New(), Tag: "both", Groups: []uuid.UUID{g1, g2}})
	createServer(&types.Server{ID: uuid.New(), Tag: "none"})

	s1, _ := findServersByGroups([]uuid.UUID{g1}, 10, 0)
	if len(s1) != 2 {
		t.Fatalf("expected 2 in g1, got %d", len(s1))
	}

	s2, _ := findServersByGroups([]uuid.UUID{g2}, 10, 0)
	if len(s2) != 2 {
		t.Fatalf("expected 2 in g2, got %d", len(s2))
	}

	sAll, _ := findServersByGroups([]uuid.UUID{g1, g2}, 10, 0)
	if len(sAll) != 3 {
		t.Fatalf("expected 3 in g1|g2, got %d", len(sAll))
	}

	sLim, _ := findServersByGroups([]uuid.UUID{g1}, 1, 0)
	if len(sLim) != 1 {
		t.Fatalf("expected 1 with limit, got %d", len(sLim))
	}

	sOff, _ := findServersByGroups([]uuid.UUID{g1}, 10, 1)
	if len(sOff) != 1 {
		t.Fatalf("expected 1 with offset, got %d", len(sOff))
	}
}

func TestFindServerByAPIKey(t *testing.T) {
	setupTestDB(t)
	s := &types.Server{ID: uuid.New(), Tag: "wg", APIKey: "wg-key", WireGuardPort: 51820}
	if err := createServer(s); err != nil {
		t.Fatal(err)
	}

	found, err := findServerByAPIKey("wg-key")
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found.ID != s.ID {
		t.Fatal("server not found by apikey index")
	}
	if found.WireGuardPort != 51820 {
		t.Fatalf("expected port 51820, got %d", found.WireGuardPort)
	}
}

func TestFindServerByAPIKey_NotFound(t *testing.T) {
	setupTestDB(t)
	found, err := findServerByAPIKey("nope")
	if err != nil {
		t.Fatal(err)
	}
	if found != nil {
		t.Fatal("expected nil")
	}
}

func TestUpdateServer_RotateAPIKey(t *testing.T) {
	setupTestDB(t)
	s := &types.Server{ID: uuid.New(), APIKey: "old", WireGuardPubKey: "pinned-key"}
	if err := createServer(s); err != nil {
		t.Fatal(err)
	}

	s.APIKey = "fresh"
	updated, err := updateServer(s)
	if err != nil {
		t.Fatal(err)
	}
	if updated.WireGuardPubKey != "" {
		t.Fatal("rotating API key must unpin WireGuardPubKey")
	}

	if found, _ := findServerByAPIKey("old"); found != nil {
		t.Fatal("old key should not resolve")
	}
	if found, _ := findServerByAPIKey("fresh"); found == nil {
		t.Fatal("new key should resolve")
	}
}

func TestCreateGroup(t *testing.T) {
	setupTestDB(t)
	g := &Group{ID: uuid.New(), Tag: "grp", Description: "desc"}
	if err := createGroup(g); err != nil {
		t.Fatal(err)
	}

	found, _ := findGroupByID(g.ID)
	if found == nil || found.Tag != "grp" {
		t.Fatal("group not found or tag mismatch")
	}
}

func TestFindGroupByID_NotFound(t *testing.T) {
	setupTestDB(t)
	found, err := findGroupByID(uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if found != nil {
		t.Fatal("expected nil")
	}
}

func TestUpdateGroup(t *testing.T) {
	setupTestDB(t)
	g := &Group{ID: uuid.New(), Tag: "orig", Description: "old"}
	createGroup(g)

	if err := updateGroup(&Group{ID: g.ID, Tag: "new", Description: "fresh"}); err != nil {
		t.Fatal(err)
	}

	found, _ := findGroupByID(g.ID)
	if found.Tag != "new" || found.Description != "fresh" {
		t.Fatal("update mismatch")
	}
}

func TestUpdateGroup_NotFound(t *testing.T) {
	setupTestDB(t)
	err := updateGroup(&Group{ID: uuid.New()})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteGroupByID(t *testing.T) {
	setupTestDB(t)
	g := &Group{ID: uuid.New(), Tag: "del"}
	createGroup(g)

	if err := deleteGroupByID(g.ID); err != nil {
		t.Fatal(err)
	}

	found, _ := findGroupByID(g.ID)
	if found != nil {
		t.Fatal("group should be deleted")
	}
}

func TestListGroups(t *testing.T) {
	setupTestDB(t)
	for i := 0; i < 3; i++ {
		createGroup(&Group{ID: uuid.New(), Tag: fmt.Sprintf("g%d", i)})
	}

	groups, err := listGroups(100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 3 {
		t.Fatalf("expected 3, got %d", len(groups))
	}
}

func TestAddToGroup_User(t *testing.T) {
	setupTestDB(t)
	g := &Group{ID: uuid.New(), Tag: "ug"}
	createGroup(g)
	u := testUser("grp@example.com", "")
	createUser(u)

	if err := addToGroup(g.ID, u.ID, "user"); err != nil {
		t.Fatal(err)
	}

	found, _ := findUserByID(u.ID)
	if !slices.Contains(uuidSliceToString(found.Groups), g.ID.String()) {
		t.Fatal("user not in group")
	}

	addToGroup(g.ID, u.ID, "user")
	found, _ = findUserByID(u.ID)
	if len(found.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(found.Groups))
	}
}

func TestAddToGroup_Server(t *testing.T) {
	setupTestDB(t)
	g := &Group{ID: uuid.New(), Tag: "sg"}
	createGroup(g)
	s := &types.Server{ID: uuid.New(), Tag: "srv"}
	createServer(s)

	if err := addToGroup(g.ID, s.ID, "server"); err != nil {
		t.Fatal(err)
	}

	found, _ := findServerByID(s.ID)
	if !slices.Contains(uuidSliceToString(found.Groups), g.ID.String()) {
		t.Fatal("server not in group")
	}
}

func TestAddToGroup_Device_Unsupported(t *testing.T) {
	setupTestDB(t)
	g := &Group{ID: uuid.New(), Tag: "dg"}
	createGroup(g)
	d := &types.Device{ID: uuid.New(), UserID: uuid.New()}
	createDevice(d)

	if err := addToGroup(g.ID, d.ID, "device"); err == nil {
		t.Fatal("adding a device to a group should be unsupported")
	}
}

func TestAddToGroup_InvalidType(t *testing.T) {
	setupTestDB(t)
	err := addToGroup(uuid.New(), uuid.New(), "invalid")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAddToGroup_NotFound(t *testing.T) {
	setupTestDB(t)
	err := addToGroup(uuid.New(), uuid.New(), "user")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRemoveFromGroup_User(t *testing.T) {
	setupTestDB(t)
	g := &Group{ID: uuid.New(), Tag: "rg"}
	createGroup(g)
	u := testUser("rm@example.com", "")
	createUser(u)
	addToGroup(g.ID, u.ID, "user")

	if err := removeFromGroup(g.ID, u.ID, "user"); err != nil {
		t.Fatal(err)
	}

	found, _ := findUserByID(u.ID)
	if len(found.Groups) != 0 {
		t.Fatal("user should have 0 groups")
	}
}

func TestRemoveFromGroup_Server(t *testing.T) {
	setupTestDB(t)
	g := &Group{ID: uuid.New(), Tag: "rsg"}
	createGroup(g)
	s := &types.Server{ID: uuid.New(), Tag: "srv"}
	createServer(s)
	addToGroup(g.ID, s.ID, "server")

	removeFromGroup(g.ID, s.ID, "server")

	found, _ := findServerByID(s.ID)
	if len(found.Groups) != 0 {
		t.Fatal("server should have 0 groups")
	}
}

func TestRemoveFromGroup_InvalidType(t *testing.T) {
	setupTestDB(t)
	err := removeFromGroup(uuid.New(), uuid.New(), "invalid")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRemoveFromGroup_NotFound(t *testing.T) {
	setupTestDB(t)
	gid := uuid.New()
	for _, typ := range []string{"user", "server"} {
		err := removeFromGroup(gid, uuid.New(), typ)
		if err == nil {
			t.Fatalf("expected error for non-existent %s", typ)
		}
	}
}

func TestFindEntitiesByGroupID(t *testing.T) {
	setupTestDB(t)
	g := &Group{ID: uuid.New(), Tag: "find"}
	createGroup(g)

	u1 := testUser("e1@example.com", "")
	u2 := testUser("e2@example.com", "")
	createUser(u1)
	createUser(u2)
	addToGroup(g.ID, u1.ID, "user")
	addToGroup(g.ID, u2.ID, "user")

	s := &types.Server{ID: uuid.New(), Tag: "srv"}
	createServer(s)
	addToGroup(g.ID, s.ID, "server")

	entities, _ := findEntitiesByGroupID(g.ID, "user", 10, 0)
	if len(entities) != 2 {
		t.Fatalf("expected 2 users, got %d", len(entities))
	}

	entities, _ = findEntitiesByGroupID(g.ID, "server", 10, 0)
	if len(entities) != 1 {
		t.Fatalf("expected 1 server, got %d", len(entities))
	}

	if _, err := findEntitiesByGroupID(g.ID, "device", 10, 0); err == nil {
		t.Fatal("device entity lookup should be unsupported")
	}

	entities, _ = findEntitiesByGroupID(g.ID, "user", 1, 0)
	if len(entities) != 1 {
		t.Fatalf("expected 1 with limit, got %d", len(entities))
	}

	entities, _ = findEntitiesByGroupID(g.ID, "user", 10, 1)
	if len(entities) != 1 {
		t.Fatalf("expected 1 with offset, got %d", len(entities))
	}

	_, err := findEntitiesByGroupID(g.ID, "invalid", 10, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateServer_WithAPIKey(t *testing.T) {
	setupTestDB(t)
	s := &types.Server{
		ID:            uuid.New(),
		Tag:           "wg",
		APIKey:        "wg-key",
		WireGuardPort: 51820,
	}
	if err := createServer(s); err != nil {
		t.Fatal(err)
	}

	found, _ := findServerByID(s.ID)
	if found == nil || found.Tag != "wg" {
		t.Fatal("server not found or tag mismatch")
	}

	found, _ = findServerByAPIKey("wg-key")
	if found == nil || found.ID != s.ID {
		t.Fatal("server not found by apikey index")
	}
}

func TestCreateServer_NoAPIKey(t *testing.T) {
	setupTestDB(t)
	s := &types.Server{ID: uuid.New(), Tag: "no-key", WireGuardPort: 51820}
	if err := createServer(s); err != nil {
		t.Fatal(err)
	}

	found, _ := findServerByID(s.ID)
	if found == nil {
		t.Fatal("server should be findable by ID")
	}

	if found, _ := findServerByAPIKey(""); found != nil {
		t.Fatal("empty apikey should not match")
	}
}

func TestUpdateServer_ClearAPIKey(t *testing.T) {
	setupTestDB(t)
	s := &types.Server{ID: uuid.New(), APIKey: "clear"}
	if err := createServer(s); err != nil {
		t.Fatal(err)
	}

	s.APIKey = ""
	if _, err := updateServer(s); err != nil {
		t.Fatal(err)
	}

	if found, _ := findServerByAPIKey("clear"); found != nil {
		t.Fatal("cleared key should not resolve")
	}
}

func TestGetUsers_Empty(t *testing.T) {
	setupTestDB(t)
	users, err := getUsers(10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 0 {
		t.Fatalf("expected 0, got %d", len(users))
	}
}

func TestGetDevices_Empty(t *testing.T) {
	setupTestDB(t)
	dl, err := getDevices(10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(dl) != 0 {
		t.Fatalf("expected 0, got %d", len(dl))
	}
}

func TestFindAllServers_Empty(t *testing.T) {
	setupTestDB(t)
	servers, err := findAllServers(100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 0 {
		t.Fatalf("expected 0, got %d", len(servers))
	}
}

func TestListGroups_Empty(t *testing.T) {
	setupTestDB(t)
	groups, err := listGroups(100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 {
		t.Fatalf("expected 0, got %d", len(groups))
	}
}

func TestListGroups_LimitOffset(t *testing.T) {
	setupTestDB(t)
	for i := 0; i < 3; i++ {
		createGroup(&Group{ID: uuid.New(), Tag: fmt.Sprintf("p%d", i)})
	}

	groups, _ := listGroups(2, 0)
	if len(groups) != 2 {
		t.Fatalf("expected 2 with limit, got %d", len(groups))
	}

	groups, _ = listGroups(10, 2)
	if len(groups) != 1 {
		t.Fatalf("expected 1 with offset, got %d", len(groups))
	}
}

func TestOpenDB_BackfillIndexes(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	if err := openDB(dbPath); err != nil {
		t.Fatal(err)
	}

	u := testUser("bf@example.com", "bf-key")
	createUser(u)

	d := &types.Device{ID: uuid.New(), UserID: uuid.New(), Tag: "bf-dev"}
	createDevice(d)

	bfServer := &types.Server{ID: uuid.New(), Tag: "bf-srv", APIKey: "bf-wgkey"}
	createServer(bfServer)

	db.Close()

	if err := openDB(dbPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	if found, _ := findUserByEmail("bf@example.com"); found == nil {
		t.Fatal("user email index not backfilled")
	}
	if err := db.View(func(tx *gobolt.Tx) error {
		if tx.Bucket([]byte(bucketUsersAPIKeyIndex)).Get([]byte("bf-key")) == nil {
			t.Fatal("user apikey index not backfilled")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if devs, _ := getDevicesByUserID(d.UserID); len(devs) != 1 {
		t.Fatal("device userid index not backfilled")
	}
	if found, _ := findServerByAPIKey("bf-wgkey"); found == nil {
		t.Fatal("server apikey index not backfilled")
	}
}
