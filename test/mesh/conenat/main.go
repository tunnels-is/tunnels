//go:build linux

// Command conenat is the full-cone UDP NAT used by the mesh punch test.
// Linux conntrack MASQUERADE is symmetric: it only accepts replies from the
// exact address a socket just talked to, so two clients cannot punch through
// it. This process port-preserves each LAN UDP source on the public address
// and accepts packets from any remote. TCP is left to conntrack MASQUERADE
// so clients can still reach the controller over HTTPS.
package main

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func main() {
	pub := flag.String("pub", "", "public IPv4 address")
	lan := flag.String("lan", "", "LAN IPv4 address")
	listen := flag.String("listen", "127.0.0.1:4500", "TPROXY listen address")
	flag.Parse()
	if *pub == "" || *lan == "" {
		fmt.Fprintln(os.Stderr, "conenat: -pub and -lan are required")
		os.Exit(2)
	}
	pubIP := net.ParseIP(*pub).To4()
	lanIP := net.ParseIP(*lan).To4()
	if pubIP == nil || lanIP == nil {
		fmt.Fprintln(os.Stderr, "conenat: -pub and -lan must be IPv4")
		os.Exit(2)
	}
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	var pubIface, lanIface string
	var err error
	deadline := time.Now().Add(30 * time.Second)
	for {
		pubIface, err = ifaceByIP(pubIP)
		if err == nil {
			lanIface, err = ifaceByIP(lanIP)
		}
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			log.Fatalf("waiting for interfaces: %v", err)
		}
		time.Sleep(200 * time.Millisecond)
	}
	log.Printf("pub %s on %s, lan %s on %s", pubIP, pubIface, lanIP, lanIface)

	relaxReversePath()
	// Docker mounts a --sysctl key read-only, so ip_forward is already 1 and
	// a write fails. Accept the value when it is already what we need.
	if err := ensureProc("/proc/sys/net/ipv4/ip_forward", "1"); err != nil {
		log.Fatalf("ip_forward: %v", err)
	}
	// all.route_localnet is OR'd onto every interface, including the LAN
	// interface attached after the container starts. Docker mounts these
	// sysctls read-only, so the test sets them with --sysctl.
	if err := ensureProc("/proc/sys/net/ipv4/conf/all/route_localnet", "1"); err != nil {
		log.Fatalf("route_localnet: %v", err)
	}

	if err := run("ip", "rule", "add", "fwmark", "1", "lookup", "100"); err != nil {
		log.Fatalf("ip rule: %v", err)
	}
	if err := run("ip", "route", "add", "local", "0.0.0.0/0", "dev", "lo", "table", "100"); err != nil {
		log.Fatalf("ip route local: %v", err)
	}
	// Conntrack remembers the first TPROXY redirect and then swallows later
	// packets of the same flow, so a handshake gets through and keepalives
	// do not. UDP must stay untracked for the full-cone relay.
	for _, args := range [][]string{
		{"iptables", "-t", "raw", "-A", "PREROUTING", "-i", lanIface, "-p", "udp", "-j", "CT", "--notrack"},
		{"iptables", "-t", "raw", "-A", "PREROUTING", "-i", pubIface, "-p", "udp", "-j", "CT", "--notrack"},
		{"iptables", "-t", "raw", "-A", "OUTPUT", "-p", "udp", "-j", "CT", "--notrack"},
	} {
		if err := run(args...); err != nil {
			log.Fatalf("notrack: %v", err)
		}
	}
	if err := run("iptables", "-t", "mangle", "-A", "PREROUTING", "-i", lanIface, "-p", "udp",
		"-j", "TPROXY", "--on-ip", "127.0.0.1", "--on-port", portOf(*listen), "--tproxy-mark", "1"); err != nil {
		log.Fatalf("tproxy rule: %v", err)
	}
	if err := run("iptables", "-t", "nat", "-A", "POSTROUTING", "-o", pubIface, "-p", "tcp", "-j", "MASQUERADE"); err != nil {
		log.Fatalf("tcp masquerade: %v", err)
	}
	_ = run("iptables", "-P", "FORWARD", "ACCEPT")

	ln, err := listenTransparent(*listen)
	if err != nil {
		log.Fatalf("listen %s: %v", *listen, err)
	}
	defer ln.Close()
	log.Printf("conenat ready pub=%s lan=%s", pubIP, lanIP)

	n := &nat{pub: pubIP, flows: map[netip.AddrPort]*flow{}}
	buf := make([]byte, 65535)
	oob := make([]byte, 512)
	for {
		nn, oobn, _, src, err := ln.ReadMsgUDP(buf, oob)
		if err != nil {
			log.Fatalf("read: %v", err)
		}
		client, ok := udpFrom(src)
		if !ok {
			continue
		}
		dest, ok := parseOrigDst(oob[:oobn])
		if !ok {
			log.Printf("drop %s: no original destination", client)
			continue
		}
		payload := append([]byte(nil), buf[:nn]...)
		n.forward(client, dest, payload)
	}
}

