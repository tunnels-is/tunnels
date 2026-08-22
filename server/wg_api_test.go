package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

func setupWGTest(t *testing.T) (string, uuid.UUID) {
	t.Helper()
	setupTestDB(t)
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	apiKey := "test-wg-key"
	s := &types.Server{
		ID:     uuid.New(),
		Tag:    "wg-test",
		APIKey: apiKey,
	}
	if err := BBolt_CreateServer(s); err != nil {
		t.Fatal(err)
	}
	return apiKey, s.ID
}

func makeWGKey() string {
	return base64.StdEncoding.EncodeToString(make([]byte, 32))
}

func seedEnabledUser(t *testing.T) uuid.UUID {
	t.Helper()
	u := &User{
		ID:            uuid.New(),
		Email:         uuid.NewString() + "@test.local",
		SubExpiration: time.Now().Add(24 * time.Hour),
	}
	if err := BBolt_CreateUser(u); err != nil {
		t.Fatal(err)
	}
	return u.ID
}

var seedIP uint32 = 10

func seedDevice(t *testing.T, wgKey string, serverID uuid.UUID) *types.Device {
	t.Helper()
	n := atomic.AddUint32(&seedIP, 1)
	d := &types.Device{
		ID:           uuid.New(),
		UserID:       seedEnabledUser(t),
		ServerID:     serverID,
		WireGuardKey: wgKey,
		WireGuardIP:  fmt.Sprintf("10.0.0.%d", 10+n%200),
	}
	if err := BBolt_CreateDevice(d); err != nil {
		t.Fatal(err)
	}
	return d
}

func callWGPeers(t *testing.T, apiKey, query string) (*httptest.ResponseRecorder, types.WGPeersResponse) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/wg/peers"+query, nil)
	if apiKey != "" {
		req.Header.Set("X-WG-KEY", apiKey)
	}
	w := httptest.NewRecorder()

	wireGuardServerKeyCheck(http.HandlerFunc(API_WGPeers)).ServeHTTP(w, req)

	var resp types.WGPeersResponse
	if w.Code == http.StatusOK {
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
	}
	return w, resp
}

func uniqueWGKey(i int) string {
	raw := make([]byte, 32)
	raw[31] = byte(i)
	raw[30] = byte(i >> 8)
	return base64.StdEncoding.EncodeToString(raw)
}

func TestAPI_WGPeers_Unauthorized(t *testing.T) {
	setupWGTest(t)
	w, _ := callWGPeers(t, "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAPI_WGPeers_BadLimit(t *testing.T) {
	apiKey, _ := setupWGTest(t)
	for _, bad := range []string{"?limit=abc", "?limit=0", "?limit=-3"} {
		w, _ := callWGPeers(t, apiKey, bad)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", bad, w.Code)
		}
	}
}

func TestAPI_WGPeers_BadOffset(t *testing.T) {
	apiKey, _ := setupWGTest(t)
	for _, bad := range []string{"?offset=abc", "?offset=-1"} {
		w, _ := callWGPeers(t, apiKey, bad)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", bad, w.Code)
		}
	}
}

func TestAPI_WGPeers_EmptyDB(t *testing.T) {
	apiKey, _ := setupWGTest(t)
	w, resp := callWGPeers(t, apiKey, "?limit=10")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(resp.Peers) != 0 {
		t.Fatalf("expected 0 peers, got %d", len(resp.Peers))
	}
	if resp.Limit != 10 || resp.Offset != 0 {
		t.Fatalf("limit/offset echo wrong: %+v", resp)
	}
	if resp.NextOffset != -1 {
		t.Fatalf("empty result should signal done with NextOffset=-1, got %d", resp.NextOffset)
	}
}

func TestAPI_WGPeers_DefaultLimit(t *testing.T) {
	apiKey, _ := setupWGTest(t)
	w, resp := callWGPeers(t, apiKey, "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if resp.Limit != wgPeersDefaultLimit {
		t.Fatalf("expected default limit %d, got %d", wgPeersDefaultLimit, resp.Limit)
	}
}

func TestAPI_WGPeers_MaxLimitCap(t *testing.T) {
	apiKey, _ := setupWGTest(t)
	_, resp := callWGPeers(t, apiKey, "?limit=99999")
	if resp.Limit != wgPeersMaxLimit {
		t.Fatalf("expected cap %d, got %d", wgPeersMaxLimit, resp.Limit)
	}
}

func TestAPI_WGPeers_SinglePage(t *testing.T) {
	apiKey, srvID := setupWGTest(t)
	for i := 0; i < 5; i++ {
		seedDevice(t, uniqueWGKey(i), srvID)
	}

	_, resp := callWGPeers(t, apiKey, "?limit=10")
	if len(resp.Peers) != 5 {
		t.Fatalf("expected 5 peers, got %d", len(resp.Peers))
	}
	if resp.NextOffset != -1 {
		t.Fatalf("page fits within limit; NextOffset should be -1, got %d", resp.NextOffset)
	}
}

