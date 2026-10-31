package punch

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/chacha20poly1305"
)

const (
	TypeReflexReq  byte = 1
	TypeReflexResp byte = 2
	TypePing       byte = 4
	TypePong       byte = 5
	TypeTeardown   byte = 6
	TypeIntro      byte = 10
)

const IntroPort uint16 = 47318

// magic frames Intro messages only. Intros travel inside the WireGuard
// tunnel to IntroPort, so the magic never appears on the underlay.
var magic = [4]byte{'T', 'N', 'L', 'P'}

const (
	// headerLen is the Intro frame header: magic(4) | type(1).
	headerLen = 5
	keyLen    = 32
	nonceLen  = 24
	idLen     = 16
	addrLen   = 16
	tagLen    = 16
	saltLen   = 16
	tagWindow = time.Minute
	// tagSlack windows either side of the receiver's current window are
	// accepted for static (first-flight) tags, so peers whose clocks differ
	// by up to ReflexSkew still meet.
	tagSlack = 2
	// staticSeqs / sessSeqs are the per-window send budgets (and receive
	// precompute depth) for static and session tags. Each tag is single use.
	staticSeqs = 32
	sessSeqs   = 64
	// sessSlack windows either side of the learned peer window are
	// precomputed for session tags.
	sessSlack = 1
	// innerHdrLen is the sealed {type, body length} prefix.
	innerHdrLen = 3
	// MinPacket is the smallest underlay punch datagram; every non-MTU
	// punch packet is padded to a bucket at least this large.
	MinPacket = 192
)

// padBuckets are the fixed sizes control packets are padded to. Only MTU
// pings and MTU probes deviate: their size is the measurement.
var padBuckets = [...]int{192, 384, 768, 1536}

const MaxPacket = 2048

const ReflexSkew = 2 * time.Minute

// LooksLikeWireGuard reports whether pkt starts like a WireGuard message:
// a little-endian u32 type 1..4, i.e. one type byte and three zero bytes.
func LooksLikeWireGuard(pkt []byte) bool {
	return len(pkt) >= 4 && pkt[0] >= 1 && pkt[0] <= 4 && pkt[1] == 0 && pkt[2] == 0 && pkt[3] == 0
}

// Is reports whether an underlay datagram should go to the punch engine
// rather than WireGuard. Punch packets carry no clear marker; instead every
// sender guarantees its first four bytes never look like WireGuard (salts
// are rerolled, WG-looking tags are skipped), so the split is exact in both
// directions: WG always starts with {1..4,0,0,0}, punch never does.
func Is(pkt []byte) bool {
	return len(pkt) >= MinPacket && len(pkt) <= MaxPacket && !LooksLikeWireGuard(pkt)
}

func isFramed(pkt []byte) bool {
	return len(pkt) >= headerLen && [4]byte(pkt[:4]) == magic
}

func frame(typ byte, body []byte) []byte {
	out := make([]byte, headerLen+len(body))
	copy(out[:4], magic[:])
	out[4] = typ
	copy(out[headerLen:], body)
	return out
}

func bodyOf(pkt []byte, typ byte) ([]byte, error) {
	if !isFramed(pkt) {
		return nil, fmt.Errorf("not a punch packet")
	}
	if pkt[4] != typ {
		return nil, fmt.Errorf("punch type %d, want %d", pkt[4], typ)
	}
	if len(pkt) > MaxPacket {
		return nil, fmt.Errorf("punch packet too large")
	}
	return pkt[headerLen:], nil
}

func putID(b []byte, id uuid.UUID) {
	copy(b[:idLen], id[:])
}

func getID(b []byte) uuid.UUID {
	var id uuid.UUID
	copy(id[:], b[:idLen])
	return id
}

func putAddr(b []byte, a netip.Addr) {
	if !a.IsValid() {
		return
	}
	raw := a.As16()
	copy(b[:addrLen], raw[:])
}

func getAddr(b []byte) netip.Addr {
	var raw [16]byte
	copy(raw[:], b[:addrLen])
	addr := netip.AddrFrom16(raw).Unmap()
	if !addr.IsValid() || addr.IsUnspecified() {
		return netip.Addr{}
	}
	return addr
}

