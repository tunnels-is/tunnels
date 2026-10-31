package wgserver

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"net/http"
	"net/netip"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/curve25519"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/tuntest"
)

func TestTryDecryptInitiator_WireGuardPacket(t *testing.T) {
	serverPriv, serverPub, packet, clientPub := testInitiation(t)
	got, ok := tryDecryptInitiator(packet, serverPriv, serverPub)
	if !ok || got != clientPub {
		t.Fatalf("decrypt ok=%v got %q want %q", ok, got, clientPub)
	}

	packet[88] ^= 0xff
	if _, ok := tryDecryptInitiator(packet, serverPriv, serverPub); ok {
		t.Fatal("tampered timestamp was accepted")
	}
	packet[88] ^= 0xff

	packet[50] ^= 0xff
	if _, ok := tryDecryptInitiator(packet, serverPriv, serverPub); ok {
		t.Fatal("tampered static block was accepted")
	}
	packet[50] ^= 0xff

	other := make([]byte, 32)
	if _, err := rand.Read(other); err != nil {
		t.Fatal(err)
	}
	if _, ok := tryDecryptInitiator(packet, other, serverPub); ok {
		t.Fatal("wrong server key was accepted")
	}
}

func TestHandleInitiation_DropsBadTimestamp(t *testing.T) {
	serverPriv, serverPub, packet, _ := testInitiation(t)
	packet[90] ^= 0xff
	b := newLazyBind(nil, serverPriv, serverPub, 4, 100)
	b.handleInitiation(&bufferedPkt{data: packet}, true)
	select {
	case <-b.requeueCh:
		t.Fatal("invalid initiation was requeued")
	default:
	}
}

func TestHandleInitiation_RequeuesWhenUnknown(t *testing.T) {
	serverPriv, serverPub, packet, _ := testInitiation(t)
	old := activeConfig.Load()
	t.Cleanup(func() { activeConfig.Store(old) })
	activeConfig.Store(nil)

	b := newLazyBind(nil, serverPriv, serverPub, 4, 100)
	b.handleInitiation(&bufferedPkt{data: append([]byte(nil), packet...)}, true)
	select {
	case got := <-b.requeueCh:
		if !bytes.Equal(got.data, packet) {
			t.Fatal("requeued a different packet")
		}
	default:
		t.Fatal("valid initiation was dropped")
	}
}

func TestHandleInitiation_InstalledRequeuesWithoutLookup(t *testing.T) {
	serverPriv, serverPub, packet, clientPub := testInitiation(t)
	hexKey, err := b64ToHex(clientPub)
	if err != nil {
		t.Fatal(err)
	}
	installedPeerAddrs.Store(hexKey, peerAddrs{ip: "10.0.0.8"})
	t.Cleanup(func() { installedPeerAddrs.Delete(hexKey) })
	markPeerRefreshed(clientPub, time.Now())

	var hits int
	srv := startTLSController(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	})
	old := activeConfig.Load()
	store := peerStore
	t.Cleanup(func() {
		activeConfig.Store(old)
		peerStore = store
	})
	cfg := testControllerCfg(srv, "k")
	activeConfig.Store(cfg)
	initSyncClient(cfg)
	peerStore = nil

	b := newLazyBind(nil, serverPriv, serverPub, 4, 100)
	b.handleInitiation(&bufferedPkt{data: append([]byte(nil), packet...)}, false)
	select {
	case <-b.requeueCh:
	default:
		t.Fatal("installed peer was not requeued")
	}
	if hits != 0 {
		t.Fatalf("installed fast path called the controller %d times", hits)
	}
}

func TestHandleInitiation_UnknownUnderLoadDoesNotQuery(t *testing.T) {
	serverPriv, serverPub, packet, _ := testInitiation(t)
	var hits int
	srv := startTLSController(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	})
	old := activeConfig.Load()
	t.Cleanup(func() { activeConfig.Store(old) })
	activeConfig.Store(testControllerCfg(srv, "k"))
	initSyncClient(activeConfig.Load())

	b := newLazyBind(nil, serverPriv, serverPub, 4, 100)
	b.handleInitiation(&bufferedPkt{data: append([]byte(nil), packet...)}, false)
	select {
	case <-b.requeueCh:
		t.Fatal("unknown key was requeued without a lookup slot")
	default:
	}
	if hits != 0 {
		t.Fatalf("controller calls %d", hits)
	}
}

