package main

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/tunnels-is/tunnels/signal"
)

const (
	proxyEntryMaxAge     = 24 * time.Hour
	proxyCleanupInterval = 1 * time.Hour
)

var (
	proxyClientMap  = make(map[string]time.Time)
	proxyClientLock = sync.RWMutex{}
)

func AddProxyClient(ip string) {
	proxyClientLock.Lock()
	defer proxyClientLock.Unlock()
	proxyClientMap[ip] = time.Now()
	logger.Info("proxy client added", slog.String("ip", ip))
}

func IsProxyClientAllowed(ip string) bool {
	proxyClientLock.RLock()
	defer proxyClientLock.RUnlock()

	created, exists := proxyClientMap[ip]
	if !exists {
		return false
	}

	if time.Since(created) > proxyEntryMaxAge {
		return false
	}

	return true
}

func RemoveProxyClient(ip string) {
	proxyClientLock.Lock()
	defer proxyClientLock.Unlock()
	delete(proxyClientMap, ip)
}

func cleanupProxyClients() {
	proxyClientLock.Lock()
	defer proxyClientLock.Unlock()

	now := time.Now()
	removed := 0
	for ip, created := range proxyClientMap {
		if now.Sub(created) > proxyEntryMaxAge {
			delete(proxyClientMap, ip)
			removed++
		}
	}

	if removed > 0 {
		logger.Info("proxy client cleanup completed",
			slog.Int("removed", removed),
			slog.Int("remaining", len(proxyClientMap)))
	}
}

func StartProxyCleanupRoutine() {
	ctx := *CTX.Load()
	cancel := *Cancel.Load()

	go signal.NewSignal("PROXY_CLEANUP", ctx, cancel, proxyCleanupInterval, goroutineLogger, cleanupProxyClients)
}

const (
	socks5Version        = 0x05
	socks5AuthNone       = 0x00
	socks5AuthNoAccept   = 0xFF
	socks5CmdConnect     = 0x01
	socks5AtypIPv4       = 0x01
	socks5AtypDomain     = 0x03
	socks5AtypIPv6       = 0x04
	socks5RepSuccess     = 0x00
	socks5RepHostUnreach = 0x04
	socks5RepCmdNotSupp  = 0x07
	socks5RepAddrNotSupp = 0x08
)

func LaunchSOCKS5Server() {
	config := Config.Load()
	addr := fmt.Sprintf("%s:%s", config.SOCKSIP, config.SOCKSPort)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("failed to start SOCKS5 server", slog.Any("err", err))
		return
	}
	defer listener.Close()

	logger.Info("SOCKS5 proxy server started", slog.String("addr", addr))

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Error("SOCKS5 accept error", slog.Any("err", err))
			continue
		}

		go handleSOCKS5Connection(conn)
	}
}

func handleSOCKS5Connection(conn net.Conn) {
	defer conn.Close()
	defer BasicRecover()

	clientAddr := conn.RemoteAddr().String()
	clientIP, _, err := net.SplitHostPort(clientAddr)
	if err != nil {
		logger.Error("failed to parse client address", slog.Any("err", err))
		return
	}

	if !IsProxyClientAllowed(clientIP) {
		logger.Warn("unauthorized SOCKS5 connection attempt", slog.String("ip", clientIP))
		return
	}

	conn.SetDeadline(time.Now().Add(5 * time.Minute))

	if err := handleSOCKS5Handshake(conn); err != nil {
		logger.Error("SOCKS5 handshake failed", slog.Any("err", err), slog.String("ip", clientIP))
		return
	}

	targetConn, err := handleSOCKS5Request(conn)
	if err != nil {
		logger.Error("SOCKS5 request failed", slog.Any("err", err), slog.String("ip", clientIP))
		return
	}
	defer targetConn.Close()

	conn.SetDeadline(time.Time{})
	targetConn.SetDeadline(time.Time{})

	relay(conn, targetConn)
}

