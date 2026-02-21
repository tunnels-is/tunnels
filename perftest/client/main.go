package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

var (
	serverAddr = flag.String("server", "", "server address ip:port (required)")
	tunIP      = flag.String("tun-ip", "172.22.22.1", "TUN interface IP")
	serverIP   = flag.String("server-ip", "", "server public IP for NAT rewrite (required)")
	route      = flag.String("route", "", "route to add via TUN, e.g. 109.61.92.0/24")
	mtu        = flag.Int("mtu", 1420, "TUN MTU")
)

// Port mapping
type portMapping struct {
	mappedPort    uint16
	originalPort  uint16
	originalSrcIP [4]byte
}

var (
	egressMap     = make(map[uint32]*portMapping)
	ingressMap    = make(map[uint16]*portMapping)
	natMu         sync.Mutex
	nextPort      = uint16(2000)
	serverIPBytes [4]byte
	tunIPBytes    [4]byte
)

// Stats
var (
	tunReadPkts  atomic.Int64
	tunReadBytes atomic.Int64
	tunWritePkts atomic.Int64
	tunWriteBytes atomic.Int64
	udpReadPkts  atomic.Int64
	udpReadBytes atomic.Int64
	udpWritePkts atomic.Int64
	udpWriteBytes atomic.Int64
	natMisses    atomic.Int64
)

func main() {
	flag.Parse()
	if *serverAddr == "" || *serverIP == "" {
		fmt.Fprintln(os.Stderr, "usage: perftest-client -server <ip:port> -server-ip <ip> [-tun-ip <ip>] [-route <cidr>]")
		os.Exit(1)
	}

	sip := net.ParseIP(*serverIP).To4()
	if sip == nil {
		log.Fatal("invalid server-ip")
	}
	copy(serverIPBytes[:], sip)

	tip := net.ParseIP(*tunIP).To4()
	if tip == nil {
		log.Fatal("invalid tun-ip")
	}
	copy(tunIPBytes[:], tip)

	tuneSysctls()

	// Create TUN using os.File for proper Go poller integration
	tunFile, tunName, err := createTUN("perftest0")
	if err != nil {
		log.Fatal("create tun:", err)
	}
	log.Println("TUN created:", tunName)

	configureTUN(tunName)

	if *route != "" {
		addRoute(*route, *tunIP)
	}

	// Connect UDP to server
	raddr, err := net.ResolveUDPAddr("udp4", *serverAddr)
	if err != nil {
		log.Fatal("resolve server:", err)
	}
	udpConn, err := net.DialUDP("udp4", nil, raddr)
	if err != nil {
		log.Fatal("dial udp:", err)
	}
	udpConn.SetReadBuffer(8 * 1024 * 1024)
	udpConn.SetWriteBuffer(8 * 1024 * 1024)

	log.Printf("connected to server %s", *serverAddr)
	log.Println("ready — run: wget https://dal.download.datapacket.com/1000mb.bin")

	go statsLogger()
	go udpToTun(udpConn, tunFile)
	tunToUDP(tunFile, udpConn)
}

func statsLogger() {
	for {
		time.Sleep(2 * time.Second)
		tr := tunReadPkts.Swap(0)
		trb := tunReadBytes.Swap(0)
		tw := tunWritePkts.Swap(0)
		twb := tunWriteBytes.Swap(0)
		ur := udpReadPkts.Swap(0)
		urb := udpReadBytes.Swap(0)
		uw := udpWritePkts.Swap(0)
		uwb := udpWriteBytes.Swap(0)
		nm := natMisses.Swap(0)
		if tr+tw+ur+uw > 0 {
			log.Printf("STATS/2s: tunRd=%d(%dKB) tunWr=%d(%dKB) udpRd=%d(%dKB) udpWr=%d(%dKB) natMiss=%d",
				tr, trb/1024, tw, twb/1024, ur, urb/1024, uw, uwb/1024, nm)
		}
	}
}