func putAddrPort(b []byte, ap netip.AddrPort) {
	putAddr(b, ap.Addr())
	binary.BigEndian.PutUint16(b[addrLen:addrLen+2], ap.Port())
}

func getAddrPort(b []byte) netip.AddrPort {
	addr := getAddr(b)
	if !addr.IsValid() {
		return netip.AddrPort{}
	}
	return netip.AddrPortFrom(addr, binary.BigEndian.Uint16(b[addrLen:addrLen+2]))
}

type Intro struct {
	Src        uuid.UUID
	DiscoPub   [32]byte
	WGPub      [32]byte
	WGIP       netip.Addr
	Candidates []netip.AddrPort
}

// BuildIntro ...
func BuildIntro(src uuid.UUID, discoPub, wgPub [32]byte, wgIP netip.Addr, cands []netip.AddrPort) ([]byte, error) {
	if !wgIP.IsValid() || len(cands) > 16 {
		return nil, fmt.Errorf("bad intro")
	}
	body := make([]byte, idLen+keyLen+keyLen+addrLen+1+(addrLen+2)*len(cands))
	putID(body, src)
	copy(body[idLen:], discoPub[:])
	copy(body[idLen+keyLen:], wgPub[:])
	putAddr(body[idLen+keyLen*2:], wgIP)
	off := idLen + keyLen*2 + addrLen
	body[off] = byte(len(cands))
	off++
	for _, c := range cands {
		putAddrPort(body[off:], c)
		off += addrLen + 2
	}
	return frame(TypeIntro, body), nil
}

// ParseIntro ...
func ParseIntro(pkt []byte) (Intro, error) {
	var in Intro
	body, err := bodyOf(pkt, TypeIntro)
	if err != nil {
		return in, err
	}
	if len(body) < idLen+keyLen*2+addrLen+1 {
		return in, fmt.Errorf("short intro")
	}
	in.Src = getID(body)
	copy(in.DiscoPub[:], body[idLen:idLen+keyLen])
	copy(in.WGPub[:], body[idLen+keyLen:idLen+keyLen*2])
	in.WGIP = getAddr(body[idLen+keyLen*2:])
	n := int(body[idLen+keyLen*2+addrLen])
	off := idLen + keyLen*2 + addrLen + 1
	if n > 16 || len(body) != off+(addrLen+2)*n || !in.WGIP.IsValid() {
		return Intro{}, fmt.Errorf("bad intro")
	}
	for i := 0; i < n; i++ {
		in.Candidates = append(in.Candidates, getAddrPort(body[off:]))
		off += addrLen + 2
	}
	return in, nil
}

// WrapIPv4UDP ...
func WrapIPv4UDP(src, dst netip.Addr, srcPort, dstPort uint16, payload []byte) ([]byte, error) {
	if !src.Is4() || !dst.Is4() {
		return nil, fmt.Errorf("ipv4 only")
	}
	pkt := make([]byte, 20+8+len(payload))
	pkt[0] = 0x45
	binary.BigEndian.PutUint16(pkt[2:4], uint16(len(pkt)))
	pkt[8] = 64
	pkt[9] = 17
	src4 := src.As4()
	dst4 := dst.As4()
	copy(pkt[12:16], src4[:])
	copy(pkt[16:20], dst4[:])
	checksumIPv4(pkt[:20])
	binary.BigEndian.PutUint16(pkt[20:22], srcPort)
	binary.BigEndian.PutUint16(pkt[22:24], dstPort)
	binary.BigEndian.PutUint16(pkt[24:26], uint16(8+len(payload)))
	copy(pkt[28:], payload)
	return pkt, nil
}

// UnwrapIPv4UDP ...
func UnwrapIPv4UDP(pkt []byte, dstPort uint16) (src netip.Addr, srcPort uint16, payload []byte, ok bool) {
	if len(pkt) < 28 || pkt[0]>>4 != 4 {
		return netip.Addr{}, 0, nil, false
	}
	ihl := int(pkt[0]&0x0f) * 4
	if ihl < 20 || pkt[9] != 17 || len(pkt) < ihl+8 {
		return netip.Addr{}, 0, nil, false
	}
	if binary.BigEndian.Uint16(pkt[ihl+2:ihl+4]) != dstPort {
		return netip.Addr{}, 0, nil, false
	}
	src = netip.AddrFrom4([4]byte{pkt[12], pkt[13], pkt[14], pkt[15]})
	srcPort = binary.BigEndian.Uint16(pkt[ihl : ihl+2])
	return src, srcPort, pkt[ihl+8:], true
}