func TestHandshakeRate_PrefixWindow(t *testing.T) {
	b := newLazyBind(nil, nil, make([]byte, 32), 4, 100)
	now := time.Unix(1_800_000_000, 0)
	b.nowFn = func() time.Time { return now }

	a := netip.MustParseAddr("198.51.100.10")
	same := netip.MustParseAddr("198.51.100.200")
	mapped := netip.MustParseAddr("::ffff:198.51.100.10")
	other := netip.MustParseAddr("198.51.101.10")
	for i := 0; i < 99; i++ {
		if !b.handshakeRateAllowed(a) {
			t.Fatalf("allow %d", i)
		}
	}
	if !b.handshakeRateAllowed(mapped) {
		t.Fatal("mapped address did not share the v4 prefix budget")
	}
	if b.handshakeRateAllowed(same) {
		t.Fatal("same /24 exceeded the shared budget")
	}
	if !b.handshakeRateAllowed(other) {
		t.Fatal("different /24 was limited")
	}

	v6a := netip.MustParseAddr("2001:db8:1:2::1")
	v6b := netip.MustParseAddr("2001:db8:1:2::abcd")
	v6c := netip.MustParseAddr("2001:db8:1:3::1")
	for i := 0; i < 100; i++ {
		if !b.handshakeRateAllowed(v6a) {
			t.Fatalf("v6 allow %d", i)
		}
	}
	if b.handshakeRateAllowed(v6b) {
		t.Fatal("same /64 exceeded the shared budget")
	}
	if !b.handshakeRateAllowed(v6c) {
		t.Fatal("different /64 was limited")
	}

	now = now.Add(time.Second)
	if !b.handshakeRateAllowed(same) {
		t.Fatal("budget did not reset on the next second")
	}
	if b.handshakeRateAllowed(netip.Addr{}) {
		t.Fatal("invalid address was allowed")
	}
}

