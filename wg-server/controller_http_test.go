package wgserver

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequireHTTPSControllerURL(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"https://ctrl.example:443", false},
		{"HTTPS://ctrl.example", false},
		{"http://ctrl.example", true},
		{"ftp://ctrl.example", true},
		{"", true},
		{"not a url", true},
		{"https://", true},
	}
	for _, tc := range cases {
		err := requireHTTPSControllerURL(tc.in)
		if tc.wantErr && err == nil {
			t.Errorf("requireHTTPSControllerURL(%q) = nil, want error", tc.in)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("requireHTTPSControllerURL(%q) = %v, want nil", tc.in, err)
		}
	}
}

func TestControllerHTTPClient_NoRedirects(t *testing.T) {
	var secondHits int
	second := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondHits++
		if r.Header.Get("X-WG-KEY") != "" {
			t.Error("X-WG-KEY should not reach redirect target")
		}
		_, _ = io.WriteString(w, "nope")
	}))
	defer second.Close()

	first := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, second.URL+"/steal", http.StatusTemporaryRedirect)
	}))
	defer first.Close()

	client := newControllerHTTPClient(true)
	req, err := http.NewRequest(http.MethodGet, first.URL+"/wg/peer", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-WG-KEY", "super-secret")

	resp, err := client.Do(req)
	if err == nil {
		if resp != nil {
			resp.Body.Close()
		}
		t.Fatal("expected error when controller redirects")
	}
	if !errors.Is(err, errControllerRedirect) && !strings.Contains(err.Error(), errControllerRedirect.Error()) {
		t.Fatalf("expected errControllerRedirect, got %v", err)
	}
	if secondHits != 0 {
		t.Fatalf("redirect target hit %d times", secondHits)
	}
}

func TestFetchConfig_RequiresHTTPS(t *testing.T) {
	_, err := FetchConfig("http://127.0.0.1:1", "key", "", false)
	if err == nil {
		t.Fatal("FetchConfig must reject http://")
	}
	if !strings.Contains(err.Error(), "https") {
		t.Fatalf("error should mention https, got %v", err)
	}
}
