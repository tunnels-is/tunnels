package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

func setupWGTest(t *testing.T) string {
	t.Helper()
	setupTestDB(t)
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	apiKey := "test-wg-key"
	cfg := &types.WGServerConfig{
		ID:     uuid.New(),
		APIKey: apiKey,
	}
	if err := DB_CreateWGServerConfig(cfg); err != nil {
		t.Fatal(err)
	}
	return apiKey
}

func makeWGKey() string {
	return base64.StdEncoding.EncodeToString(make([]byte, 32))
}

func seedDevice(t *testing.T, wgKey string) *types.Device {
	t.Helper()
	d := &types.Device{
		ID:           uuid.New(),
		UserID:       uuid.New(),
		WireGuardKey: wgKey,
		WireGuardIP:  "10.0.0.5",
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
	API_WGPeers(w, req)

	var resp types.WGPeersResponse
	if w.Code == http.StatusOK {
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
	}
	return w, resp
}

// uniqueWGKey returns a 32-byte base64 key derived from i so it's stable and
// distinct across seeds within one test.
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
	apiKey := setupWGTest(t)
	for _, bad := range []string{"?limit=abc", "?limit=0", "?limit=-3"} {
		w, _ := callWGPeers(t, apiKey, bad)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", bad, w.Code)
		}
	}
}

func TestAPI_WGPeers_BadOffset(t *testing.T) {
	apiKey := setupWGTest(t)
	for _, bad := range []string{"?offset=abc", "?offset=-1"} {
		w, _ := callWGPeers(t, apiKey, bad)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", bad, w.Code)
		}
	}
}

func TestAPI_WGPeers_EmptyDB(t *testing.T) {
	apiKey := setupWGTest(t)
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
	apiKey := setupWGTest(t)
	w, resp := callWGPeers(t, apiKey, "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if resp.Limit != wgPeersDefaultLimit {
		t.Fatalf("expected default limit %d, got %d", wgPeersDefaultLimit, resp.Limit)
	}
}

func TestAPI_WGPeers_MaxLimitCap(t *testing.T) {
	apiKey := setupWGTest(t)
	_, resp := callWGPeers(t, apiKey, "?limit=99999")
	if resp.Limit != wgPeersMaxLimit {
		t.Fatalf("expected cap %d, got %d", wgPeersMaxLimit, resp.Limit)
	}
}

func TestAPI_WGPeers_SinglePage(t *testing.T) {
	apiKey := setupWGTest(t)
	for i := 0; i < 5; i++ {
		seedDevice(t, uniqueWGKey(i))
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
	apiKey := setupWGTest(t)
	for i := 0; i < 7; i++ {
		seedDevice(t, uniqueWGKey(i))
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
	apiKey := setupWGTest(t)

	good := seedDevice(t, uniqueWGKey(1))
	// Empty WireGuardKey: still in the devices bucket, but excluded from Peers.
	if err := BBolt_CreateDevice(&types.Device{ID: uuid.New(), UserID: uuid.New()}); err != nil {
		t.Fatal(err)
	}
	// Non-32-byte WireGuardKey: included by cursor, filtered by b64KeyToHex.
	if err := BBolt_CreateDevice(&types.Device{
		ID: uuid.New(), UserID: uuid.New(),
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
	// All three devices fit within limit=10, so this is the final page.
	if resp.NextOffset != -1 {
		t.Fatalf("expected NextOffset=-1 on terminal page, got %d", resp.NextOffset)
	}
}

func TestAPI_WGPeers_PartialPageEndsPagination(t *testing.T) {
	apiKey := setupWGTest(t)
	// 3 devices, limit=2: first page is full, second page has 1, then done.
	for i := 0; i < 3; i++ {
		seedDevice(t, uniqueWGKey(i))
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

// small local itoa to avoid pulling in strconv just for tests.
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
