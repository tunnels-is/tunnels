package punch

import (
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestPairAdmitted ...
func TestPairAdmitted(t *testing.T) {
	a := netip.MustParseAddr("10.0.0.5")
	b := netip.MustParseAddr("10.0.1.8")
	if !PairAdmitted(true, false, nil, a, true, false, nil, b) {
		t.Fatal("firewall off on both sides must admit")
	}
	if !PairAdmitted(false, true, nil, a, false, false, []string{a.String()}, b) {
		t.Fatal("allow-all plus a full-IP allow must admit")
	}
	if !PairAdmitted(false, false, []string{b.String()}, a, false, false, []string{a.String()}, b) {
		t.Fatal("mutual full-IP allows must admit")
	}
	if PairAdmitted(false, false, []string{b.String() + ":22"}, a, false, false, []string{a.String()}, b) {
		t.Fatal("a port-specific allow must not open a direct path")
	}
	if PairAdmitted(false, false, []string{"*:443"}, a, false, false, []string{a.String()}, b) {
		t.Fatal("*:port must not open a direct path")
	}
	if PairAdmitted(false, false, nil, a, false, false, []string{a.String()}, b) {
		t.Fatal("one side refusing must not admit")
	}
	if PairAdmitted(true, false, nil, a, false, false, []string{a.String() + ":9090"}, b) {
		t.Fatal("firewall off does not override the other side's port rule")
	}
}

// TestReflexRoundTrip ...
func TestReflexRoundTrip(t *testing.T) {
	srvPriv, err := NewPrivate()
	if err != nil {
		t.Fatal(err)
	}
	srvPub, err := Public(srvPriv)
	if err != nil {
		t.Fatal(err)
	}
	cliPriv, err := NewPrivate()
	if err != nil {
		t.Fatal(err)
	}
	dev := uuid.New()
	now := time.Now()
	req, pend, err := BuildReflexReq(dev, cliPriv, srvPub, now, 7)
	if err != nil {
		t.Fatal(err)
	}
	if !Is(req) || len(req) != MinPacket || LooksLikeWireGuard(req) {
		t.Fatalf("reflex request len %d", len(req))
	}
	got, err := OpenReflexReq(srvPriv, req, now)
	if err != nil {
		t.Fatal(err)
	}
	wantPub, _ := Public(cliPriv)
	if got.Device != dev || got.DiscoPub != wantPub || got.Nonce != 7 {
		t.Fatalf("reflex identity mismatch")
	}
	if _, err := OpenReflexReq(srvPriv, req, now.Add(5*time.Minute)); err == nil {
		t.Fatal("stale reflex must fail")
	}
	resp, err := BuildReflexResp(&got, netip.MustParseAddrPort("198.51.100.10:41000"))
	if err != nil {
		t.Fatal(err)
	}
	ap, err := OpenReflexResp(&pend, resp)
	if err != nil || ap.String() != "198.51.100.10:41000" || pend.Nonce() != 7 {
		t.Fatalf("reflex response %s %v", ap, err)
	}
	flipped := append([]byte(nil), resp...)
	flipped[reflexRespPrefix+1] ^= 0xff
	if _, err := OpenReflexResp(&pend, flipped); err == nil {
		t.Fatal("tampered reflex response opened")
	}
	resalted := append([]byte(nil), resp...)
	resalted[0] ^= 0x80
	if _, err := OpenReflexResp(&pend, resalted); err == nil {
		t.Fatal("re-salted reflex response opened")
	}
}

var testHub = netip.MustParseAddrPort("192.0.2.8:51820")

func deliverReflex(t *testing.T, eng *Engine, srvPriv [32]byte, id uuid.UUID, ap netip.AddrPort, res Result, now time.Time) {
	t.Helper()
	eng.SetServerEndpoint(testHub)
	for _, d := range res.Sends {
		if !d.Server || d.Type != TypeReflexReq {
			continue
		}
		req, err := OpenReflexReq(srvPriv, d.Data, now)
		if err != nil {
			t.Fatal(err)
		}
		if req.Device != id {
			t.Fatalf("reflex device %s, want %s", req.Device, id)
		}
		resp, err := BuildReflexResp(&req, ap)
		if err != nil {
			t.Fatal(err)
		}
		eng.OnPacket(testHub, resp, now)
		return
	}
	t.Fatal("no reflex request")
}

// TestEnginePunchesAndRefuses ...
func TestEnginePunchesAndRefuses(t *testing.T) {
	aPriv, _ := NewPrivate()
	bPriv, _ := NewPrivate()
	srvPriv, _ := NewPrivate()
	srvPub, _ := Public(srvPriv)
	var aWG, bWG [32]byte
	aWG[0] = 1
	bWG[0] = 2
	aID, bID := uuid.New(), uuid.New()
	aIP := netip.MustParseAddr("10.0.0.5")
	bIP := netip.MustParseAddr("10.0.1.8")
	a := NewEngine(aID, aPriv, aWG, aIP)
	b := NewEngine(bID, bPriv, bWG, bIP)
	a.SetServerDisco(srvPub)
	b.SetServerDisco(srvPub)
	a.SetLocalPolicy(false, false, []string{bIP.String()})
	b.SetLocalPolicy(false, false, []string{aIP.String()})

	now := time.Now()
	aReflex := netip.MustParseAddrPort("198.51.100.10:41000")
	bReflex := netip.MustParseAddrPort("198.51.100.20:42000")
	deliverReflex(t, a, srvPriv, aID, aReflex, a.Poll(now), now)
	deliverReflex(t, b, srvPriv, bID, bReflex, b.Poll(now), now)

	a.NoteTraffic(bIP)
	b.NoteTraffic(aIP)
	now = now.Add(time.Second)
	type tagged struct {
		from string
		d    Datagram
	}
	q := []tagged{}
	push := func(from string, res Result) {
		for _, d := range res.Sends {
			q = append(q, tagged{from, d})
		}
	}
	push("a", a.Poll(now))
	push("b", b.Poll(now))
	if !PairAdmitted(false, false, []string{bIP.String()}, aIP, false, false, []string{aIP.String()}, bIP) {
		t.Fatal("relay would have dropped a pair the firewall admits")
	}
	other := map[string]*Engine{"a": b, "b": a}
	selfAddr := map[string]netip.AddrPort{"a": aReflex, "b": bReflex}
	overlay := map[string]netip.Addr{"a": aIP, "b": bIP}
	flip := map[string]string{"a": "b", "b": "a"}
	for steps := 0; steps < 64 && len(q) > 0; steps++ {
		item := q[0]
		q = q[1:]
		if item.d.Server {
			continue
		}
		from := selfAddr[item.from]
		if item.d.Tunnel {
			from = netip.AddrPortFrom(overlay[item.from], IntroPort)
		}
		res := other[item.from].OnPacket(from, item.d.Data, now)
		push(flip[item.from], res)
	}
	if _, ok := a.Direct(bIP); !ok {
		t.Fatal("A did not promote a direct path")
	}
	if _, ok := b.Direct(aIP); !ok {
		t.Fatal("B did not promote a direct path")
	}

	revoked := a.SetLocalPolicy(false, false, nil)
	if len(revoked.Down) != 1 {
		t.Fatalf("revoke down=%v", revoked.Down)
	}
	if _, ok := a.Direct(bIP); ok {
		t.Fatal("direct path survived a policy revoke")
	}
}
