package punch

import (
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
)

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

func TestReflexAndRendezvousRoundTrip(t *testing.T) {
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
	req, err := BuildReflexReq(dev, cliPriv, srvPub, now)
	if err != nil {
		t.Fatal(err)
	}
	if !Is(req) || TypeOf(req) != TypeReflexReq {
		t.Fatalf("reflex type %d", TypeOf(req))
	}
	gotID, gotPub, err := OpenReflexReq(srvPriv, req, now)
	if err != nil {
		t.Fatal(err)
	}
	wantPub, _ := Public(cliPriv)
	if gotID != dev || gotPub != wantPub {
		t.Fatalf("reflex identity mismatch")
	}
	if _, _, err := OpenReflexReq(srvPriv, req, now.Add(5*time.Minute)); err == nil {
		t.Fatal("stale reflex must fail")
	}

	otherPriv, _ := NewPrivate()
	otherPub, _ := Public(otherPriv)
	var wgPub [32]byte
	wgPub[0] = 7
	msg := Rendezvous{
		Src:        dev,
		Dst:        uuid.New(),
		Epoch:      3,
		WGPub:      wgPub,
		WGIP:       netip.MustParseAddr("10.0.0.5"),
		Candidates: []netip.AddrPort{netip.MustParseAddrPort("203.0.113.8:40000")},
	}
	pkt, err := BuildRendezvous(cliPriv, otherPub, msg)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := OpenRendezvous(otherPriv, wantPub, pkt)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Epoch != 3 || opened.WGIP != msg.WGIP || len(opened.Candidates) != 1 {
		t.Fatalf("rendezvous %+v", opened)
	}
	if _, err := OpenRendezvous(otherPriv, srvPub, pkt); err == nil {
		t.Fatal("wrong sender key must fail")
	}
}

func TestEnginePunchesAndRefuses(t *testing.T) {
	aPriv, _ := NewPrivate()
	bPriv, _ := NewPrivate()
	aPub, _ := Public(aPriv)
	bPub, _ := Public(bPriv)
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
	a.SetPeers([]Peer{{DeviceID: bID, DiscoPub: bPub, WGPub: bWG, WGIP: bIP}})
	b.SetPeers([]Peer{{DeviceID: aID, DiscoPub: aPub, WGPub: aWG, WGIP: aIP}})

	now := time.Now()
	aReflex := netip.MustParseAddrPort("198.51.100.10:41000")
	bReflex := netip.MustParseAddrPort("198.51.100.20:42000")
	deliverReflex := func(eng *Engine, id uuid.UUID, priv [32]byte, ap netip.AddrPort, res Result) Result {
		for _, d := range res.Sends {
			if !d.Server || TypeOf(d.Data) != TypeReflexReq {
				continue
			}
			if _, _, err := OpenReflexReq(srvPriv, d.Data, now); err != nil {
				t.Fatal(err)
			}
			return eng.OnPacket(netip.AddrPort{}, BuildReflexResp(id, ap), now)
		}
		return Result{}
	}
	ar := a.Poll(now)
	br := b.Poll(now)
	deliverReflex(a, aID, aPriv, aReflex, ar)
	deliverReflex(b, bID, bPriv, bReflex, br)

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
	flip := map[string]string{"a": "b", "b": "a"}
	for steps := 0; steps < 64 && len(q) > 0; steps++ {
		item := q[0]
		q = q[1:]
		if item.d.Server && TypeOf(item.d.Data) != TypeRendezvous {
			continue
		}
		res := other[item.from].OnPacket(selfAddr[item.from], item.d.Data, now)
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