func handleSOCKS5Handshake(conn net.Conn) error {
	buf := make([]byte, 256)

	n, err := conn.Read(buf[:2])
	if err != nil || n != 2 {
		return fmt.Errorf("failed to read SOCKS5 greeting")
	}

	if buf[0] != socks5Version {
		return fmt.Errorf("unsupported SOCKS version: %d", buf[0])
	}

	nmethods := int(buf[1])

	n, err = conn.Read(buf[:nmethods])
	if err != nil || n != nmethods {
		return fmt.Errorf("failed to read auth methods")
	}

	hasNoAuth := false
	for i := range nmethods {
		if buf[i] == socks5AuthNone {
			hasNoAuth = true
			break
		}
	}

	if !hasNoAuth {
		conn.Write([]byte{socks5Version, socks5AuthNoAccept})
		return fmt.Errorf("no acceptable auth method")
	}

	_, err = conn.Write([]byte{socks5Version, socks5AuthNone})
	return err
}

func handleSOCKS5Request(conn net.Conn) (net.Conn, error) {
	buf := make([]byte, 256)

	n, err := conn.Read(buf[:4])
	if err != nil || n != 4 {
		return nil, fmt.Errorf("failed to read SOCKS5 request")
	}

	if buf[0] != socks5Version {
		return nil, fmt.Errorf("unsupported SOCKS version in request")
	}

	if buf[1] != socks5CmdConnect {
		sendSOCKS5Reply(conn, socks5RepCmdNotSupp, nil, 0)
		return nil, fmt.Errorf("unsupported command: %d", buf[1])
	}

	addrType := buf[3]

	var targetAddr string
	var targetPort uint16

	switch addrType {
	case socks5AtypIPv4:
		n, err = conn.Read(buf[:4])
		if err != nil || n != 4 {
			return nil, fmt.Errorf("failed to read IPv4 address")
		}
		targetAddr = net.IP(buf[:4]).String()

	case socks5AtypDomain:
		n, err = conn.Read(buf[:1])
		if err != nil || n != 1 {
			return nil, fmt.Errorf("failed to read domain length")
		}
		domainLen := int(buf[0])

		n, err = conn.Read(buf[:domainLen])
		if err != nil || n != domainLen {
			return nil, fmt.Errorf("failed to read domain")
		}
		targetAddr = string(buf[:domainLen])

	case socks5AtypIPv6:
		n, err = conn.Read(buf[:16])
		if err != nil || n != 16 {
			return nil, fmt.Errorf("failed to read IPv6 address")
		}
		targetAddr = net.IP(buf[:16]).String()

	default:
		sendSOCKS5Reply(conn, socks5RepAddrNotSupp, nil, 0)
		return nil, fmt.Errorf("unsupported address type: %d", addrType)
	}

	n, err = conn.Read(buf[:2])
	if err != nil || n != 2 {
		return nil, fmt.Errorf("failed to read port")
	}
	targetPort = uint16(buf[0])<<8 | uint16(buf[1])

	targetFullAddr := net.JoinHostPort(targetAddr, fmt.Sprintf("%d", targetPort))
	targetConn, err := net.DialTimeout("tcp", targetFullAddr, 10*time.Second)
	if err != nil {
		replyCode := byte(socks5RepHostUnreach)
		if opErr, ok := err.(*net.OpError); ok {
			if opErr.Timeout() {
				replyCode = socks5RepHostUnreach
			}
		}
		sendSOCKS5Reply(conn, replyCode, nil, 0)
		return nil, fmt.Errorf("failed to connect to target: %w", err)
	}

	localAddr := targetConn.LocalAddr().(*net.TCPAddr)

	sendSOCKS5Reply(conn, socks5RepSuccess, localAddr.IP.To4(), uint16(localAddr.Port))

	return targetConn, nil
}

func sendSOCKS5Reply(conn net.Conn, rep byte, bindIP net.IP, bindPort uint16) {
	reply := make([]byte, 10)
	reply[0] = socks5Version
	reply[1] = rep
	reply[2] = 0x00
	reply[3] = socks5AtypIPv4

	if len(bindIP) >= 4 {
		copy(reply[4:8], bindIP[:4])
	}

	reply[8] = byte(bindPort >> 8)
	reply[9] = byte(bindPort & 0xFF)

	conn.Write(reply)
}

func relay(conn1, conn2 net.Conn) {
	done := make(chan struct{}, 2)

	copyData := func(dst, src net.Conn) {
		defer func() { done <- struct{}{} }()
		io.Copy(dst, src)
	}

	go copyData(conn1, conn2)
	go copyData(conn2, conn1)

	<-done
}
