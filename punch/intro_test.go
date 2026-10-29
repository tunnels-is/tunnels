package punch

import (
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestIntroExchangeReachesDirectPath(t *testing.T) {
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
	giveReflex := func(eng *Engine, id uuid.UUID, res Result, ap netip.AddrPort) {
		t.Helper()
		for _, d := range res.Sends {
			if !d.Server || TypeOf(d.Data) != TypeReflexReq {
				continue
			}
			if _, _, err := OpenReflexReq(srvPriv, d.Data, now); err != nil {
				t.Fatal(err)
			}
			eng.OnPacket(netip.AddrPort{}, BuildReflexResp(id, ap), now)
		}
	}
	giveReflex(a, aID, a.Poll(now), aReflex)
	giveReflex(b, bID, b.Poll(now), bReflex)

	a.NoteTraffic(bIP)
	now = now.Add(time.Second)
	sent := a.Poll(now)
	var intro Datagram
	for _, d := range sent.Sends {
		if d.Tunnel && TypeOf(d.Data) == TypeIntro {
			intro = d
		}
		if d.Server && TypeOf(d.Data) == TypeRendezvous {
			t.Fatal("introduction must not ask the server to relay a rendezvous")
		}
	}
	if intro.Data == nil || intro.To.Addr() != bIP || intro.To.Port() != IntroPort {
		t.Fatalf("intro %+v", intro.To)
	}
	parsed, err := ParseIntro(intro.Data)
	if err != nil || parsed.Src != aID || parsed.WGIP != aIP || parsed.WGPub != aWG {
		t.Fatalf("parsed %+v %v", parsed, err)
	}

	reply := b.OnPacket(netip.AddrPortFrom(aIP, IntroPort), intro.Data, now)
	var back Datagram
	var bPing bool
	for _, d := range reply.Sends {
		if d.Tunnel && TypeOf(d.Data) == TypeIntro {
			back = d
		}
		if !d.Tunnel && !d.Server && d.To == aReflex && TypeOf(d.Data) == TypePing {
			bPing = true
		}
	}
	if back.Data == nil || back.To.Addr() != aIP {
		t.Fatal("peer did not answer through the tunnel")
	}
	if !bPing {
		t.Fatal("peer did not punch the advertised address")
	}

	learned := a.OnPacket(netip.AddrPortFrom(bIP, IntroPort), back.Data, now)
	var aPing Datagram
	for _, d := range learned.Sends {
		if d.To == bReflex && TypeOf(d.Data) == TypePing {
			aPing = d
		}
	}
	if aPing.Data == nil {
		t.Fatal("origin did not punch the peer address")
	}

	// Two pongs each way promote the direct path.
	fromB := b.OnPacket(aReflex, aPing.Data, now)
	var pong Datagram
	for _, d := range fromB.Sends {
		if TypeOf(d.Data) == TypePong {
			pong = d
		}
	}
	if pong.Data == nil {
		t.Fatal("no pong")
	}
	a.OnPacket(bReflex, pong.Data, now)
	second := a.Poll(now.Add(Keepalive))
	var ping2 Datagram
	for _, d := range second.Sends {
		if d.To == bReflex && TypeOf(d.Data) == TypePing {
			ping2 = d
		}
	}
	if ping2.Data == nil {
		t.Fatal("no second ping")
	}
	again := b.OnPacket(aReflex, ping2.Data, now.Add(Keepalive))
	var pong2 Datagram
	for _, d := range again.Sends {
		if TypeOf(d.Data) == TypePong {
			pong2 = d
		}
	}
	up := a.OnPacket(bReflex, pong2.Data, now.Add(Keepalive))
	if len(up.Up) != 1 || up.Up[0].Via != bReflex || up.Up[0].Peer.DeviceID != bID {
		t.Fatalf("direct path %+v", up.Up)
	}
}

func TestIntroSkipsPublicDestinations(t *testing.T) {
	a := NewEngine(uuid.New(), mustKey(t), [32]byte{1}, netip.MustParseAddr("10.0.0.5"))
	a.SetLocalPolicy(true, false, nil)
	pub, err := Public(mustKey(t))
	if err != nil {
		t.Fatal(err)
	}
	public := netip.MustParseAddr("203.0.113.9")
	a.NoteTraffic(public)
	res := a.Poll(time.Now())
	for _, d := range res.Sends {
		if d.Tunnel {
			t.Fatal("public destination was introduced")
		}
	}
	pkt, err := BuildIntro(uuid.New(), pub, [32]byte{2}, public, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := a.OnPacket(netip.AddrPort{}, pkt, time.Now()); len(got.Sends) != 0 || len(got.Up) != 0 {
		t.Fatalf("public intro was accepted: %+v", got)
	}
}

func mustKey(t *testing.T) [32]byte {
	t.Helper()
	k, err := NewPrivate()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestIntroRejectedByLocalFirewall(t *testing.T) {
	aPriv, _ := NewPrivate()
	bPriv, _ := NewPrivate()
	aPub, _ := Public(aPriv)
	var aWG, bWG [32]byte
	aWG[0] = 1
	bWG[0] = 2
	aIP := netip.MustParseAddr("10.0.0.5")
	bIP := netip.MustParseAddr("10.0.1.8")
	b := NewEngine(uuid.New(), bPriv, bWG, bIP)
	b.SetLocalPolicy(false, false, nil)
	pkt, err := BuildIntro(uuid.New(), aPub, aWG, aIP, nil)
	if err != nil {
		t.Fatal(err)
	}
	res := b.OnPacket(netip.AddrPortFrom(aIP, IntroPort), pkt, time.Now())
	if len(res.Sends) != 0 || len(res.Up) != 0 {
		t.Fatalf("firewall should drop the intro: %+v", res)
	}
}