func checksumIPv4(hdr []byte) {
	hdr[10] = 0
	hdr[11] = 0
	var csum uint32
	for i := 0; i+1 < len(hdr); i += 2 {
		csum += uint32(hdr[i])<<8 | uint32(hdr[i+1])
	}
	for csum > 0xFFFF {
		csum = (csum >> 16) + (csum & 0xFFFF)
	}
	hdr[10] = byte(^csum >> 8)
	hdr[11] = byte(^csum & 0xFF)
}

func sealNonce(key [32]byte, nonce, plain, aad []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key[:])
	if err != nil {
		return nil, err
	}
	return aead.Seal(nil, nonce, plain, aad), nil
}

func openNonce(key [32]byte, nonce, ct, aad []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key[:])
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nonce, ct, aad)
}

var zeroNonce [nonceLen]byte

// padLen is the bucket a packet of n bytes is padded to.
func padLen(n int) int {
	for _, b := range padBuckets {
		if n <= b {
			return b
		}
	}
	return n
}

// newSalt returns a random salt whose first four bytes never look like a
// WireGuard header. Salted packets start with their salt.
func newSalt() ([saltLen]byte, error) {
	var s [saltLen]byte
	for {
		if _, err := rand.Read(s[:]); err != nil {
			return s, err
		}
		if !LooksLikeWireGuard(s[:]) {
			return s, nil
		}
	}
}

func xor32(a, b [32]byte) [32]byte {
	var out [32]byte
	for i := range out {
		out[i] = a[i] ^ b[i]
	}
	return out
}

// Direct packets (Ping, Pong, Teardown):
//
//	tag(16) | eph^mask(32) | nonce(24) | AEAD{type(1) len(2) src(16) dst(16) body | zero pad}
//
// tag  = BLAKE2s(tagKey, window | seq)[:16]: single use, per direction,
//
//	from the static pair key before a session exists and from the session
//	(ee) key after. Tags that would look like WireGuard are skipped by both
//	sides, so the first four bytes never collide with WG types 1..4.
//
// mask = BLAKE2s(hpKey, nonce); hpKey follows the tag phase.
// AAD  = tag | eph^mask. The type lives inside the sealed part.
//
// Non-MTU packets are padded to padBuckets; MTU pings are sized exactly.

type directOut struct {
	key [32]byte
	hp  [32]byte
	tag [tagLen]byte
}

type directParts struct {
	tag    [tagLen]byte
	masked [32]byte
	nonce  []byte
	ct     []byte
	aad    []byte
}

const directPrefix = tagLen + keyLen

func window(now time.Time) int64 {
	return now.Unix() / int64(tagWindow/time.Second)
}

// slotTag is the tag for sequence seq of window w. ok is false when the tag
// would look like a WireGuard header; that slot is never used.
func slotTag(tagKey [32]byte, w int64, seq int) (tag [tagLen]byte, ok bool) {
	var b [10]byte
	binary.BigEndian.PutUint64(b[:8], uint64(w))
	binary.BigEndian.PutUint16(b[8:], uint16(seq))
	sum := prf(tagKey, b[:])
	copy(tag[:], sum[:tagLen])
	return tag, !LooksLikeWireGuard(tag[:])
}

func ephMask(hp [32]byte, nonce []byte) [32]byte {
	return prf(hp, nonce)
}

