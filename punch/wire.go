package punch

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
)

const (
	Version byte = 2

	TypeReflexReq  byte = 1
	TypeReflexResp byte = 2
	TypeRendezvous byte = 3
	TypePing       byte = 4
	TypePong       byte = 5
	TypeTeardown   byte = 6
	TypeForward    byte = 7
	TypeMTUProbe   byte = 8
	TypeMTUAck     byte = 9
	TypeIntro      byte = 10
)

const IntroPort uint16 = 47318

var magic = [4]byte{'T', 'N', 'L', 'P'}

const (
	headerLen = 6
	keyLen    = 32
	nonceLen  = 24
	idLen     = 16
	addrLen   = 16
)

const MaxPacket = 2048

const ReflexSkew = 2 * time.Minute

// Is ...
func Is(pkt []byte) bool {
	return len(pkt) >= headerLen && [4]byte(pkt[:4]) == magic && pkt[4] == Version
}

func frame(typ byte, body []byte) []byte {
	out := make([]byte, headerLen+len(body))
	copy(out[:4], magic[:])
	out[4] = Version
	out[5] = typ
	copy(out[headerLen:], body)
	return out
}

func bodyOf(pkt []byte, typ byte) ([]byte, error) {
	if !Is(pkt) {
		return nil, fmt.Errorf("not a punch packet")
	}
	if pkt[5] != typ {
		return nil, fmt.Errorf("punch type %d, want %d", pkt[5], typ)
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

func reflexReqAAD(id uuid.UUID, pub [32]byte) []byte {
	aad := make([]byte, headerLen+idLen+keyLen)
	copy(aad[:4], magic[:])
	aad[4] = Version
	aad[5] = TypeReflexReq
	putID(aad[headerLen:], id)
	copy(aad[headerLen+idLen:], pub[:])
	return aad
}

func reflexRespAAD(id uuid.UUID) []byte {
	aad := make([]byte, headerLen+idLen)
	copy(aad[:4], magic[:])
	aad[4] = Version
	aad[5] = TypeReflexResp
	putID(aad[headerLen:], id)
	return aad
}

// BuildReflexReq ...
func BuildReflexReq(device uuid.UUID, discoPriv, serverPub [32]byte, now time.Time, nonce uint64) ([]byte, error) {
	pub, err := Public(discoPriv)
	if err != nil {
		return nil, err
	}
	var plain [16]byte
	binary.BigEndian.PutUint64(plain[:8], uint64(now.UnixNano()))
	binary.BigEndian.PutUint64(plain[8:], nonce)
	box, err := Seal(discoPriv, serverPub, plain[:], reflexReqAAD(device, pub))
	if err != nil {
		return nil, err
	}
	body := make([]byte, idLen+keyLen+len(box))
	putID(body, device)
	copy(body[idLen:idLen+keyLen], pub[:])
	copy(body[idLen+keyLen:], box)
	return frame(TypeReflexReq, body), nil
}

// OpenReflexReq ...
func OpenReflexReq(serverPriv [32]byte, pkt []byte, now time.Time) (uuid.UUID, [32]byte, uint64, error) {
	var pub [32]byte
	body, err := bodyOf(pkt, TypeReflexReq)
	if err != nil {
		return uuid.UUID{}, pub, 0, err
	}
	if len(body) != idLen+keyLen+nonceLen+16+16 {
		return uuid.UUID{}, pub, 0, fmt.Errorf("short reflex request")
	}
	id := getID(body)
	copy(pub[:], body[idLen:idLen+keyLen])
	plain, err := Open(serverPriv, pub, body[idLen+keyLen:], reflexReqAAD(id, pub))
	if err != nil {
		return uuid.UUID{}, pub, 0, err
	}
	if len(plain) != 16 {
		return uuid.UUID{}, pub, 0, fmt.Errorf("bad reflex request")
	}
	ts := time.Unix(0, int64(binary.BigEndian.Uint64(plain[:8])))
	if ts.Before(now.Add(-ReflexSkew)) || ts.After(now.Add(ReflexSkew)) {
		return uuid.UUID{}, pub, 0, fmt.Errorf("reflex timestamp outside skew")
	}
	return id, pub, binary.BigEndian.Uint64(plain[8:]), nil
}

// BuildReflexResp ...
func BuildReflexResp(serverPriv, clientPub [32]byte, device uuid.UUID, observed netip.AddrPort, nonce uint64) ([]byte, error) {
	var plain [8 + addrLen + 2]byte
	binary.BigEndian.PutUint64(plain[:8], nonce)
	putAddrPort(plain[8:], observed)
	box, err := Seal(serverPriv, clientPub, plain[:], reflexRespAAD(device))
	if err != nil {
		return nil, err
	}
	body := make([]byte, idLen+len(box))
	putID(body, device)
	copy(body[idLen:], box)
	return frame(TypeReflexResp, body), nil
}

// OpenReflexResp ...
func OpenReflexResp(clientPriv, serverPub [32]byte, pkt []byte) (uuid.UUID, netip.AddrPort, uint64, error) {
	body, err := bodyOf(pkt, TypeReflexResp)
	if err != nil {
		return uuid.UUID{}, netip.AddrPort{}, 0, err
	}
	if len(body) != idLen+nonceLen+8+addrLen+2+16 {
		return uuid.UUID{}, netip.AddrPort{}, 0, fmt.Errorf("bad reflex response length")
	}
	id := getID(body)
	plain, err := Open(clientPriv, serverPub, body[idLen:], reflexRespAAD(id))
	if err != nil {
		return uuid.UUID{}, netip.AddrPort{}, 0, err
	}
	if len(plain) != 8+addrLen+2 {
		return uuid.UUID{}, netip.AddrPort{}, 0, fmt.Errorf("bad reflex response")
	}
	return id, getAddrPort(plain[8:]), binary.BigEndian.Uint64(plain[:8]), nil
}

func directAAD(typ byte, src, dst uuid.UUID, eph [32]byte) []byte {
	aad := make([]byte, headerLen+idLen*2+keyLen)
	copy(aad[:4], magic[:])
	aad[4] = Version
	aad[5] = typ
	putID(aad[headerLen:], src)
	putID(aad[headerLen+idLen:], dst)
	copy(aad[headerLen+idLen*2:], eph[:])
	return aad
}

func sealDirect(typ byte, key, eph [32]byte, src, dst uuid.UUID, plain []byte) ([]byte, error) {
	box, err := sealKey(key, plain, directAAD(typ, src, dst, eph))
	if err != nil {
		return nil, err
	}
	body := make([]byte, idLen*2+keyLen+len(box))
	putID(body, src)
	putID(body[idLen:], dst)
	copy(body[idLen*2:idLen*2+keyLen], eph[:])
	copy(body[idLen*2+keyLen:], box)
	return frame(typ, body), nil
}

func splitDirect(pkt []byte, typ byte) (src, dst uuid.UUID, eph [32]byte, box []byte, err error) {
	body, err := bodyOf(pkt, typ)
	if err != nil {
		return src, dst, eph, nil, err
	}
	if len(body) < idLen*2+keyLen+nonceLen+16 {
		return src, dst, eph, nil, fmt.Errorf("short punch body")
	}
	src = getID(body)
	dst = getID(body[idLen:])
	copy(eph[:], body[idLen*2:idLen*2+keyLen])
	box = body[idLen*2+keyLen:]
	return src, dst, eph, box, nil
}

func openDirect(typ byte, key [32]byte, pkt []byte) (plain []byte, src, dst uuid.UUID, eph [32]byte, err error) {
	src, dst, eph, box, err := splitDirect(pkt, typ)
	if err != nil {
		return nil, src, dst, eph, err
	}
	plain, err = openKey(key, box, directAAD(typ, src, dst, eph))
	return plain, src, dst, eph, err
}

func buildSealed(typ byte, srcPriv, dstPub [32]byte, src, dst uuid.UUID, plain []byte) ([]byte, error) {
	ephPriv, err := NewPrivate()
	if err != nil {
		return nil, err
	}
	ephPub, err := Public(ephPriv)
	if err != nil {
		return nil, err
	}
	send, _, err := directionalKeys(srcPriv, dstPub)
	if err != nil {
		return nil, err
	}
	return sealDirect(typ, send, ephPub, src, dst, plain)
}

func openSealed(typ byte, dstPriv, srcPub [32]byte, pkt []byte) (plain []byte, src, dst uuid.UUID, err error) {
	_, recv, err := directionalKeys(dstPriv, srcPub)
	if err != nil {
		return nil, src, dst, err
	}
	plain, src, dst, _, err = openDirect(typ, recv, pkt)
	return plain, src, dst, err
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

// BuildPingPad ...
func BuildPingPad(srcPriv, dstPub [32]byte, src, dst uuid.UUID, txid uint64, plainLen int) ([]byte, error) {
	if plainLen < 8 {
		return nil, fmt.Errorf("short ping")
	}
	if directPacketLen(plainLen) > MaxPacket {
		return nil, fmt.Errorf("ping too large")
	}
	plain := make([]byte, plainLen)
	binary.BigEndian.PutUint64(plain[:8], txid)
	return buildSealed(TypePing, srcPriv, dstPub, src, dst, plain)
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

func directPacketLen(plainLen int) int {
	return headerLen + idLen*2 + keyLen + nonceLen + plainLen + 16
}

// BuildMTUProbe ...
func BuildMTUProbe(payloadLen int) ([]byte, error) {
	if payloadLen < headerLen || payloadLen > MaxPacket {
		return nil, fmt.Errorf("bad mtu probe length")
	}
	return frame(TypeMTUProbe, make([]byte, payloadLen-headerLen)), nil
}

// BuildMTUAck ...
func BuildMTUAck() []byte {
	return frame(TypeMTUAck, nil)
}

// IsMTUAck ...
func IsMTUAck(pkt []byte) bool {
	return len(pkt) == headerLen && TypeOf(pkt) == TypeMTUAck
}

// MTUAckFor ...
func MTUAckFor(pkt []byte) []byte {
	if TypeOf(pkt) != TypeMTUProbe || len(pkt) < headerLen || len(pkt) > MaxPacket {
		return nil
	}
	return BuildMTUAck()
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

// TypeOf ...
func TypeOf(pkt []byte) byte {
	if !Is(pkt) {
		return 0
	}
	return pkt[5]
}
