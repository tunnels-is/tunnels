package punch

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/curve25519"
)

// Reflex v3 (hardened) is a Noise-X-like request to the hub disco key and
// an ephemeral-keyed response, with no clear header on either:
//
// request (MinPacket bytes):
//
//	salt(16) | chk(8) | e^mask(32) | sBox(48) | pBox
//	chk   = BLAKE2s(Kh, "req" | salt)[:8], Kh = KDF(hub disco pub): a cheap
//	        pre-filter only holders of the hub pub can produce
//	mask  = BLAKE2s(Kh, "eph" | salt)
//	sBox  = AEAD_k1{client disco pub}, k1 = KDF(DH(e, hub))
//	pBox  = AEAD_k2{ver type device ts nonce | pad}, k2 = KDF(es | ss | client)
//
// response (MinPacket bytes):
//
//	salt(16) | chk(16) | h^mask(32) | AEAD_kr{ver type nonce addr | pad}
//	kb    = KDF(es | ss | client), bound to e
//	chk   = BLAKE2s(kb, "resp" | salt)[:16], mask = BLAKE2s(kb, "hp" | salt)
//	kr    = KDF(kb | DH(h, e)), h a fresh hub ephemeral
//
// The response is forward secret: once the hub drops h and the client drops
// e, the two disco privates alone no longer open it. The request cannot be
// forward secret against a later hub key compromise without a round trip;
// its contents (device id, client disco pub, time) are what the hub needs
// to answer, and the hub is the only party that can ever read them.
//
// Every key is fresh per request, so the boxes use the all-zero nonce.

const (
	labelReflexS    = "tunnels.is punch v3 reflex s"
	labelReflexReq  = "tunnels.is punch v3 reflex req"
	labelReflexBase = "tunnels.is punch v3 reflex base"
	labelReflexResp = "tunnels.is punch v3 reflex resp"
	labelHubDemux   = "tunnels.is punch v3 hub demux"

	reqChkLen        = 8
	respChkLen       = 16
	reflexReqPrefix  = saltLen + reqChkLen + keyLen
	reflexSBoxLen    = keyLen + 16
	reflexReqPlain   = 2 + idLen + 8 + 8
	reflexReqLen     = MinPacket
	reflexRespPrefix = saltLen + respChkLen + keyLen
	reflexRespPlain  = 2 + 8 + addrLen + 2
	reflexRespLen    = MinPacket

	// replayCacheMax bounds the hub reflex replay cache.
	replayCacheMax = 8192
)

func hubDemuxKey(hubPub [32]byte) [32]byte {
	return kdf(hubPub[:], nil, nil, labelHubDemux)
}

func reqChk(hd [32]byte, salt []byte) []byte {
	sum := prf(hd, []byte("req"), salt)
	return sum[:reqChkLen]
}

func respChk(base [32]byte, salt []byte) []byte {
	sum := prf(base, []byte("resp"), salt)
	return sum[:respChkLen]
}

// HubKey is the hub's disco key with its derived demux key cached, so junk
// on the WG port costs one BLAKE2s before it is dropped.
type HubKey struct {
	priv  [32]byte
	pub   [32]byte
	demux [32]byte
}

// NewHubKey ...
func NewHubKey(priv [32]byte) (*HubKey, error) {
	pub, err := Public(priv)
	if err != nil {
		return nil, err
	}
	return &HubKey{priv: priv, pub: pub, demux: hubDemuxKey(pub)}, nil
}

// Wipe ...
func (h *HubKey) Wipe() {
	wipe32(&h.priv)
	wipe32(&h.demux)
}

// ReflexPending is what a client keeps between a reflex request and its
// response: the request ephemeral (needed for the forward-secret response
// key) and the response base key. Wipe it when the response arrives or a
// new request replaces it.
type ReflexPending struct {
	ephPriv [32]byte
	ephPub  [32]byte
	base    [32]byte
	nonce   uint64
	live    bool
}

// Nonce ...
func (p *ReflexPending) Nonce() uint64 { return p.nonce }

// Wipe ...
func (p *ReflexPending) Wipe() {
	wipe32(&p.ephPriv)
	wipe32(&p.base)
	p.live = false
}

