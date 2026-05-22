package wgserver

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"

	"golang.zx2c4.com/wireguard/conn"
)

// pinnedBind is a conn.Bind that listens on a specific source IP rather than
// the wildcard address. It lets multiple wg-server instances share the same
// UDP port across different public IPs on the same host.
//
// The bind is single-family: it pins either v4 or v6, not both. Constructing
// the bind with a v4 address opens only a v4 socket and rejects v6 sends.
// Single-family is sufficient for the multi-instance use case — each instance
// pins its own IP.
type pinnedBind struct {
	addr netip.Addr

	mu     sync.Mutex
	c      *net.UDPConn
	closed bool
}

func newPinnedBind(addr netip.Addr) *pinnedBind {
	return &pinnedBind{addr: addr}
}

func (b *pinnedBind) Open(port uint16) ([]conn.ReceiveFunc, uint16, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.c != nil {
		return nil, 0, conn.ErrBindAlreadyOpen
	}
	if !b.addr.IsValid() {
		return nil, 0, errors.New("pinnedBind: invalid pinned address")
	}

	network := "udp4"
	if b.addr.Is6() && !b.addr.Is4In6() {
		network = "udp6"
	}
	udpAddr := &net.UDPAddr{IP: b.addr.AsSlice(), Port: int(port)}
	c, err := net.ListenUDP(network, udpAddr)
	if err != nil {
		return nil, 0, fmt.Errorf("listen %s %s: %w", network, udpAddr, err)
	}
	b.c = c
	b.closed = false

	la := c.LocalAddr().(*net.UDPAddr)
	return []conn.ReceiveFunc{b.makeReceive(c)}, uint16(la.Port), nil
}

func (b *pinnedBind) makeReceive(c *net.UDPConn) conn.ReceiveFunc {
	return func(bufs [][]byte, sizes []int, eps []conn.Endpoint) (int, error) {
		if len(bufs) == 0 {
			return 0, errors.New("pinnedBind: empty bufs")
		}
		n, src, err := c.ReadFromUDPAddrPort(bufs[0])
		if err != nil {
			return 0, err
		}
		sizes[0] = n
		eps[0] = &conn.StdNetEndpoint{AddrPort: src}
		return 1, nil
	}
}

func (b *pinnedBind) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.c == nil {
		return nil
	}
	err := b.c.Close()
	b.c = nil
	b.closed = true
	return err
}

func (b *pinnedBind) SetMark(mark uint32) error {
	// SO_MARK is a Linux-specific routing hint; not needed for our use case
	// since SNAT pins the source via iptables.
	return nil
}

func (b *pinnedBind) Send(bufs [][]byte, ep conn.Endpoint) error {
	stdEP, ok := ep.(*conn.StdNetEndpoint)
	if !ok {
		return conn.ErrWrongEndpointType
	}

	b.mu.Lock()
	c := b.c
	b.mu.Unlock()
	if c == nil {
		return net.ErrClosed
	}

	dst := stdEP.AddrPort
	dstIsV6 := dst.Addr().Is6() && !dst.Addr().Is4In6()
	pinnedIsV6 := b.addr.Is6() && !b.addr.Is4In6()
	if dstIsV6 != pinnedIsV6 {
		return fmt.Errorf("pinnedBind: address family mismatch (pinned=%v dst=%v)", b.addr, dst.Addr())
	}

	udpDst := net.UDPAddrFromAddrPort(dst)
	for _, buf := range bufs {
		if _, err := c.WriteToUDP(buf, udpDst); err != nil {
			return err
		}
	}
	return nil
}

func (b *pinnedBind) ParseEndpoint(s string) (conn.Endpoint, error) {
	ap, err := netip.ParseAddrPort(s)
	if err != nil {
		return nil, err
	}
	return &conn.StdNetEndpoint{AddrPort: ap}, nil
}

func (b *pinnedBind) BatchSize() int { return 1 }
