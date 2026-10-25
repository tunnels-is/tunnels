package client

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/tunnels-is/tunnels/punch"
)

const (
	serverMTUProbeWait = 450 * time.Millisecond
	serverMTUCacheTTL  = 10 * time.Minute
	// mssFloor is the smallest TCP MSS a clamp will advertise.
	mssFloor = 536
)

type mtuCacheEntry struct {
	at  time.Time
	mtu int
}

var (
	mtuCacheMu sync.Mutex
	mtuCache   = map[string]mtuCacheEntry{}
)

// resolvedMTU returns the tunnel interface MTU. A configured value is kept.
// Zero discovers the server path and, if that fails, falls back to 1420.
// The discovered value is not written back onto meta.
func resolvedMTU(meta *TunnelMeta, serverIP, port string) int32 {
	if meta != nil && meta.MTU > 0 {
		return meta.MTU
	}
	if serverIP == "" || port == "" {
		INFO("mtu: no server endpoint; using ", punch.DefaultInnerMTU)
		return punch.DefaultInnerMTU
	}
	mtu, err := discoverServerMTU(serverIP, port)
	if err != nil || mtu < punch.MinInnerMTU {
		if err != nil {
			INFO("mtu: discovery failed for ", serverIP, ":", port, " (", err, "); using ", punch.DefaultInnerMTU)
		} else {
			INFO("mtu: discovery for ", serverIP, ":", port, " returned ", mtu, "; using ", punch.DefaultInnerMTU)
		}
		return punch.DefaultInnerMTU
	}
	INFO("mtu: ", serverIP, ":", port, " inner ", mtu)
	return int32(mtu)
}

func discoverServerMTU(serverIP, port string) (int, error) {
	key := net.JoinHostPort(serverIP, port)
	if m, ok := cachedServerMTU(key); ok {
		return m, nil
	}
	raddr, err := net.ResolveUDPAddr("udp", key)
	if err != nil {
		return 0, err
	}
	inner, err := discoverPathMTU(raddr)
	if err != nil {
		return 0, err
	}
	storeServerMTU(key, inner)
	return inner, nil
}

func cachedServerMTU(key string) (int, bool) {
	mtuCacheMu.Lock()
	defer mtuCacheMu.Unlock()
	e, ok := mtuCache[key]
	if !ok || time.Since(e.at) > serverMTUCacheTTL {
		return 0, false
	}
	return e.mtu, true
}

func storeServerMTU(key string, mtu int) {
	mtuCacheMu.Lock()
	mtuCache[key] = mtuCacheEntry{at: time.Now(), mtu: mtu}
	mtuCacheMu.Unlock()
}

// discoverPathMTU probes raddr with DF set and returns the inner MTU that fits.
func discoverPathMTU(raddr *net.UDPAddr) (int, error) {
	if raddr == nil || raddr.IP == nil {
		return 0, errors.New("no address")
	}
	v6 := raddr.IP.To4() == nil
	network := "udp4"
	if v6 {
		network = "udp6"
	}
	conn, err := net.DialUDP(network, nil, raddr)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	restore := setUDPDontFrag(conn, v6)
	defer restore()

	hi := punch.MaxOuterMTU
	if m := egressMTU(raddr.IP); m >= punch.MinOuter(v6) && m < hi {
		hi = m
	}
	lo := punch.MinOuter(v6)
	if hi < lo {
		return 0, fmt.Errorf("interface mtu %d below minimum", hi)
	}
	outer, ok, err := searchOuter(conn, v6, lo, hi)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, errors.New("no mtu ack")
	}
	return outer - punch.Overhead(v6), nil
}

func searchOuter(conn *net.UDPConn, v6 bool, lo, hi int) (int, bool, error) {
	fit := 0
	probe := hi
	retried1500 := false
	for steps := 0; steps < 16; steps++ {
		payload := punch.ProbePayloadLen(probe, v6)
		fits, localTooBig, err := probeOnce(conn, payload)
		if err != nil {
			return 0, false, err
		}
		if fits {
			fit = probe
			lo = probe + 1
		} else {
			if probe == punch.MaxOuterMTU && !retried1500 && !localTooBig {
				retried1500 = true
				continue
			}
			hi = probe - 1
		}
		if lo > hi {
			break
		}
		next := (lo + hi) / 2
		if next == probe {
			break
		}
		probe = next
	}
	if fit == 0 {
		return 0, false, nil
	}
	return fit, true, nil
}

func probeOnce(conn *net.UDPConn, payload int) (fits, localTooBig bool, err error) {
	pkt, err := punch.BuildMTUProbe(payload)
	if err != nil {
		return false, false, err
	}
	if err = conn.SetReadDeadline(time.Now().Add(serverMTUProbeWait)); err != nil {
		return false, false, err
	}
	if _, err = conn.Write(pkt); err != nil {
		if isMsgTooBig(err) {
			return false, true, nil
		}
		return false, false, err
	}
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			return false, false, nil
		}
		return false, false, err
	}
	return punch.IsMTUAck(buf[:n]), false, nil
}