func sealDirect(typ byte, out directOut, eph [32]byte, src, dst uuid.UUID, body []byte, exact bool) ([]byte, error) {
	if len(body) > 0xffff {
		return nil, fmt.Errorf("punch body too large")
	}
	n := directPacketLen(len(body))
	if !exact {
		n = padLen(n)
	}
	if n > MaxPacket {
		return nil, fmt.Errorf("punch packet too large")
	}
	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	masked := xor32(eph, ephMask(out.hp, nonce))
	pkt := make([]byte, 0, n)
	pkt = append(pkt, out.tag[:]...)
	pkt = append(pkt, masked[:]...)
	plain := make([]byte, n-directPrefix-nonceLen-16)
	plain[0] = typ
	binary.BigEndian.PutUint16(plain[1:3], uint16(len(body)))
	putID(plain[innerHdrLen:], src)
	putID(plain[innerHdrLen+idLen:], dst)
	copy(plain[innerHdrLen+idLen*2:], body)
	ct, err := sealNonce(out.key, nonce, plain, pkt[:directPrefix])
	wipe(plain)
	if err != nil {
		return nil, err
	}
	pkt = append(pkt, nonce...)
	pkt = append(pkt, ct...)
	return pkt, nil
}

func splitDirect(pkt []byte) (directParts, error) {
	var p directParts
	if len(pkt) < directPacketLen(0) || len(pkt) > MaxPacket {
		return p, fmt.Errorf("bad punch length")
	}
	copy(p.tag[:], pkt[:tagLen])
	copy(p.masked[:], pkt[tagLen:directPrefix])
	p.nonce = pkt[directPrefix : directPrefix+nonceLen]
	p.ct = pkt[directPrefix+nonceLen:]
	p.aad = pkt[:directPrefix]
	return p, nil
}

func (p *directParts) eph(hp [32]byte) [32]byte {
	return xor32(p.masked, ephMask(hp, p.nonce))
}

func (p *directParts) open(key [32]byte) (typ byte, body []byte, src, dst uuid.UUID, err error) {
	plain, err := openNonce(key, p.nonce, p.ct, p.aad)
	if err != nil {
		return 0, nil, src, dst, err
	}
	if len(plain) < innerHdrLen+idLen*2 {
		return 0, nil, src, dst, fmt.Errorf("bad punch plaintext")
	}
	n := int(binary.BigEndian.Uint16(plain[1:3]))
	off := innerHdrLen + idLen*2
	if off+n > len(plain) {
		return 0, nil, src, dst, fmt.Errorf("bad punch body length")
	}
	return plain[0], plain[off : off+n], getID(plain[innerHdrLen:]), getID(plain[innerHdrLen+idLen:]), nil
}

// staticTagMatch checks a tag against every static slot around now. Only
// the stateless helpers use it; the engine keeps an O(1) map instead.
func staticTagMatch(tagKey [32]byte, tag [tagLen]byte, now time.Time) bool {
	w := window(now)
	for d := int64(-tagSlack); d <= tagSlack; d++ {
		for seq := 0; seq < staticSeqs; seq++ {
			t, ok := slotTag(tagKey, w+d, seq)
			if ok && hmacEqual(t[:], tag[:]) {
				return true
			}
		}
	}
	return false
}

// statelessSeq hands the stateless builders distinct slots so that up to
// staticSeqs packets per window never share a (single-use) tag. It starts
// at a random offset; the engine itself uses per-session counters.
var statelessSeq atomic.Uint32

func init() {
	var b [4]byte
	if _, err := rand.Read(b[:]); err == nil {
		statelessSeq.Store(binary.BigEndian.Uint32(b[:]))
	}
}

func randomStaticTag(tagKey [32]byte, now time.Time) ([tagLen]byte, error) {
	w := window(now)
	for i := 0; i < staticSeqs; i++ {
		seq := int(statelessSeq.Add(1) % staticSeqs)
		if t, ok := slotTag(tagKey, w, seq); ok {
			return t, nil
		}
	}
	return [tagLen]byte{}, errTagBudget
}

func buildSealedAt(typ byte, srcPriv, dstPub [32]byte, src, dst uuid.UUID, plain []byte, now time.Time, exact bool) ([]byte, error) {
	ephPriv, err := NewPrivate()
	if err != nil {
		return nil, err
	}
	ephPub, err := Public(ephPriv)
	wipe32(&ephPriv)
	if err != nil {
		return nil, err
	}
	k, err := derivePair(srcPriv, dstPub)
	if err != nil {
		return nil, err
	}
	defer k.wipe()
	tag, err := randomStaticTag(k.tagSend, now)
	if err != nil {
		return nil, err
	}
	out := directOut{key: k.send, hp: k.hpSend, tag: tag}
	return sealDirect(typ, out, ephPub, src, dst, plain, exact)
}