// ReflexRequest is what the hub learns from an opened reflex request.
type ReflexRequest struct {
	Device   uuid.UUID
	DiscoPub [32]byte
	Nonce    uint64
	ephPub   [32]byte
	base     [32]byte
	expires  time.Time
}

// Wipe ...
func (r *ReflexRequest) Wipe() {
	wipe32(&r.base)
}

// ReplayID identifies the request for replay detection (its ephemeral).
func (r *ReflexRequest) ReplayID() [32]byte { return r.ephPub }

// Expires is when the request's timestamp leaves the skew window.
func (r *ReflexRequest) Expires() time.Time { return r.expires }

func reflexKeys(es, ss []byte, ephPub, hubPub, clientPub [32]byte) (req, base [32]byte) {
	ikm := make([]byte, 0, len(es)+len(ss)+keyLen)
	ikm = append(ikm, es...)
	ikm = append(ikm, ss...)
	ikm = append(ikm, clientPub[:]...)
	defer wipe(ikm)
	req = kdf(ikm, ephPub[:], hubPub[:], labelReflexReq)
	base = kdf(ikm, hubPub[:], ephPub[:], labelReflexBase)
	return req, base
}

func reflexRespKey(base [32]byte, dh []byte, hubEph, reqEph [32]byte) [32]byte {
	ikm := make([]byte, 0, keyLen+len(dh))
	ikm = append(ikm, base[:]...)
	ikm = append(ikm, dh...)
	defer wipe(ikm)
	return kdf(ikm, hubEph[:], reqEph[:], labelReflexResp)
}

// BuildReflexReq ...
func BuildReflexReq(device uuid.UUID, discoPriv, serverPub [32]byte, now time.Time, nonce uint64) ([]byte, ReflexPending, error) {
	var pend ReflexPending
	pub, err := Public(discoPriv)
	if err != nil {
		return nil, pend, err
	}
	ephPriv, err := NewPrivate()
	if err != nil {
		return nil, pend, err
	}
	ok := false
	defer func() {
		if !ok {
			wipe32(&ephPriv)
		}
	}()
	ephPub, err := Public(ephPriv)
	if err != nil {
		return nil, pend, err
	}
	es, err := curve25519.X25519(ephPriv[:], serverPub[:])
	if err != nil {
		return nil, pend, err
	}
	defer wipe(es)
	ss, err := curve25519.X25519(discoPriv[:], serverPub[:])
	if err != nil {
		return nil, pend, err
	}
	defer wipe(ss)
	k1 := kdf(es, ephPub[:], serverPub[:], labelReflexS)
	defer wipe32(&k1)
	k2, base := reflexKeys(es, ss, ephPub, serverPub, pub)
	defer wipe32(&k2)

	hd := hubDemuxKey(serverPub)
	salt, err := newSalt()
	if err != nil {
		wipe32(&base)
		return nil, pend, err
	}
	masked := xor32(ephPub, prf(hd, []byte("eph"), salt[:]))
	pkt := make([]byte, 0, reflexReqLen)
	pkt = append(pkt, salt[:]...)
	pkt = append(pkt, reqChk(hd, salt[:])...)
	pkt = append(pkt, masked[:]...)
	sBox, err := sealNonce(k1, zeroNonce[:], pub[:], pkt)
	if err != nil {
		wipe32(&base)
		return nil, pend, err
	}
	pkt = append(pkt, sBox...)
	plain := make([]byte, reflexReqLen-len(pkt)-16)
	plain[0] = Version
	plain[1] = TypeReflexReq
	putID(plain[2:], device)
	binary.BigEndian.PutUint64(plain[2+idLen:], uint64(now.UnixNano()))
	binary.BigEndian.PutUint64(plain[2+idLen+8:], nonce)
	pBox, err := sealNonce(k2, zeroNonce[:], plain, pkt)
	wipe(plain)
	if err != nil {
		wipe32(&base)
		return nil, pend, err
	}
	pkt = append(pkt, pBox...)
	pend = ReflexPending{ephPriv: ephPriv, ephPub: ephPub, base: base, nonce: nonce, live: true}
	ok = true
	wipe32(&ephPriv)
	wipe32(&base)
	return pkt, pend, nil
}

