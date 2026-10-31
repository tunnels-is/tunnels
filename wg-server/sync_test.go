package wgserver

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tunnels-is/tunnels/types"
)

func startTLSController(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func testControllerCfg(srv *httptest.Server, apiKey string) *Config {
	return &Config{
		ControllerURL:      srv.URL,
		APIKey:             apiKey,
		InsecureSkipVerify: true,
	}
}

// TestQueryPeer_OK ...
func TestQueryPeer_OK(t *testing.T) {
	want := types.WGPeer{
		PublicKeyHex:  "deadbeef",
		DeviceID:      "dev-1",
		WireGuardIP:   "10.0.0.5",
		WireGuardIPv6: "fd00::5",
	}

	srv := startTLSController(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wg/peer" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("pubkey"); got != "abc==" {
			t.Errorf("unexpected pubkey: %q", got)
		}
		if got := r.Header.Get("X-WG-KEY"); got != "test-api-key" {
			t.Errorf("missing or wrong X-WG-KEY header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(want)
	})

	cfg := testControllerCfg(srv, "test-api-key")
	initSyncClient(cfg)

	res, got := queryPeer(cfg, "abc==")
	if res != authAllowed {
		t.Fatalf("expected authAllowed, got %v", res)
	}
	if got == nil || *got != want {
		t.Fatalf("mismatch: got %+v want %+v", got, want)
	}
}

// TestQueryPeer_Denied ...
func TestQueryPeer_Denied(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound} {
		srv := startTLSController(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		})
		cfg := testControllerCfg(srv, "k")
		initSyncClient(cfg)

		res, got := queryPeer(cfg, "x")
		if res != authUnknown {
			t.Fatalf("status %d without Decision: expected authUnknown, got %v", code, res)
		}
		if got != nil {
			t.Fatalf("status %d: expected nil peer", code)
		}
	}
}

// TestQueryPeer_ServerErrorIsUnknown ...
func TestQueryPeer_ServerErrorIsUnknown(t *testing.T) {
	srv := startTLSController(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	cfg := testControllerCfg(srv, "k")
	initSyncClient(cfg)

	if res, _ := queryPeer(cfg, "x"); res != authUnknown {
		t.Fatalf("5xx should be authUnknown, got %v", res)
	}
}

// TestQueryPeer_QueryEscaping ...
func TestQueryPeer_QueryEscaping(t *testing.T) {
	var seen string
	srv := startTLSController(t, func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.Query().Get("pubkey")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"Decision":"deny"}`))
	})

	cfg := testControllerCfg(srv, "k")
	initSyncClient(cfg)

	raw := "abc/def+ghi=="
	if res, _ := queryPeer(cfg, raw); res != authDenied {
		t.Fatalf("404 should be authDenied, got %v", res)
	}
	if seen != raw {
		t.Fatalf("query decode mismatch: got %q want %q", seen, raw)
	}
}

// TestQueryPeer_HTTPControllerURLRejected ...
func TestQueryPeer_HTTPControllerURLRejected(t *testing.T) {
	cfg := &Config{ControllerURL: "http://127.0.0.1:1", APIKey: "k"}
	initSyncClient(cfg)
	if res, _ := queryPeer(cfg, "x"); res != authUnknown {
		t.Fatalf("http:// controller must be rejected, got %v", res)
	}
}

// TestQueryPeer_RedirectNotFollowed ...
func TestQueryPeer_RedirectNotFollowed(t *testing.T) {
	var evilHits int
	evil := startTLSController(t, func(w http.ResponseWriter, r *http.Request) {
		evilHits++
		if r.Header.Get("X-WG-KEY") != "" {
			t.Error("X-WG-KEY must not be sent to redirect target")
		}
		w.WriteHeader(http.StatusOK)
	})

	good := startTLSController(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, evil.URL+"/wg/peer?pubkey=stolen", http.StatusFound)
	})

	cfg := testControllerCfg(good, "secret-key")
	initSyncClient(cfg)

	res, peer := queryPeer(cfg, "x")
	if res != authUnknown {
		t.Fatalf("redirect should fail closed as authUnknown, got %v peer=%v", res, peer)
	}
	if evilHits != 0 {
		t.Fatalf("redirect target was contacted %d times", evilHits)
	}
}

func TestQueryPeer_ExplicitDeny(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound} {
		srv := startTLSController(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(code)
			_, _ = w.Write([]byte(`{"Error":"no","Decision":"deny"}`))
		})
		cfg := testControllerCfg(srv, "k")
		initSyncClient(cfg)
		res, got := queryPeer(cfg, "x")
		if res != authDenied || got != nil {
			t.Fatalf("status %d Decision=deny: got %v peer=%v", code, res, got)
		}
	}
}

func TestQueryPeer_BudgetSkipsController(t *testing.T) {
	var hits atomic.Int32
	srv := startTLSController(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"Decision":"deny"}`))
	})
	cfg := testControllerCfg(srv, "k")
	initSyncClient(cfg)
	for i := 0; i < peerQueryBurst; i++ {
		if res, _ := queryPeer(cfg, "x"); res != authDenied {
			t.Fatalf("call %d: got %v", i, res)
		}
	}
	if res, _ := queryPeer(cfg, "x"); res != authUnknown {
		t.Fatalf("over budget: got %v", res)
	}
	if hits.Load() != int32(peerQueryBurst) {
		t.Fatalf("controller hits %d", hits.Load())
	}
}