func tuneSysctls() {
	sysctls := map[string]string{
		"/proc/sys/net/core/rmem_max":                  "26214400",
		"/proc/sys/net/core/wmem_max":                  "26214400",
		"/proc/sys/net/ipv4/tcp_congestion_control":    "bbr",
		"/proc/sys/net/core/default_qdisc":             "fq",
		"/proc/sys/net/ipv4/tcp_slow_start_after_idle": "0",
		"/proc/sys/net/ipv4/tcp_mtu_probing":           "1",
	}
	for path, val := range sysctls {
		os.WriteFile(path, []byte(val), 0644)
	}
}

// createTUN using os.File for proper Go netpoller integration
func createTUN(name string) (tunFile *os.File, ifName string, err error) {
	fd, err := unix.Open("/dev/net/tun", unix.O_RDWR|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, "", fmt.Errorf("open /dev/net/tun: %w", err)
	}

	type ifReq struct {
		Name  [16]byte
		Flags uint16
		_     [22]byte
	}

	var req ifReq
	copy(req.Name[:], name)
	req.Flags = unix.IFF_TUN | unix.IFF_NO_PI

	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), unix.TUNSETIFF, uintptr(unsafe.Pointer(&req)))
	if errno != 0 {
		unix.Close(fd)
		return nil, "", fmt.Errorf("ioctl TUNSETIFF: %v", errno)
	}

	ifName = string(req.Name[:])
	for i, b := range ifName {
		if b == 0 {
			ifName = ifName[:i]
			break
		}
	}

	// Wrap in os.File so Go's netpoller (epoll) manages it
	tunFile = os.NewFile(uintptr(fd), "/dev/net/tun")
	return tunFile, ifName, nil
}

func configureTUN(name string) {
	run("ip", "addr", "add", *tunIP+"/32", "dev", name)
	run("ip", "link", "set", name, "up")
	run("ip", "link", "set", name, "mtu", fmt.Sprintf("%d", *mtu))
	run("ip", "link", "set", name, "txqueuelen", "2000")
}

func addRoute(cidr, gw string) {
	err := exec.Command("ip", "route", "add", cidr, "via", gw).Run()
	if err != nil {
		log.Printf("add route %s via %s: %v (may already exist)", cidr, gw, err)
	} else {
		log.Printf("route added: %s via %s", cidr, gw)
	}
}

func run(args ...string) {
	if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
		log.Printf("cmd %v: %v", args, err)
	}
}

// tunToUDP: read from TUN, NAT rewrite, send over UDP
func tunToUDP(tunFile *os.File, udpConn *net.UDPConn) {
	buf := make([]byte, 65536)
	for {
		n, err := tunFile.Read(buf)
		if err != nil {
			log.Fatal("tun read:", err)
		}
		if n < 20 {
			continue
		}

		pkt := buf[:n]
		if pkt[0]>>4 != 4 {
			continue
		}

		proto := pkt[9]
		if proto != 6 && proto != 17 {
			continue
		}

		ihl := (pkt[0] & 0x0F) * 4
		if int(ihl)+4 > n {
			continue
		}

		tunReadPkts.Add(1)
		tunReadBytes.Add(int64(n))

		tpHeader := pkt[ihl:]
		srcPort := binary.BigEndian.Uint16(tpHeader[0:2])

		key := uint32(srcPort)
		natMu.Lock()
		pm, exists := egressMap[key]
		if !exists {
			pm = &portMapping{
				mappedPort:   nextPort,
				originalPort: srcPort,
			}
			copy(pm.originalSrcIP[:], pkt[12:16])
			egressMap[key] = pm
			ingressMap[nextPort] = pm
			nextPort++
			if nextPort > 65000 {
				nextPort = 2000
			}
		}
		natMu.Unlock()

		// NAT rewrite
		pkt[12] = serverIPBytes[0]
		pkt[13] = serverIPBytes[1]
		pkt[14] = serverIPBytes[2]
		pkt[15] = serverIPBytes[3]
		binary.BigEndian.PutUint16(tpHeader[0:2], pm.mappedPort)

		recalcIPChecksum(pkt[:ihl])
		recalcTransportChecksum(pkt[:ihl], tpHeader)

		_, err = udpConn.Write(pkt)
		if err != nil {
			log.Println("udp write:", err)
		}
		udpWritePkts.Add(1)
		udpWriteBytes.Add(int64(n))
	}
}