// OpenReflexReq ...
func OpenReflexReq(serverPriv [32]byte, pkt []byte, now time.Time) (ReflexRequest, error) {
	h, err := NewHubKey(serverPriv)
	if err != nil {
		return ReflexRequest{}, err
	}
	defer h.Wipe()
	return h.OpenReflexReq(pkt, now)
}

// IsReflexReq is the cheap keyed pre-filter for reflex requests.
func (h *HubKey) IsReflexReq(pkt []byte) bool {
	return len(pkt) == reflexReqLen && !LooksLikeWireGuard(pkt) &&
		hmacEqual(pkt[saltLen:saltLen+reqChkLen], reqChk(h.demux, pkt[:saltLen]))
}

// OpenReflexReq ...
func (h *HubKey) OpenReflexReq(pkt []byte, now time.Time) (ReflexRequest, error) {
	var out ReflexRequest
	if !h.IsReflexReq(pkt) {
		return out, fmt.Errorf("not a reflex request")
	}
	var masked [32]byte
	copy(masked[:], pkt[saltLen+reqChkLen:reflexReqPrefix])
	ephPub := xor32(masked, prf(h.demux, []byte("eph"), pkt[:saltLen]))
	es, err := curve25519.X25519(h.priv[:], ephPub[:])
	if err != nil {
		return out, err
	}
	defer wipe(es)
	k1 := kdf(es, ephPub[:], h.pub[:], labelReflexS)
	defer wipe32(&k1)
	sEnd := reflexReqPrefix + reflexSBoxLen
	rawPub, err := openNonce(k1, zeroNonce[:], pkt[reflexReqPrefix:sEnd], pkt[:reflexReqPrefix])
	if err != nil {
		return out, err
	}
	if len(rawPub) != keyLen {
		return out, fmt.Errorf("bad reflex request")
	}
	var clientPub [32]byte
	copy(clientPub[:], rawPub)
	ss, err := curve25519.X25519(h.priv[:], clientPub[:])
	if err != nil {
		return out, err
	}
	defer wipe(ss)
	k2, base := reflexKeys(es, ss, ephPub, h.pub, clientPub)
	defer wipe32(&k2)
	plain, err := openNonce(k2, zeroNonce[:], pkt[sEnd:], pkt[:sEnd])
	if err != nil {
		wipe32(&base)
		return out, err
	}
	defer wipe(plain)
	if len(plain) < reflexReqPlain || plain[0] != Version || plain[1] != TypeReflexReq {
		wipe32(&base)
		return out, fmt.Errorf("bad reflex request")
	}
	ts := time.Unix(0, int64(binary.BigEndian.Uint64(plain[2+idLen:])))
	if ts.Before(now.Add(-ReflexSkew)) || ts.After(now.Add(ReflexSkew)) {
		wipe32(&base)
		return out, fmt.Errorf("reflex timestamp outside skew")
	}
	out.Device = getID(plain[2:])
	out.DiscoPub = clientPub
	out.Nonce = binary.BigEndian.Uint64(plain[2+idLen+8:])
	out.ephPub = ephPub
	out.base = base
	out.expires = ts.Add(ReflexSkew)
	return out, nil
}

// BuildReflexResp seals the observed address under a key that mixes a fresh
// hub ephemeral with the request ephemeral.
func BuildReflexResp(req *ReflexRequest, observed netip.AddrPort) ([]byte, error) {
	if req == nil || req.base == ([32]byte{}) {
		return nil, fmt.Errorf("no reflex request")
	}
	hPriv, err := NewPrivate()
	if err != nil {
		return nil, err
	}
	defer wipe32(&hPriv)
	hPub, err := Public(hPriv)
	if err != nil {
		return nil, err
	}
	dh, err := curve25519.X25519(hPriv[:], req.ephPub[:])
	if err != nil {
		return nil, err
	}
	kr := reflexRespKey(req.base, dh, hPub, req.ephPub)
	wipe(dh)
	defer wipe32(&kr)
	salt, err := newSalt()
	if err != nil {
		return nil, err
	}
	masked := xor32(hPub, prf(req.base, []byte("hp"), salt[:]))
	pkt := make([]byte, 0, reflexRespLen)
	pkt = append(pkt, salt[:]...)
	pkt = append(pkt, respChk(req.base, salt[:])...)
	pkt = append(pkt, masked[:]...)
	plain := make([]byte, reflexRespLen-reflexRespPrefix-16)
	plain[0] = Version
	plain[1] = TypeReflexResp
	binary.BigEndian.PutUint64(plain[2:10], req.Nonce)
	putAddrPort(plain[10:], observed)
	box, err := sealNonce(kr, zeroNonce[:], plain, pkt)
	wipe(plain)
	if err != nil {
		return nil, err
	}
	return append(pkt, box...), nil
}

