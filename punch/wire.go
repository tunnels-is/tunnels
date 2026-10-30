package punch

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
)

const (
	Version byte = 1

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

// BuildReflexReq ...
func BuildReflexReq(device uuid.UUID, discoPriv, serverPub [32]byte, now time.Time) ([]byte, error) {
	pub, err := Public(discoPriv)
	if err != nil {
		return nil, err
	}
	var plain [8]byte
	binary.BigEndian.PutUint64(plain[:], uint64(now.UnixNano()))
	box, err := Seal(discoPriv, serverPub, plain[:])
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
func OpenReflexReq(serverPriv [32]byte, pkt []byte, now time.Time) (uuid.UUID, [32]byte, error) {
	var pub [32]byte
	body, err := bodyOf(pkt, TypeReflexReq)
	if err != nil {
		return uuid.UUID{}, pub, err
	}
	if len(body) < idLen+keyLen+nonceLen+8+16 {
		return uuid.UUID{}, pub, fmt.Errorf("short reflex request")
	}
	id := getID(body)
	copy(pub[:], body[idLen:idLen+keyLen])
	plain, err := Open(serverPriv, pub, body[idLen+keyLen:])
	if err != nil {
		return uuid.UUID{}, pub, err
	}
	if len(plain) != 8 {
		return uuid.UUID{}, pub, fmt.Errorf("bad reflex timestamp")
	}
	ts := time.Unix(0, int64(binary.BigEndian.Uint64(plain)))
	if ts.Before(now.Add(-ReflexSkew)) || ts.After(now.Add(ReflexSkew)) {
		return uuid.UUID{}, pub, fmt.Errorf("reflex timestamp outside skew")
	}
	return id, pub, nil
}

// BuildReflexResp ...
func BuildReflexResp(device uuid.UUID, observed netip.AddrPort) []byte {
	body := make([]byte, idLen+addrLen+2)
	putID(body, device)
	putAddrPort(body[idLen:], observed)
	return frame(TypeReflexResp, body)
}

// ParseReflexResp ...
func ParseReflexResp(pkt []byte) (uuid.UUID, netip.AddrPort, error) {
	body, err := bodyOf(pkt, TypeReflexResp)
	if err != nil {
		return uuid.UUID{}, netip.AddrPort{}, err
	}
	if len(body) != idLen+addrLen+2 {
		return uuid.UUID{}, netip.AddrPort{}, fmt.Errorf("bad reflex response length")
	}
	return getID(body), getAddrPort(body[idLen:]), nil
}

// BuildPing ...
func BuildPing(srcPriv, dstPub [32]byte, src, dst uuid.UUID, txid uint64) ([]byte, error) {
	var plain [8]byte
	binary.BigEndian.PutUint64(plain[:], txid)
	return sealDirect(TypePing, srcPriv, dstPub, src, dst, plain[:])
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
	return sealDirect(TypePing, srcPriv, dstPub, src, dst, plain)
}

// OpenPing ...
func OpenPing(dstPriv, srcPub [32]byte, pkt []byte) (src, dst uuid.UUID, txid uint64, err error) {
	plain, src, dst, err := openDirect(TypePing, dstPriv, srcPub, pkt)
	if err != nil {
		return src, dst, 0, err
	}
	if len(plain) < 8 {
		return src, dst, 0, fmt.Errorf("bad ping body")
	}
	return src, dst, binary.BigEndian.Uint64(plain[:8]), nil
}

func directPacketLen(plainLen int) int {
	return headerLen + idLen*2 + nonceLen + plainLen + 16
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
func BuildPong(srcPriv, dstPub [32]byte, src, dst uuid.UUID, txid uint64, observed netip.AddrPort) ([]byte, error) {
	plain := make([]byte, 8+addrLen+2)
	binary.BigEndian.PutUint64(plain[:8], txid)
	putAddrPort(plain[8:], observed)
	return sealDirect(TypePong, srcPriv, dstPub, src, dst, plain)
}

// OpenPong ...
func OpenPong(dstPriv, srcPub [32]byte, pkt []byte) (src, dst uuid.UUID, txid uint64, observed netip.AddrPort, err error) {
	plain, src, dst, err := openDirect(TypePong, dstPriv, srcPub, pkt)
	if err != nil {
		return src, dst, 0, netip.AddrPort{}, err
	}
	if len(plain) != 8+addrLen+2 {
		return src, dst, 0, netip.AddrPort{}, fmt.Errorf("bad pong body")
	}
	return src, dst, binary.BigEndian.Uint64(plain[:8]), getAddrPort(plain[8:]), nil
}

// BuildTeardown ...
func BuildTeardown(srcPriv, dstPub [32]byte, src, dst uuid.UUID) ([]byte, error) {
	return sealDirect(TypeTeardown, srcPriv, dstPub, src, dst, []byte{1})
}

// OpenTeardown ...
func OpenTeardown(dstPriv, srcPub [32]byte, pkt []byte) (src, dst uuid.UUID, err error) {
	_, src, dst, err = openDirect(TypeTeardown, dstPriv, srcPub, pkt)
	return src, dst, err
}

func sealDirect(typ byte, srcPriv, dstPub [32]byte, src, dst uuid.UUID, plain []byte) ([]byte, error) {
	box, err := Seal(srcPriv, dstPub, plain)
	if err != nil {
		return nil, err
	}
	body := make([]byte, idLen*2+len(box))
	putID(body, src)
	putID(body[idLen:], dst)
	copy(body[idLen*2:], box)
	return frame(typ, body), nil
}

func openDirect(typ byte, dstPriv, srcPub [32]byte, pkt []byte) ([]byte, uuid.UUID, uuid.UUID, error) {
	body, err := bodyOf(pkt, typ)
	if err != nil {
		return nil, uuid.UUID{}, uuid.UUID{}, err
	}
	if len(body) < idLen*2+nonceLen+16 {
		return nil, uuid.UUID{}, uuid.UUID{}, fmt.Errorf("short punch body")
	}
	src := getID(body)
	dst := getID(body[idLen:])
	plain, err := Open(dstPriv, srcPub, body[idLen*2:])
	if err != nil {
		return nil, src, dst, err
	}
	return plain, src, dst, nil
}

// TypeOf ...
func TypeOf(pkt []byte) byte {
	if !Is(pkt) {
		return 0
	}
	return pkt[5]
}