// LinkMTU is the live interface MTU, or 0 when the adapter is not up.
func (t *TUN) LinkMTU() int32 {
	if t == nil {
		return 0
	}
	a := t.tunnel.Load()
	if a == nil || a.MTU <= 0 {
		return 0
	}
	return a.MTU
}

func (t *TUN) peerPathMTU(ip [4]byte) int {
	if t == nil {
		return 0
	}
	t.punchMu.Lock()
	defer t.punchMu.Unlock()
	if t.punchMTU == nil {
		return 0
	}
	return t.punchMTU[ip]
}

// notePeerMTU records a direct peer whose path is smaller than the interface.
// The interface MTU stays at the server-path measurement. A tighter peer gets
// a per-destination route where the OS supports one, plus an MSS clamp.
func (t *TUN) notePeerMTU(p punch.PathMTU) {
	if t == nil || !p.Peer.WGIP.Is4() || p.Inner < punch.MinInnerMTU {
		return
	}
	ip := p.Peer.WGIP.As4()
	ifaceMTU := 0
	ifName := ""
	if a := t.tunnel.Load(); a != nil {
		ifaceMTU = int(a.MTU)
		ifName = a.Name
	}
	if ifaceMTU > 0 && p.Inner >= ifaceMTU {
		t.clearPeerMTU(ip)
		return
	}
	t.punchMu.Lock()
	if t.punchMTU == nil {
		t.punchMTU = map[[4]byte]int{}
	}
	prev, seen := t.punchMTU[ip]
	t.punchMTU[ip] = p.Inner
	t.punchMu.Unlock()
	if seen && prev == p.Inner {
		return
	}
	if ifName != "" {
		if err := addPeerMTURoute(ifName, netip.AddrFrom4(ip).String(), p.Inner); err != nil {
			DEBUG("mtu: peer route ", netip.AddrFrom4(ip).String(), ": ", err)
		}
	}
	INFO("mtu: peer ", p.Peer.WGIP.String(), " inner ", p.Inner)
}

func (t *TUN) clearPeerMTU(ip [4]byte) {
	if t == nil {
		return
	}
	t.punchMu.Lock()
	_, ok := t.punchMTU[ip]
	if ok {
		delete(t.punchMTU, ip)
	}
	if t.punchMTULogged != nil {
		delete(t.punchMTULogged, ip)
	}
	ifName := ""
	if a := t.tunnel.Load(); a != nil {
		ifName = a.Name
	}
	t.punchMu.Unlock()
	if ok && ifName != "" {
		_ = delPeerMTURoute(ifName, netip.AddrFrom4(ip).String())
	}
}

func (t *TUN) logPeerMTUDrop(ip [4]byte, mtu int) {
	t.punchMu.Lock()
	if t.punchMTULogged == nil {
		t.punchMTULogged = map[[4]byte]struct{}{}
	}
	_, seen := t.punchMTULogged[ip]
	if !seen {
		t.punchMTULogged[ip] = struct{}{}
	}
	t.punchMu.Unlock()
	if !seen {
		INFO("mtu: dropping UDP to ", net.IP(ip[:]).String(), " larger than ", mtu)
	}
}

// fitPeerMTU clamps a TCP SYN MSS to a tighter peer, and drops UDP that would
// leave as an outer datagram bigger than that peer. TCP data is left alone:
// the route MTU is what keeps those segments in range.
func (t *TUN) fitPeerMTU(packet []byte, dst [4]byte, fragmented bool) bool {
	if t == nil || fragmented {
		return true
	}
	mtu := t.peerPathMTU(dst)
	if mtu <= 0 {
		return true
	}
	switch t.EP_Protocol {
	case 17:
		total := len(packet)
		if len(packet) >= 4 {
			n := int(binary.BigEndian.Uint16(packet[2:4]))
			if n >= 20 && n <= len(packet) {
				total = n
			}
		}
		if total > mtu {
			t.logPeerMTUDrop(dst, mtu)
			return false
		}
	case 6:
		clampTCPv4MSS(t.EP_IPv4Header, t.EP_TPHeader, mtu)
	}
	return true
}

func clampTCPv4MSS(ipHdr, tcp []byte, mtu int) {
	if len(tcp) < 20 || ipHdr == nil {
		return
	}
	if tcp[13]&0x02 == 0 {
		return
	}
	hdrLen := int(tcp[12]>>4) * 4
	if hdrLen < 24 || hdrLen > len(tcp) {
		return
	}
	limit := mtu - 40
	if limit < mssFloor {
		limit = mssFloor
	}
	opts := tcp[20:hdrLen]
	changed := false
	for i := 0; i < len(opts); {
		kind := opts[i]
		if kind == 0 {
			break
		}
		if kind == 1 {
			i++
			continue
		}
		if i+1 >= len(opts) {
			break
		}
		optLen := int(opts[i+1])
		if optLen < 2 || i+optLen > len(opts) {
			break
		}
		if kind == 2 && optLen == 4 {
			mss := int(binary.BigEndian.Uint16(opts[i+2 : i+4]))
			if mss > limit {
				binary.BigEndian.PutUint16(opts[i+2:i+4], uint16(limit))
				changed = true
			}
		}
		i += optLen
	}
	if changed {
		RecalculateTransportChecksum(ipHdr, tcp)
	}
}
