package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	ifaceIP   = flag.String("ip", "", "server public interface IP (required)")
	udpPort   = flag.Int("port", 444, "UDP listen port for tunnel data")
	startPort = flag.Uint("start-port", 2000, "start of client port range")
)

// Stats
var (
	udpInPkts   atomic.Int64
	udpInBytes  atomic.Int64
	udpOutPkts  atomic.Int64
	udpOutBytes atomic.Int64
	rawInPkts   atomic.Int64
	rawSkipped  atomic.Int64
	rawSendErr  atomic.Int64
	rawOversized atomic.Int64
)

var clientAddr atomic.Pointer[net.UDPAddr]

func main() {
	flag.Parse()
	if *ifaceIP == "" {
		fmt.Fprintln(os.Stderr, "usage: perftest-server -ip <server-public-ip>")
		os.Exit(1)
	}

	ip := net.ParseIP(*ifaceIP).To4()
	if ip == nil {
		log.Fatal("invalid IP")
	}

	// UDP socket for tunnel data
	laddr := &net.UDPAddr{Port: *udpPort}
	udpConn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		log.Fatal("udp listen:", err)
	}
	udpConn.SetReadBuffer(8 * 1024 * 1024)
	udpConn.SetWriteBuffer(8 * 1024 * 1024)
	log.Printf("UDP tunnel listening on :%d", *udpPort)

	// Raw TCP socket for reading return traffic
	rawTCPfd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_TCP)
	if err != nil {
		log.Fatal("raw tcp socket:", err)
	}
	err = syscall.Bind(rawTCPfd, &syscall.SockaddrInet4{Addr: [4]byte{ip[0], ip[1], ip[2], ip[3]}})
	if err != nil {
		log.Fatal("raw tcp bind:", err)
	}
	syscall.SetsockoptInt(rawTCPfd, syscall.SOL_SOCKET, syscall.SO_RCVBUF, 8*1024*1024)

	// Raw socket for writing outbound packets
	rawSendFd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_RAW)
	if err != nil {
		log.Fatal("raw send socket:", err)
	}
	syscall.SetsockoptInt(rawSendFd, syscall.SOL_SOCKET, syscall.SO_SNDBUF, 8*1024*1024)

	log.Println("server ready, waiting for client...")

	go statsLogger()
	go internetToUDP(rawTCPfd, udpConn)
	udpToInternet(udpConn, rawSendFd)
}

func statsLogger() {
	for {
		time.Sleep(2 * time.Second)
		ui := udpInPkts.Swap(0)
		uib := udpInBytes.Swap(0)
		uo := udpOutPkts.Swap(0)
		uob := udpOutBytes.Swap(0)
		ri := rawInPkts.Swap(0)
		rs := rawSkipped.Swap(0)
		re := rawSendErr.Swap(0)
		ro := rawOversized.Swap(0)
		if ui+uo+ri > 0 {
			log.Printf("STATS/2s: udpIn=%d(%dKB) udpOut=%d(%dKB) rawIn=%d rawSkip=%d rawSendErr=%d rawOversized=%d",
				ui, uib/1024, uo, uob/1024, ri, rs, re, ro)
		}
	}
}

// udpToInternet: receive packets from client via UDP, forward to internet via raw socket
func udpToInternet(udpConn *net.UDPConn, rawSendFd int) {
	buf := make([]byte, 65536)
	for {
		n, addr, err := udpConn.ReadFromUDP(buf)
		if err != nil {
			log.Fatal("udp read:", err)
		}
		if n < 20 {
			continue
		}

		udpInPkts.Add(1)
		udpInBytes.Add(int64(n))

		// Remember client address
		clientAddr.Store(addr)

		pkt := buf[:n]

		// Extract dst IP from IP header for sendto
		dstIP := pkt[16:20]
		sa := &syscall.SockaddrInet4{
			Addr: [4]byte{dstIP[0], dstIP[1], dstIP[2], dstIP[3]},
		}

		err = syscall.Sendto(rawSendFd, pkt, 0, sa)
		if err != nil {
			rawSendErr.Add(1)
		}
	}
}

// internetToUDP: read return TCP traffic from raw socket, send to client via UDP
func internetToUDP(rawTCPfd int, udpConn *net.UDPConn) {
	buf := make([]byte, 65536)
	sp := uint16(*startPort)

	for {
		n, _, err := syscall.Recvfrom(rawTCPfd, buf, 0)
		if err != nil {
			log.Fatal("raw tcp read:", err)
		}
		if n < 20 {
			continue
		}

		rawInPkts.Add(1)

		// IPv4 only
		if buf[0]>>4 != 4 {
			rawSkipped.Add(1)
			continue
		}

		ihl := (buf[0] & 0x0F) * 4
		if int(ihl)+4 > n {
			rawSkipped.Add(1)
			continue
		}

		// Check dst port — only forward if in our client's port range
		dstPort := binary.BigEndian.Uint16(buf[ihl+2 : ihl+4])
		if dstPort < sp {
			rawSkipped.Add(1)
			continue
		}

		// Check for oversized packets (GRO coalescing)
		if n > 1500 {
			rawOversized.Add(1)
		}

		ca := clientAddr.Load()
		if ca == nil {
			continue
		}

		_, err = udpConn.WriteToUDP(buf[:n], ca)
		if err != nil {
			log.Println("udp write:", err)
		}
		udpOutPkts.Add(1)
		udpOutBytes.Add(int64(n))
	}
}