// udpToTun: read from UDP, reverse NAT, write to TUN
func udpToTun(udpConn *net.UDPConn, tunFile *os.File) {
	buf := make([]byte, 65536)
	for {
		n, err := udpConn.Read(buf)
		if err != nil {
			log.Fatal("udp read:", err)
		}
		if n < 20 {
			continue
		}

		pkt := buf[:n]
		if pkt[0]>>4 != 4 {
			continue
		}

		ihl := (pkt[0] & 0x0F) * 4
		if int(ihl)+4 > n {
			continue
		}

		udpReadPkts.Add(1)
		udpReadBytes.Add(int64(n))

		tpHeader := pkt[ihl:]
		dstPort := binary.BigEndian.Uint16(tpHeader[2:4])

		natMu.Lock()
		pm, exists := ingressMap[dstPort]
		natMu.Unlock()
		if !exists {
			natMisses.Add(1)
			continue
		}

		// Reverse NAT
		pkt[16] = tunIPBytes[0]
		pkt[17] = tunIPBytes[1]
		pkt[18] = tunIPBytes[2]
		pkt[19] = tunIPBytes[3]
		binary.BigEndian.PutUint16(tpHeader[2:4], pm.originalPort)

		recalcIPChecksum(pkt[:ihl])
		recalcTransportChecksum(pkt[:ihl], tpHeader)

		_, err = tunFile.Write(pkt)
		if err != nil {
			log.Println("tun write:", err)
		}
		tunWritePkts.Add(1)
		tunWriteBytes.Add(int64(n))
	}
}

func recalcIPChecksum(hdr []byte) {
	hdr[10] = 0
	hdr[11] = 0
	var csum uint32
	for i := 0; i < len(hdr)-1; i += 2 {
		csum += uint32(hdr[i])<<8 | uint32(hdr[i+1])
	}
	for csum > 0xFFFF {
		csum = (csum >> 16) + (csum & 0xFFFF)
	}
	hdr[10] = byte(^csum >> 8)
	hdr[11] = byte(^csum & 0xFF)
}

func recalcTransportChecksum(ipHdr []byte, tpPkt []byte) {
	proto := ipHdr[9]
	switch proto {
	case 6:
		tpPkt[16] = 0
		tpPkt[17] = 0
	case 17:
		tpPkt[6] = 0
		tpPkt[7] = 0
	}

	var csum uint32
	csum += (uint32(ipHdr[12]) + uint32(ipHdr[14])) << 8
	csum += uint32(ipHdr[13]) + uint32(ipHdr[15])
	csum += (uint32(ipHdr[16]) + uint32(ipHdr[18])) << 8
	csum += uint32(ipHdr[17]) + uint32(ipHdr[19])
	csum += uint32(proto)
	tcpLen := uint32(len(tpPkt))
	csum += tcpLen & 0xffff
	csum += tcpLen >> 16

	length := len(tpPkt) - 1
	for i := 0; i < length; i += 2 {
		csum += uint32(tpPkt[i]) << 8
		csum += uint32(tpPkt[i+1])
	}
	if len(tpPkt)%2 == 1 {
		csum += uint32(tpPkt[length]) << 8
	}
	for csum > 0xffff {
		csum = (csum >> 16) + (csum & 0xffff)
	}

	switch proto {
	case 6:
		binary.BigEndian.PutUint16(tpPkt[16:18], ^uint16(csum))
	case 17:
		binary.BigEndian.PutUint16(tpPkt[6:8], ^uint16(csum))
	}
}