type nat struct {
	pub   net.IP
	flows map[netip.AddrPort]*flow
}

type flow struct {
	conn *net.UDPConn
}

func (n *nat) forward(client, dest netip.AddrPort, payload []byte) {
	f := n.mapping(client)
	if f == nil {
		return
	}
	if _, err := f.conn.WriteToUDP(payload, udpAddr(dest)); err != nil {
		log.Printf("forward %s -> %s: %v", client, dest, err)
	}
}

func (n *nat) mapping(client netip.AddrPort) *flow {
	if f := n.flows[client]; f != nil {
		return f
	}
	if client.Port() == 0 {
		return nil
	}
	// A normal socket, not a transparent one. A transparent socket bound to
	// this port is eligible for every locally routed UDP packet once the
	// TPROXY mark makes 0.0.0.0/0 local, so it would swallow the client's
	// later packets and the handshake would never be followed by data.
	lc := net.ListenConfig{Control: func(_, _ string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			_ = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_RCVBUF, 4<<20)
		})
	}}
	pc, err := lc.ListenPacket(context.Background(), "udp4", net.JoinHostPort(n.pub.String(), fmt.Sprint(client.Port())))
	if err != nil {
		log.Printf("bind public %s:%d for %s: %v", n.pub, client.Port(), client, err)
		return nil
	}
	uc := pc.(*net.UDPConn)
	f := &flow{conn: uc}
	n.flows[client] = f
	log.Printf("map %s -> %s:%d", client, n.pub, client.Port())
	go n.receive(client, uc)
	return f
}

func (n *nat) receive(client netip.AddrPort, uc *net.UDPConn) {
	buf := make([]byte, 65535)
	for {
		nn, remote, err := uc.ReadFromUDP(buf)
		if err != nil {
			log.Printf("public read %s: %v", client, err)
			return
		}
		if remote == nil {
			continue
		}
		payload := append([]byte(nil), buf[:nn]...)
		from, ok := udpFrom(remote)
		if !ok {
			continue
		}
		if err := spoof(udpAddr(client), from, payload); err != nil {
			log.Printf("spoof %s -> %s: %v", from, client, err)
		}
	}
}

var (
	rawOnce sync.Once
	rawFD   int
	rawErr  error
	rawMu   sync.Mutex
)

func rawSocket() (int, error) {
	rawOnce.Do(func() {
		fd, err := unix.Socket(unix.AF_INET, unix.SOCK_RAW, unix.IPPROTO_UDP)
		if err != nil {
			rawErr = err
			return
		}
		if err := unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_HDRINCL, 1); err != nil {
			unix.Close(fd)
			rawErr = err
			return
		}
		// IP_TRANSPARENT lets the header carry the server's source address.
		// A raw socket is not in the UDP demux, so it does not steal the
		// client's packets from the TPROXY listener.
		if err := unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_TRANSPARENT, 1); err != nil {
			unix.Close(fd)
			rawErr = err
			return
		}
		rawFD = fd
	})
	return rawFD, rawErr
}

func spoof(to *net.UDPAddr, from netip.AddrPort, payload []byte) error {
	fd, err := rawSocket()
	if err != nil {
		return err
	}
	pkt := buildIPv4UDP(from, to, payload)
	addr := &unix.SockaddrInet4{}
	copy(addr.Addr[:], to.IP.To4())
	rawMu.Lock()
	err = unix.Sendto(fd, pkt, 0, addr)
	rawMu.Unlock()
	return err
}