func TestHandshakeRate_CollisionFailsClosed(t *testing.T) {
	base := netip.MustParseAddr("10.1.0.1")
	baseKey, ok := ratePrefixKey(base)
	if !ok {
		t.Fatal("prefix")
	}
	want := rateSlotIndex(baseKey)
	var other netip.Addr
	found := false
	for a := 2; a < 255 && !found; a++ {
		for b := 0; b < 255; b++ {
			ip := netip.AddrFrom4([4]byte{10, byte(a), byte(b), 1})
			key, ok := ratePrefixKey(ip)
			if ok && rateSlotIndex(key) == want {
				other = ip
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("no colliding prefix")
	}

	b := newLazyBind(nil, nil, make([]byte, 32), 4, 100)
	now := time.Unix(1_800_000_000, 0)
	b.nowFn = func() time.Time { return now }
	if !b.handshakeRateAllowed(base) {
		t.Fatal("first prefix")
	}
	if b.handshakeRateAllowed(other) {
		t.Fatal("colliding prefix was allowed in the same second")
	}
	now = now.Add(time.Second)
	if !b.handshakeRateAllowed(other) {
		t.Fatal("colliding prefix was not reused after the slot expired")
	}
}

func TestHandshakeCookie_WireGuardAcceptsReply(t *testing.T) {
	serverPub := make([]byte, 32)
	if _, err := rand.Read(serverPub); err != nil {
		t.Fatal(err)
	}
	b := newLazyBind(&sendBind{}, nil, serverPub, 4, 100)
	pkt := make([]byte, msgInitiationSize)
	pkt[0] = msgInitiation
	pkt[4], pkt[5] = 0x11, 0x22
	if _, err := rand.Read(pkt[8:116]); err != nil {
		t.Fatal(err)
	}
	stampMAC1(pkt, serverPub)

	var pk device.NoisePublicKey
	copy(pk[:], serverPub)
	var gen device.CookieGenerator
	gen.Init(pk)
	gen.AddMacs(pkt)

	ep, err := conn.NewDefaultBind().ParseEndpoint("203.0.113.9:51820")
	if err != nil {
		t.Fatal(err)
	}
	reply, err := b.writeCookieReply(pkt, ep.DstToBytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(reply) != msgCookieReplySize || reply[0] != msgCookieReplyType || reply[4] != 0x11 || reply[5] != 0x22 {
		t.Fatalf("reply header %x", reply[:8])
	}
	var msg device.MessageCookieReply
	if err := binary.Read(bytes.NewReader(reply), binary.LittleEndian, &msg); err != nil {
		t.Fatal(err)
	}
	if !gen.ConsumeReply(&msg) {
		t.Fatal("wireguard rejected the cookie reply")
	}
	next := append([]byte(nil), pkt...)
	gen.AddMacs(next)
	if !b.validMAC2(next, ep.DstToBytes()) {
		t.Fatal("mac2 was rejected")
	}
	other, err := conn.NewDefaultBind().ParseEndpoint("203.0.113.10:51820")
	if err != nil {
		t.Fatal(err)
	}
	if b.validMAC2(next, other.DstToBytes()) {
		t.Fatal("mac2 was accepted for a different source")
	}
	next[140] ^= 0xff
	if b.validMAC2(next, ep.DstToBytes()) {
		t.Fatal("tampered mac2 was accepted")
	}
}

func TestHandshakeUnderLoad_SendsCookie(t *testing.T) {
	serverPub := make([]byte, 32)
	if _, err := rand.Read(serverPub); err != nil {
		t.Fatal(err)
	}
	sb := &sendBind{}
	b := newLazyBind(sb, make([]byte, 32), serverPub, 4, 100)
	fillHandshakeSlots(b)
	pkt := stampedInit(t, serverPub)
	ep := mustEndpoint(t, "203.0.113.9:40000")
	b.takeHandshake(pkt, ep)
	b.handshakeWG.Wait()
	sb.mu.Lock()
	defer sb.mu.Unlock()
	if len(sb.sent) != 1 || len(sb.sent[0]) != msgCookieReplySize || sb.sent[0][0] != msgCookieReplyType {
		t.Fatalf("sent %d cookies", len(sb.sent))
	}
}

func TestHandshakeUnderLoad_ValidMAC2SkipsCookie(t *testing.T) {
	serverPub := make([]byte, 32)
	if _, err := rand.Read(serverPub); err != nil {
		t.Fatal(err)
	}
	sb := &sendBind{}
	b := newLazyBind(sb, make([]byte, 32), serverPub, 4, 100)
	pkt := stampedInit(t, serverPub)
	ep := mustEndpoint(t, "203.0.113.9:40000")

	var pk device.NoisePublicKey
	copy(pk[:], serverPub)
	var gen device.CookieGenerator
	gen.Init(pk)
	gen.AddMacs(pkt)
	reply, err := b.writeCookieReply(pkt, ep.DstToBytes())
	if err != nil {
		t.Fatal(err)
	}
	var msg device.MessageCookieReply
	if err := binary.Read(bytes.NewReader(reply), binary.LittleEndian, &msg); err != nil {
		t.Fatal(err)
	}
	if !gen.ConsumeReply(&msg) {
		t.Fatal("consume")
	}
	gen.AddMacs(pkt)

	fillHandshakeSlots(b)
	b.takeHandshake(pkt, ep)
	b.handshakeWG.Wait()
	sb.mu.Lock()
	defer sb.mu.Unlock()
	if len(sb.sent) != 0 {
		t.Fatal("cookie sent for a packet that already had mac2")
	}
}

func TestHandshake_NotUnderLoadSkipsCookie(t *testing.T) {
	serverPub := make([]byte, 32)
	if _, err := rand.Read(serverPub); err != nil {
		t.Fatal(err)
	}
	sb := &sendBind{}
	b := newLazyBind(sb, make([]byte, 32), serverPub, 4, 100)
	b.takeHandshake(stampedInit(t, serverPub), mustEndpoint(t, "203.0.113.9:40000"))
	b.handshakeWG.Wait()
	sb.mu.Lock()
	defer sb.mu.Unlock()
	if len(sb.sent) != 0 {
		t.Fatal("cookie sent while a handshake slot was free")
	}
}

func testInitiation(t *testing.T) (serverPriv, serverPub, packet []byte, clientPub string) {
	t.Helper()
	serverPriv = make([]byte, 32)
	clientPriv := make([]byte, 32)
	if _, err := rand.Read(serverPriv); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(clientPriv); err != nil {
		t.Fatal(err)
	}
	var err error
	serverPub, err = curve25519.X25519(serverPriv, curve25519.Basepoint)
	if err != nil {
		t.Fatal(err)
	}
	clientPubBytes, err := curve25519.X25519(clientPriv, curve25519.Basepoint)
	if err != nil {
		t.Fatal(err)
	}
	clientPub = base64.StdEncoding.EncodeToString(clientPubBytes)

	dev := newNoiseDevice(t, clientPriv)
	var remote device.NoisePublicKey
	copy(remote[:], serverPub)
	peer, err := dev.NewPeer(remote)
	if err != nil {
		t.Fatal(err)
	}
	peer.Start()
	msg, err := dev.CreateMessageInitiation(peer)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, msg); err != nil {
		t.Fatal(err)
	}
	packet = buf.Bytes()
	if len(packet) != msgInitiationSize {
		t.Fatalf("initiation is %d bytes", len(packet))
	}
	return serverPriv, serverPub, packet, clientPub
}

func newNoiseDevice(t *testing.T, priv []byte) *device.Device {
	t.Helper()
	var sk device.NoisePrivateKey
	if err := sk.FromHex(hex.EncodeToString(priv)); err != nil {
		t.Fatal(err)
	}
	dev := device.NewDevice(tuntest.NewChannelTUN().TUN(), conn.NewDefaultBind(), device.NewLogger(device.LogLevelSilent, ""))
	t.Cleanup(func() { dev.Close() })
	if err := dev.SetPrivateKey(sk); err != nil {
		t.Fatal(err)
	}
	return dev
}

func stampMAC1(pkt, serverPub []byte) {
	key := noiseHash([]byte(labelMAC1), serverPub)
	mac, err := blake2s.New128(key)
	if err != nil {
		panic(err)
	}
	mac.Write(pkt[:msgInitiationSize-2*blake2s.Size128])
	mac.Sum(pkt[msgInitiationSize-2*blake2s.Size128 : msgInitiationSize-2*blake2s.Size128])
}

func stampedInit(t *testing.T, serverPub []byte) []byte {
	t.Helper()
	pkt := make([]byte, msgInitiationSize)
	pkt[0] = msgInitiation
	if _, err := rand.Read(pkt[8:116]); err != nil {
		t.Fatal(err)
	}
	stampMAC1(pkt, serverPub)
	return pkt
}

func mustEndpoint(t *testing.T, addr string) conn.Endpoint {
	t.Helper()
	ep, err := conn.NewDefaultBind().ParseEndpoint(addr)
	if err != nil {
		t.Fatal(err)
	}
	return ep
}

func fillHandshakeSlots(b *LazyBind) {
	for i := 0; i < cap(b.handshakeSem); i++ {
		b.handshakeSem <- struct{}{}
	}
}

type sendBind struct {
	mu   sync.Mutex
	sent [][]byte
}

func (s *sendBind) Open(uint16) ([]conn.ReceiveFunc, uint16, error) { return nil, 0, nil }
func (s *sendBind) Close() error                                    { return nil }
func (s *sendBind) SetMark(uint32) error                            { return nil }
func (s *sendBind) Send(bufs [][]byte, _ conn.Endpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, buf := range bufs {
		s.sent = append(s.sent, append([]byte(nil), buf...))
	}
	return nil
}
func (s *sendBind) ParseEndpoint(string) (conn.Endpoint, error) {
	return nil, nil
}
func (s *sendBind) BatchSize() int { return 1 }
