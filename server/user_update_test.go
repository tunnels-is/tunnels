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
)

// The APIKey is now server-generated: any non-empty request yields a fresh
// server-minted UUID (the client can't set its own secret), and empty clears it.
func TestAPI_UserUpdate_APIKeyServerGenerated(t *testing.T) {
	setupTestDB(t)
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	u := &User{ID: uuid.New(), Email: "u@example.com", Groups: []uuid.UUID{}, Tokens: []*DeviceToken{}}
	if err := BBolt_CreateUser(u); err != nil {
		t.Fatal(err)
	}

	call := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/client/user/update", strings.NewReader(body))
		req = req.WithContext(context.WithValue(req.Context(), contextKeyUser, u))
		w := httptest.NewRecorder()
		API_UserUpdate(w, req)
		return w
	}

	// A client-supplied (even weak) value is ignored; the server returns a fresh
	// valid-UUID key that is NOT the submitted value.
	w := call(`{"APIKey":"not-a-uuid"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("non-empty APIKey request should regenerate (200), got %d: %s", w.Code, w.Body.String())
	}
	var resp struct{ APIKey string }
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.APIKey == "" || resp.APIKey == "not-a-uuid" {
		t.Fatalf("expected a fresh server-generated key, got %q", resp.APIKey)
	}
	if _, err := uuid.Parse(resp.APIKey); err != nil {
		t.Fatalf("server-generated APIKey is not a valid UUID: %q", resp.APIKey)
	}

	// Empty clears the key.
	if w := call(`{"APIKey":""}`); w.Code != http.StatusOK {
		t.Fatalf("empty APIKey should be allowed (clears the key), got %d", w.Code)
	}
}