func buildIPv4UDP(src netip.AddrPort, dst *net.UDPAddr, payload []byte) []byte {
	n := 20 + 8 + len(payload)
	b := make([]byte, n)
	b[0] = 0x45
	binary.BigEndian.PutUint16(b[2:4], uint16(n))
	b[8] = 64
	b[9] = unix.IPPROTO_UDP
	copy(b[12:16], src.Addr().AsSlice())
	copy(b[16:20], dst.IP.To4())
	binary.BigEndian.PutUint16(b[20:22], src.Port())
	binary.BigEndian.PutUint16(b[22:24], uint16(dst.Port))
	binary.BigEndian.PutUint16(b[24:26], uint16(8+len(payload)))
	copy(b[28:], payload)
	return b
}

func listenTransparent(addr string) (*net.UDPConn, error) {
	lc := net.ListenConfig{Control: func(_, _ string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			if err := unix.SetsockoptInt(int(fd), unix.SOL_IP, unix.IP_TRANSPARENT, 1); err != nil {
				log.Printf("IP_TRANSPARENT: %v", err)
			}
			if err := unix.SetsockoptInt(int(fd), unix.SOL_IP, unix.IP_RECVORIGDSTADDR, 1); err != nil {
				log.Printf("IP_RECVORIGDSTADDR: %v", err)
			}
			_ = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_RCVBUF, 4<<20)
		})
	}}
	pc, err := lc.ListenPacket(context.Background(), "udp4", addr)
	if err != nil {
		return nil, err
	}
	uc, ok := pc.(*net.UDPConn)
	if !ok {
		pc.Close()
		return nil, fmt.Errorf("listen returned %T", pc)
	}
	return uc, nil
}

func parseOrigDst(oob []byte) (netip.AddrPort, bool) {
	msgs, err := syscall.ParseSocketControlMessage(oob)
	if err != nil {
		return netip.AddrPort{}, false
	}
	for _, m := range msgs {
		if m.Header.Level != syscall.SOL_IP || (m.Header.Type != unix.IP_ORIGDSTADDR && m.Header.Type != unix.IP_RECVORIGDSTADDR) {
			continue
		}
		if len(m.Data) < 8 {
			continue
		}
		port := uint16(m.Data[2])<<8 | uint16(m.Data[3])
		ip := net.IPv4(m.Data[4], m.Data[5], m.Data[6], m.Data[7]).To4()
		addr, ok := netip.AddrFromSlice(ip)
		if !ok || port == 0 {
			continue
		}
		return netip.AddrPortFrom(addr, port), true
	}
	return netip.AddrPort{}, false
}

func ifaceByIP(ip net.IP) (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, ifi := range ifaces {
		addrs, err := ifi.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if ok && ipn.IP.To4() != nil && ipn.IP.To4().Equal(ip) {
				return ifi.Name, nil
			}
		}
	}
	return "", fmt.Errorf("no interface has %s", ip)
}

func relaxReversePath() {
	ents, _ := os.ReadDir("/proc/sys/net/ipv4/conf")
	for _, e := range ents {
		_ = ensureProc("/proc/sys/net/ipv4/conf/"+e.Name()+"/rp_filter", "0")
	}
}

func ensureProc(path, value string) error {
	b, err := os.ReadFile(path)
	if err == nil && strings.TrimSpace(string(b)) == value {
		return nil
	}
	if werr := os.WriteFile(path, []byte(value), 0o644); werr != nil {
		if err == nil {
			return fmt.Errorf("%s is %q and not writable: %w", path, strings.TrimSpace(string(b)), werr)
		}
		return werr
	}
	return nil
}

func run(args ...string) error {
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %v: %s", args, err, out)
	}
	return nil
}

func portOf(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "4500"
	}
	return port
}

func udpFrom(a *net.UDPAddr) (netip.AddrPort, bool) {
	if a == nil {
		return netip.AddrPort{}, false
	}
	ip := a.IP.To4()
	if ip == nil {
		return netip.AddrPort{}, false
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return netip.AddrPort{}, false
	}
	return netip.AddrPortFrom(addr, uint16(a.Port)), true
}

func udpAddr(a netip.AddrPort) *net.UDPAddr {
	return &net.UDPAddr{IP: net.IP(a.Addr().AsSlice()), Port: int(a.Port())}
}