func buildSealed(typ byte, srcPriv, dstPub [32]byte, src, dst uuid.UUID, plain []byte) ([]byte, error) {
	return buildSealedAt(typ, srcPriv, dstPub, src, dst, plain, time.Now(), false)
}

func openSealedAt(typ byte, dstPriv, srcPub [32]byte, pkt []byte, now time.Time) (plain []byte, src, dst uuid.UUID, err error) {
	p, err := splitDirect(pkt)
	if err != nil {
		return nil, src, dst, err
	}
	k, err := derivePair(dstPriv, srcPub)
	if err != nil {
		return nil, src, dst, err
	}
	defer k.wipe()
	if !staticTagMatch(k.tagRecv, p.tag, now) {
		return nil, src, dst, fmt.Errorf("unknown punch tag")
	}
	got, plain, src, dst, err := p.open(k.recv)
	if err != nil {
		return nil, src, dst, err
	}
	if got != typ {
		return nil, src, dst, fmt.Errorf("punch type %d, want %d", got, typ)
	}
	return plain, src, dst, nil
}

func openSealed(typ byte, dstPriv, srcPub [32]byte, pkt []byte) (plain []byte, src, dst uuid.UUID, err error) {
	return openSealedAt(typ, dstPriv, srcPub, pkt, time.Now())
}

func pingTxid(plain []byte) (uint64, error) {
	if len(plain) < 8 {
		return 0, fmt.Errorf("bad ping body")
	}
	for _, b := range plain[8:] {
		if b != 0 {
			return 0, fmt.Errorf("bad ping body")
		}
	}
	return binary.BigEndian.Uint64(plain[:8]), nil
}

// BuildPing ...
func BuildPing(srcPriv, dstPub [32]byte, src, dst uuid.UUID, txid uint64) ([]byte, error) {
	var plain [8]byte
	binary.BigEndian.PutUint64(plain[:], txid)
	return buildSealed(TypePing, srcPriv, dstPub, src, dst, plain[:])
}

// BuildPingPad builds an MTU ping of exactly directPacketLen(plainLen)
// bytes. MTU pings are not bucket padded: their size is the measurement.
func BuildPingPad(srcPriv, dstPub [32]byte, src, dst uuid.UUID, txid uint64, plainLen int) ([]byte, error) {
	if plainLen < 8 {
		return nil, fmt.Errorf("short ping")
	}
	if directPacketLen(plainLen) > MaxPacket {
		return nil, fmt.Errorf("ping too large")
	}
	plain := make([]byte, plainLen)
	binary.BigEndian.PutUint64(plain[:8], txid)
	return buildSealedAt(TypePing, srcPriv, dstPub, src, dst, plain, time.Now(), true)
}

// OpenPing ...
func OpenPing(dstPriv, srcPub [32]byte, pkt []byte) (src, dst uuid.UUID, txid uint64, err error) {
	plain, src, dst, err := openSealed(TypePing, dstPriv, srcPub, pkt)
	if err != nil {
		return src, dst, 0, err
	}
	txid, err = pingTxid(plain)
	if err != nil {
		return src, dst, 0, err
	}
	return src, dst, txid, nil
}

// directPacketLen is the unpadded UDP payload size of a direct packet whose
// body (txid plus padding for pings) is plainLen bytes.
func directPacketLen(plainLen int) int {
	return tagLen + keyLen + nonceLen + innerHdrLen + idLen*2 + plainLen + 16
}

// MTU probes to the hub are sent before the hub disco key is known, so
// their marker is keyed with a public constant: it hides the probe from
// naive pattern matching (no fixed bytes) but not from anyone who reads
// this code. Probes are size-revealing by design; acks are bucket padded.
var mtuKey = kdf([]byte("tunnels.is punch mtu"), nil, nil, "mtu marker")

const mtuChkLen = 16