func TestQueryPeer_TimesOut(t *testing.T) {
	srv := startTLSController(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
		w.WriteHeader(http.StatusOK)
	})
	cfg := testControllerCfg(srv, "k")
	initSyncClient(cfg)
	start := time.Now()
	res, peer := queryPeer(cfg, "x")
	elapsed := time.Since(start)
	if res != authUnknown || peer != nil {
		t.Fatalf("timed out query: got %v peer=%v", res, peer)
	}
	if elapsed < time.Second || elapsed > 4*time.Second {
		t.Fatalf("query took %s, want about %s", elapsed, peerQueryTimeout)
	}
}

func TestPeerQueryBudget_Refills(t *testing.T) {
	b := newTokenBucket(20, 20)
	now := time.Unix(1_800_000_000, 0)
	b.reset(now)
	for i := 0; i < 20; i++ {
		if !b.allow(now) {
			t.Fatalf("token %d", i)
		}
	}
	if b.allow(now) {
		t.Fatal("burst exceeded")
	}
	if b.allow(now.Add(500 * time.Millisecond)) {
		t.Fatal("partial second refilled")
	}
	if !b.allow(now.Add(time.Second)) {
		t.Fatal("full second did not refill")
	}
}

func TestCurrentPeerGen_SkipsStale(t *testing.T) {
	st := &peerState{gen: 5}
	called := false
	if got := withCurrentPeerGen(st, 4, func() authResult {
		called = true
		return authDenied
	}); got != authUnknown || called {
		t.Fatalf("stale gen applied: %v called=%v", got, called)
	}
	if got := withCurrentPeerGen(st, 5, func() authResult {
		called = true
		return authAllowed
	}); got != authAllowed || !called {
		t.Fatalf("current gen: %v called=%v", got, called)
	}
}

func TestPeerFlight_SharesOneCall(t *testing.T) {
	var g flightGroup
	var n atomic.Int32
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	fn := func() (authResult, *types.WGPeer) {
		n.Add(1)
		time.Sleep(30 * time.Millisecond)
		return authAllowed, &types.WGPeer{DeviceID: "d"}
	}
	var a, b authResult
	go func() {
		defer wg.Done()
		<-start
		a, _ = g.Do("k", fn)
	}()
	go func() {
		defer wg.Done()
		<-start
		b, _ = g.Do("k", fn)
	}()
	close(start)
	wg.Wait()
	if n.Load() != 1 || a != authAllowed || b != authAllowed {
		t.Fatalf("calls=%d a=%v b=%v", n.Load(), a, b)
	}
}