// isReflexResp is the client's cheap keyed pre-filter.
func isReflexResp(pend *ReflexPending, pkt []byte) bool {
	return pend != nil && pend.live && len(pkt) == reflexRespLen &&
		hmacEqual(pkt[saltLen:saltLen+respChkLen], respChk(pend.base, pkt[:saltLen]))
}

// OpenReflexResp ...
func OpenReflexResp(pend *ReflexPending, pkt []byte) (netip.AddrPort, error) {
	if !isReflexResp(pend, pkt) {
		return netip.AddrPort{}, fmt.Errorf("not a reflex response")
	}
	var masked [32]byte
	copy(masked[:], pkt[saltLen+respChkLen:reflexRespPrefix])
	hPub := xor32(masked, prf(pend.base, []byte("hp"), pkt[:saltLen]))
	dh, err := curve25519.X25519(pend.ephPriv[:], hPub[:])
	if err != nil {
		return netip.AddrPort{}, err
	}
	kr := reflexRespKey(pend.base, dh, hPub, pend.ephPub)
	wipe(dh)
	defer wipe32(&kr)
	plain, err := openNonce(kr, zeroNonce[:], pkt[reflexRespPrefix:], pkt[:reflexRespPrefix])
	if err != nil {
		return netip.AddrPort{}, err
	}
	defer wipe(plain)
	if len(plain) < reflexRespPlain || plain[0] != Version || plain[1] != TypeReflexResp {
		return netip.AddrPort{}, fmt.Errorf("bad reflex response")
	}
	if binary.BigEndian.Uint64(plain[2:10]) != pend.nonce {
		return netip.AddrPort{}, fmt.Errorf("reflex nonce mismatch")
	}
	return getAddrPort(plain[10:]), nil
}

// ReplayCache remembers reflex request ephemerals until their timestamp
// leaves the skew window. It holds at most max entries; when full the
// oldest insertion is evicted, so memory is fixed (about 64 bytes/entry).
type ReplayCache struct {
	mu   sync.Mutex
	max  int
	seen map[[32]byte]time.Time
	ring []replayEntry
	head int
	n    int
}

type replayEntry struct {
	id  [32]byte
	exp time.Time
}

// NewReplayCache ...
func NewReplayCache(max int) *ReplayCache {
	if max <= 0 {
		max = replayCacheMax
	}
	return &ReplayCache{max: max, seen: make(map[[32]byte]time.Time), ring: make([]replayEntry, max)}
}

func (c *ReplayCache) popLocked() {
	e := c.ring[c.head]
	if exp, ok := c.seen[e.id]; ok && exp.Equal(e.exp) {
		delete(c.seen, e.id)
	}
	c.ring[c.head] = replayEntry{}
	c.head = (c.head + 1) % c.max
	c.n--
}

// Fresh records id and reports whether it was not seen before (within its
// lifetime). A false result means the request is a replay.
func (c *ReplayCache) Fresh(id [32]byte, expires, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for c.n > 0 && !c.ring[c.head].exp.After(now) {
		c.popLocked()
	}
	if exp, ok := c.seen[id]; ok && exp.After(now) {
		return false
	}
	if c.n == c.max {
		c.popLocked()
	}
	c.ring[(c.head+c.n)%c.max] = replayEntry{id: id, exp: expires}
	c.n++
	c.seen[id] = expires
	return true
}

// Len ...
func (c *ReplayCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.seen)
}