func mtuProbeChk(salt []byte) []byte {
	sum := prf(mtuKey, []byte("probe"), salt)
	return sum[:mtuChkLen]
}

func mtuAckChk(probeSalt, salt []byte) []byte {
	sum := prf(mtuKey, []byte("ack"), probeSalt, salt)
	return sum[:mtuChkLen]
}

// BuildMTUProbe ...
func BuildMTUProbe(payloadLen int) ([]byte, error) {
	if payloadLen < MinPacket || payloadLen > MaxPacket {
		return nil, fmt.Errorf("bad mtu probe length")
	}
	salt, err := newSalt()
	if err != nil {
		return nil, err
	}
	pkt := make([]byte, payloadLen)
	copy(pkt, salt[:])
	copy(pkt[saltLen:], mtuProbeChk(salt[:]))
	if _, err := rand.Read(pkt[saltLen+mtuChkLen:]); err != nil {
		return nil, err
	}
	return pkt, nil
}

// IsMTUProbe ...
func IsMTUProbe(pkt []byte) bool {
	if !Is(pkt) {
		return false
	}
	return hmacEqual(pkt[saltLen:saltLen+mtuChkLen], mtuProbeChk(pkt[:saltLen]))
}

// MTUAckFor returns the ack for an MTU probe, or nil. The ack is one
// MinPacket bucket, never larger than the probe.
func MTUAckFor(pkt []byte) []byte {
	if !IsMTUProbe(pkt) {
		return nil
	}
	salt, err := newSalt()
	if err != nil {
		return nil
	}
	ack := make([]byte, MinPacket)
	copy(ack, salt[:])
	copy(ack[saltLen:], mtuAckChk(pkt[:saltLen], salt[:]))
	if _, err := rand.Read(ack[saltLen+mtuChkLen:]); err != nil {
		return nil
	}
	return ack
}

// MTUAckMatches reports whether ack answers probe.
func MTUAckMatches(probe, ack []byte) bool {
	if len(probe) < MinPacket || len(ack) != MinPacket || LooksLikeWireGuard(ack) {
		return false
	}
	return hmacEqual(ack[saltLen:saltLen+mtuChkLen], mtuAckChk(probe[:saltLen], ack[:saltLen]))
}

// BuildPong ...
func BuildPong(srcPriv, dstPub [32]byte, src, dst uuid.UUID, txid uint64) ([]byte, error) {
	var plain [8]byte
	binary.BigEndian.PutUint64(plain[:], txid)
	return buildSealed(TypePong, srcPriv, dstPub, src, dst, plain[:])
}

// OpenPong ...
func OpenPong(dstPriv, srcPub [32]byte, pkt []byte) (src, dst uuid.UUID, txid uint64, err error) {
	plain, src, dst, err := openSealed(TypePong, dstPriv, srcPub, pkt)
	if err != nil {
		return src, dst, 0, err
	}
	if len(plain) != 8 {
		return src, dst, 0, fmt.Errorf("bad pong body")
	}
	return src, dst, binary.BigEndian.Uint64(plain), nil
}

// BuildTeardown ...
func BuildTeardown(srcPriv, dstPub [32]byte, src, dst uuid.UUID, epoch uint64) ([]byte, error) {
	var plain [8]byte
	binary.BigEndian.PutUint64(plain[:], epoch)
	return buildSealed(TypeTeardown, srcPriv, dstPub, src, dst, plain[:])
}

// OpenTeardown ...
func OpenTeardown(dstPriv, srcPub [32]byte, pkt []byte) (src, dst uuid.UUID, epoch uint64, err error) {
	plain, src, dst, err := openSealed(TypeTeardown, dstPriv, srcPub, pkt)
	if err != nil {
		return src, dst, 0, err
	}
	if len(plain) != 8 {
		return src, dst, 0, fmt.Errorf("bad teardown")
	}
	return src, dst, binary.BigEndian.Uint64(plain), nil
}

// TypeOf returns the type of a TNLP-framed message. Only Intros are framed,
// and they travel inside the tunnel; underlay packets have no clear type.
func TypeOf(pkt []byte) byte {
	if !isFramed(pkt) {
		return 0
	}
	return pkt[4]
}