func TestAPI_WGPeers_Pagination(t *testing.T) {
	apiKey, srvID := setupWGTest(t)
	for i := 0; i < 7; i++ {
		seedDevice(t, uniqueWGKey(i), srvID)
	}

	seenIDs := make(map[string]int)
	offset := 0
	pages := 0
	for {
		pages++
		if pages > 10 {
			t.Fatal("pagination did not terminate")
		}
		query := "?limit=3"
		if offset > 0 {
			query += "&offset=" + itoa(offset)
		}
		w, resp := callWGPeers(t, apiKey, query)
		if w.Code != http.StatusOK {
			t.Fatalf("page %d: status %d", pages, w.Code)
		}
		if resp.Offset != offset {
			t.Fatalf("page %d: response Offset=%d, want %d", pages, resp.Offset, offset)
		}
		for _, p := range resp.Peers {
			if _, dup := seenIDs[p.DeviceID]; dup {
				t.Fatalf("device %s returned on multiple pages", p.DeviceID)
			}
			seenIDs[p.DeviceID] = pages
		}
		if resp.NextOffset == -1 {
			break
		}
		if resp.NextOffset <= offset {
			t.Fatalf("page %d: NextOffset %d did not advance from %d", pages, resp.NextOffset, offset)
		}
		offset = resp.NextOffset
	}

	if len(seenIDs) != 7 {
		t.Fatalf("expected to walk 7 unique devices, got %d", len(seenIDs))
	}
}

func TestAPI_WGPeers_SkipsEmptyAndInvalidKeys(t *testing.T) {
	apiKey, srvID := setupWGTest(t)

	good := seedDevice(t, uniqueWGKey(1), srvID)

	if err := BBolt_CreateDevice(&types.Device{ID: uuid.New(), UserID: seedEnabledUser(t), ServerID: srvID}); err != nil {
		t.Fatal(err)
	}

	if err := BBolt_CreateDevice(&types.Device{
		ID: uuid.New(), UserID: seedEnabledUser(t), ServerID: srvID,
		WireGuardKey: base64.StdEncoding.EncodeToString([]byte("too-short")),
	}); err != nil {
		t.Fatal(err)
	}

	_, resp := callWGPeers(t, apiKey, "?limit=10")
	if len(resp.Peers) != 1 {
		t.Fatalf("expected only the valid peer, got %d", len(resp.Peers))
	}
	if resp.Peers[0].DeviceID != good.ID.String() {
		t.Fatalf("wrong device returned: %s", resp.Peers[0].DeviceID)
	}

	if resp.NextOffset != -1 {
		t.Fatalf("expected NextOffset=-1 on terminal page, got %d", resp.NextOffset)
	}
}

func TestAPI_WGPeers_PartialPageEndsPagination(t *testing.T) {
	apiKey, srvID := setupWGTest(t)

	for i := 0; i < 3; i++ {
		seedDevice(t, uniqueWGKey(i), srvID)
	}

	_, page1 := callWGPeers(t, apiKey, "?limit=2")
	if len(page1.Peers) != 2 || page1.NextOffset != 2 {
		t.Fatalf("page1 wrong: peers=%d nextOffset=%d", len(page1.Peers), page1.NextOffset)
	}

	_, page2 := callWGPeers(t, apiKey, "?limit=2&offset=2")
	if len(page2.Peers) != 1 {
		t.Fatalf("page2 expected 1 peer, got %d", len(page2.Peers))
	}
	if page2.NextOffset != -1 {
		t.Fatalf("page2 should terminate; NextOffset=%d", page2.NextOffset)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func callWGConfigFetch(t *testing.T, apiKey, pubKey string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/wg/server-config/fetch", nil)
	if apiKey != "" {
		req.Header.Set("X-WG-KEY", apiKey)
	}
	if pubKey != "" {
		req.Header.Set("X-WG-PubKey", pubKey)
	}
	w := httptest.NewRecorder()
	wireGuardServerKeyCheck(http.HandlerFunc(API_WGServerConfigFetch)).ServeHTTP(w, req)
	return w
}

func TestAPI_WGServerConfigFetch_PinsThenRejectsReplacement(t *testing.T) {
	apiKey, id := setupWGTest(t)
	first := makeWGKey()
	second := uniqueWGKey(99)

	w := callWGConfigFetch(t, apiKey, first)
	if w.Code != http.StatusOK {
		t.Fatalf("first pin: %d %s", w.Code, w.Body.String())
	}
	got, err := BBolt_FindServerByID(id.String())
	if err != nil || got == nil || got.WireGuardPubKey != first {
		t.Fatalf("stored pubkey=%q want %q err=%v", got.WireGuardPubKey, first, err)
	}

	w = callWGConfigFetch(t, apiKey, first)
	if w.Code != http.StatusOK {
		t.Fatalf("same key restart: %d %s", w.Code, w.Body.String())
	}

	w = callWGConfigFetch(t, apiKey, second)
	if w.Code != http.StatusConflict {
		t.Fatalf("replacement: %d %s, want 409", w.Code, w.Body.String())
	}
	got, _ = BBolt_FindServerByID(id.String())
	if got.WireGuardPubKey != first {
		t.Fatalf("pin changed to %q", got.WireGuardPubKey)
	}
}

func TestAPI_WGServerConfigFetch_NewAPIKeyAllowsNewPubKey(t *testing.T) {
	apiKey, id := setupWGTest(t)
	first := makeWGKey()
	if w := callWGConfigFetch(t, apiKey, first); w.Code != http.StatusOK {
		t.Fatalf("pin: %d", w.Code)
	}

	s, _ := BBolt_FindServerByID(id.String())
	s.APIKey = "rotated-key"
	if _, err := BBolt_UpdateServer(s); err != nil {
		t.Fatal(err)
	}

	second := uniqueWGKey(7)
	w := callWGConfigFetch(t, "rotated-key", second)
	if w.Code != http.StatusOK {
		t.Fatalf("rebind after rotate: %d %s", w.Code, w.Body.String())
	}
	got, _ := BBolt_FindServerByID(id.String())
	if got.WireGuardPubKey != second {
		t.Fatalf("pubkey=%q want %q", got.WireGuardPubKey, second)
	}
}