func TestReconcilePeer_ExplicitDenyRemovesInstalled(t *testing.T) {
	restorePeerGlobals(t)
	pub, hexKey := testPubKey(t)
	installedPeerAddrs.Store(hexKey, peerAddrs{ip: "10.0.0.5"})
	t.Cleanup(func() { installedPeerAddrs.Delete(hexKey) })

	srv := startTLSController(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"Decision":"deny"}`))
	})
	cfg := testControllerCfg(srv, "k")
	cfg.WireGuardSubnet = "10.0.0.0/24"
	activeConfig.Store(cfg)
	initSyncClient(cfg)
	peerStore = nil

	if got := reconcilePeer(pub); got != authDenied {
		t.Fatalf("got %v", got)
	}
	if _, ok := installedPeerAddrs.Load(hexKey); ok {
		t.Fatal("denied peer stayed installed")
	}
}

func TestReconcilePeer_BadAssignedIPKeepsInstalled(t *testing.T) {
	restorePeerGlobals(t)
	pub, hexKey := testPubKey(t)
	installedPeerAddrs.Store(hexKey, peerAddrs{ip: "10.0.0.5"})
	t.Cleanup(func() { installedPeerAddrs.Delete(hexKey) })

	srv := startTLSController(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(types.WGPeer{DeviceID: "dev", WireGuardIP: "10.9.9.9"})
	})
	cfg := testControllerCfg(srv, "k")
	cfg.WireGuardSubnet = "10.0.0.0/24"
	activeConfig.Store(cfg)
	initSyncClient(cfg)
	peerStore = nil

	if got := reconcilePeer(pub); got != authUnknown {
		t.Fatalf("bad assignment should not evict, got %v", got)
	}
	if _, ok := installedPeerAddrs.Load(hexKey); !ok {
		t.Fatal("peer was removed")
	}
}

func TestReconcilePeer_UnchangedIPSkipsIPC(t *testing.T) {
	restorePeerGlobals(t)
	pub, hexKey := testPubKey(t)
	installedPeerAddrs.Store(hexKey, peerAddrs{ip: "10.0.0.5"})
	t.Cleanup(func() { installedPeerAddrs.Delete(hexKey) })

	srv := startTLSController(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(types.WGPeer{DeviceID: "dev", WireGuardIP: "10.0.0.5"})
	})
	cfg := testControllerCfg(srv, "k")
	cfg.WireGuardSubnet = "10.0.0.0/24"
	activeConfig.Store(cfg)
	initSyncClient(cfg)
	peerStore = nil

	if got := reconcilePeer(pub); got != authAllowed {
		t.Fatalf("unchanged peer should stay without IPC, got %v", got)
	}
}

func TestReconcilePeer_StaleResultDoesNotApply(t *testing.T) {
	restorePeerGlobals(t)
	pub, hexKey := testPubKey(t)
	installedPeerAddrs.Store(hexKey, peerAddrs{ip: "10.0.0.5"})
	t.Cleanup(func() { installedPeerAddrs.Delete(hexKey) })

	release := make(chan struct{})
	joined := make(chan struct{}, 1)
	var calls atomic.Int32
	afterPeerFlightJoin = func() {
		select {
		case joined <- struct{}{}:
		default:
		}
	}
	srv := startTLSController(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			<-release
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"Decision":"deny"}`))
	})
	cfg := testControllerCfg(srv, "k")
	cfg.WireGuardSubnet = "10.0.0.0/24"
	activeConfig.Store(cfg)
	initSyncClient(cfg)
	peerStore = nil

	var wg sync.WaitGroup
	var older, newer authResult
	wg.Add(2)
	go func() {
		defer wg.Done()
		older = reconcilePeer(pub)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for calls.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("leader did not reach the controller")
		}
		time.Sleep(time.Millisecond)
	}
	go func() {
		defer wg.Done()
		newer = reconcilePeer(pub)
	}()
	select {
	case <-joined:
	case <-time.After(2 * time.Second):
		t.Fatal("second lookup did not join")
	}
	close(release)
	wg.Wait()

	if calls.Load() != 1 {
		t.Fatalf("controller calls %d", calls.Load())
	}
	if older != authUnknown {
		t.Fatalf("older result %v", older)
	}
	if newer != authDenied {
		t.Fatalf("newer result %v", newer)
	}
	if _, ok := installedPeerAddrs.Load(hexKey); ok {
		t.Fatal("current deny left the peer installed")
	}
}

func restorePeerGlobals(t *testing.T) {
	t.Helper()
	cfg := activeConfig.Load()
	store := peerStore
	join := afterPeerFlightJoin
	t.Cleanup(func() {
		activeConfig.Store(cfg)
		peerStore = store
		afterPeerFlightJoin = join
	})
	peerStore = nil
	afterPeerFlightJoin = nil
}

func testPubKey(t *testing.T) (string, string) {
	t.Helper()
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	pub := base64.StdEncoding.EncodeToString(raw)
	hexKey, err := b64ToHex(pub)
	if err != nil {
		t.Fatal(err)
	}
	return pub, hexKey
}
