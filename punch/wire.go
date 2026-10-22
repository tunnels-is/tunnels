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
)

var magic = [4]byte{'T', 'N', 'L', 'P'}

const (
	headerLen = 6
	keyLen    = 32
	nonceLen  = 24
	idLen     = 16
	addrLen   = 16
)

// MaxPacket is the largest punch datagram we will parse.
const MaxPacket = 2048

// ReflexSkew is how far a reflex timestamp may be from the receiver's clock.
const ReflexSkew = 2 * time.Minute

// Is reports whether pkt is a punch frame of the current version.
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

// BuildReflexReq seals a timestamp for the wg-server's disco key.
// The device id and disco public key travel in the clear so the server can
// select the right key before it opens the box.
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

// OpenReflexReq opens a reflex request with the server's disco private key.
// The caller must check that the returned public key is the one registered
// for the device.
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

// BuildReflexResp reports the address the server observed for device.
func BuildReflexResp(device uuid.UUID, observed netip.AddrPort) []byte {
	body := make([]byte, idLen+addrLen+2)
	putID(body, device)
	putAddrPort(body[idLen:], observed)
	return frame(TypeReflexResp, body)
}

// ParseReflexResp reads a reflex response.
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

// Rendezvous is the candidate advertisement sealed to one peer.
type Rendezvous struct {
	Src, Dst   uuid.UUID
	Epoch      uint64
	WGPub      [32]byte
	WGIP       netip.Addr
	Candidates []netip.AddrPort
}

// BuildRendezvous seals r to the destination disco key.
func BuildRendezvous(srcPriv, dstPub [32]byte, r Rendezvous) ([]byte, error) {
	if len(r.Candidates) > 16 {
		return nil, fmt.Errorf("too many candidates")
	}
	plain := make([]byte, 8+keyLen+addrLen+1+(addrLen+2)*len(r.Candidates))
	binary.BigEndian.PutUint64(plain[:8], r.Epoch)
	copy(plain[8:8+keyLen], r.WGPub[:])
	putAddr(plain[8+keyLen:], r.WGIP)
	plain[8+keyLen+addrLen] = byte(len(r.Candidates))
	off := 8 + keyLen + addrLen + 1
	for _, c := range r.Candidates {
		putAddrPort(plain[off:], c)
		off += addrLen + 2
	}
	box, err := Seal(srcPriv, dstPub, plain)
	if err != nil {
		return nil, err
	}
	body := make([]byte, idLen*2+len(box))
	putID(body, r.Src)
	putID(body[idLen:], r.Dst)
	copy(body[idLen*2:], box)
	return frame(TypeRendezvous, body), nil
}

// OpenRendezvous opens a rendezvous addressed to dstPriv.
// srcPub is the sender's registered disco key, not a value read from the box.
func OpenRendezvous(dstPriv, srcPub [32]byte, pkt []byte) (Rendezvous, error) {
	var r Rendezvous
	body, err := bodyOf(pkt, TypeRendezvous)
	if err != nil {
		return r, err
	}
	if len(body) < idLen*2+nonceLen+16 {
		return r, fmt.Errorf("short rendezvous")
	}
	r.Src = getID(body)
	r.Dst = getID(body[idLen:])
	plain, err := Open(dstPriv, srcPub, body[idLen*2:])
	if err != nil {
		return r, err
	}
	need := 8 + keyLen + addrLen + 1
	if len(plain) < need {
		return r, fmt.Errorf("short rendezvous body")
	}
	r.Epoch = binary.BigEndian.Uint64(plain[:8])
	copy(r.WGPub[:], plain[8:8+keyLen])
	r.WGIP = getAddr(plain[8+keyLen:])
	n := int(plain[8+keyLen+addrLen])
	plain = plain[need:]
	if len(plain) != n*(addrLen+2) {
		return r, fmt.Errorf("bad candidate list")
	}
	r.Candidates = make([]netip.AddrPort, 0, n)
	for i := 0; i < n; i++ {
		ap := getAddrPort(plain[i*(addrLen+2):])
		if ap.IsValid() {
			r.Candidates = append(r.Candidates, ap)
		}
	}
	return r, nil
}

// BuildPing seals a probe. txid is echoed by the pong.
func BuildPing(srcPriv, dstPub [32]byte, src, dst uuid.UUID, txid uint64) ([]byte, error) {
	var plain [8]byte
	binary.BigEndian.PutUint64(plain[:], txid)
	return sealDirect(TypePing, srcPriv, dstPub, src, dst, plain[:])
}

// OpenPing opens a ping and returns its txid.
func OpenPing(dstPriv, srcPub [32]byte, pkt []byte) (src, dst uuid.UUID, txid uint64, err error) {
	plain, src, dst, err := openDirect(TypePing, dstPriv, srcPub, pkt)
	if err != nil {
		return src, dst, 0, err
	}
	if len(plain) != 8 {
		return src, dst, 0, fmt.Errorf("bad ping body")
	}
	return src, dst, binary.BigEndian.Uint64(plain), nil
}

// BuildPong seals a probe reply. observed is the address the ping arrived from.
func BuildPong(srcPriv, dstPub [32]byte, src, dst uuid.UUID, txid uint64, observed netip.AddrPort) ([]byte, error) {
	plain := make([]byte, 8+addrLen+2)
	binary.BigEndian.PutUint64(plain[:8], txid)
	putAddrPort(plain[8:], observed)
	return sealDirect(TypePong, srcPriv, dstPub, src, dst, plain)
}

// OpenPong opens a pong.
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

// BuildTeardown tells peer to drop the direct path.
func BuildTeardown(srcPriv, dstPub [32]byte, src, dst uuid.UUID) ([]byte, error) {
	return sealDirect(TypeTeardown, srcPriv, dstPub, src, dst, []byte{1})
}

// OpenTeardown opens a teardown.
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

// BuildForward wraps a client rendezvous for the peer's home wg-server.
// srcIP is the sender's tunnel address, taken from the home server's peer
// table, so the far server can apply that device's firewall policy.
func BuildForward(src, dst uuid.UUID, srcIP netip.Addr, rendezvous []byte) []byte {
	body := make([]byte, idLen*2+addrLen+len(rendezvous))
	putID(body, src)
	putID(body[idLen:], dst)
	putAddr(body[idLen*2:], srcIP)
	copy(body[idLen*2+addrLen:], rendezvous)
	return frame(TypeForward, body)
}

// ParseForward unwraps a server-to-server rendezvous.
func ParseForward(pkt []byte) (src, dst uuid.UUID, srcIP netip.Addr, rendezvous []byte, err error) {
	body, err := bodyOf(pkt, TypeForward)
	if err != nil {
		return uuid.UUID{}, uuid.UUID{}, netip.Addr{}, nil, err
	}
	if len(body) < idLen*2+addrLen+headerLen {
		return uuid.UUID{}, uuid.UUID{}, netip.Addr{}, nil, fmt.Errorf("short forward")
	}
	src = getID(body)
	dst = getID(body[idLen:])
	srcIP = getAddr(body[idLen*2:])
	rendezvous = append([]byte(nil), body[idLen*2+addrLen:]...)
	if !Is(rendezvous) || rendezvous[5] != TypeRendezvous {
		return uuid.UUID{}, uuid.UUID{}, netip.Addr{}, nil, fmt.Errorf("forward payload is not a rendezvous")
	}
	return src, dst, srcIP, rendezvous, nil
}

// TypeOf returns the punch type byte, or 0 when pkt is not a punch frame.
func TypeOf(pkt []byte) byte {
	if !Is(pkt) {
		return 0
	}
	return pkt[5]
}
