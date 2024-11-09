package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"runtime/debug"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/zveinn/crypt"
	"github.com/zveinn/tunnels"
	"golang.org/x/net/quic"
	"golang.org/x/sys/unix"
)

var CertPool *x509.CertPool

func ControlSocketListener(SIGNAL *SIGNAL) {
	defer RecoverAndReturnID(SIGNAL, 1)
	l, err := quic.Listen(
		"udp4",
		net.JoinHostPort(Config.ControlIP, Config.ControlPort),
		quicConfig,
	)
	if err != nil {
		panic(err)
	}
	for {
		con, err := l.Accept(context.Background())
		if err != nil {
			ERR("ACCEPT ERROR:", err)
			time.Sleep(3 * time.Millisecond)
			continue
		}

		go acceptUserUDPTLSSocket(con)
	}
}

func validateSignatureAndExtractConnectRequest(buff []byte) (scr crypt.SignedConnectRequest, cr *ConnectRequest, err error) {
	scr, err = crypt.ValidateSignature(buff, publicSigningKey)
	if err != nil {
		WARN("Invalid payload signature:", err)
		return
	}

	cr = new(ConnectRequest)
	err = json.Unmarshal(scr.Payload, &cr)
	if err != nil {
		WARN("Invalid connect request(unmarshal):", err)
		err = errors.New("Invalid payload from user")
		return
	}

	if Config.ID != cr.SeverID {
		ERR("Invalid server, current id: ", Config.ID, " provided id: ", cr.SeverID)
		err = errors.New("invalid server id")
		return
	}

	if time.Since(cr.Created).Seconds() > 20 {
		ERR("Expired connection request", err)
		err = errors.New("invalid cr timer")
		return
	}

	return
}

func acceptUserUDPTLSSocket(conn *quic.Conn) {
	var s *quic.Stream
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
		if s != nil {
			s.Close()
		}
		if conn != nil {
			conn.Close()
		}
	}()

	buff := make([]byte, 1500)
	var err error
	var n int

	s, err = conn.AcceptStream(context.Background())
	if err != nil {
		ERR("Unable to accept stream:", err)
		return
	}

	n, err = s.Read(buff)
	if err != nil {
		ERR("Unable to read from client:", err)
		return
	}

	// fmt.Println("CR+HANDSHAKE:", string(buff[:n]))

	_, CR, err := validateSignatureAndExtractConnectRequest(buff[:n])
	if err != nil {
		ERR("Payload validation error: ", err)
		return
	}

	totalConnections := 0
	totalUserConnection := 0
	for i := range UserPortMappings {
		if UserPortMappings[i] == nil {
			continue
		}

		if UserPortMappings[i].ID == CR.UserID.Hex() {
			totalUserConnection++
		}

		totalConnections++
	}

	if totalConnections >= slots {
		WARN("Server is full", totalUserConnection, totalConnections, slots)
		return
	}

	if totalUserConnection > Config.UserMaxConnections {
		WARN("User has more then 4 connections", totalUserConnection)
		return
	}

	var EH *crypt.SocketWrapper
	EH, err = crypt.NewEncryptionHandler(CR.EncType)
	if err != nil {
		ERR("unable to create encryption handler", err)
		return
	}

	EH.SetHandshakeStream(s)

	err = EH.InitHandshake()
	if err != nil {
		ERR("Handshakte initialization failed", err)
		return
	}

	CRR := CreateCRRFromServer(Config)
	index, err := CreateClientPortMapping(CRR, CR, EH)
	if err != nil {
		ERR("Port allocation failed", err)
		return
	}

	CRRB, err := json.Marshal(CRR)
	if err != nil {
		ERR("Unable to marshal CCR", err)
		return
	}

	n, err = s.Write(CRRB)
	if err != nil {
		ERR("Unable to write CRRB", err)
		return
	}
	if n != len(CRRB) {
		ERR("Did not write full CRRB", err)
		return
	}
	s.Flush()

	go toUserChannel(index)
	go fromUserChannel(index)
}

type PortRange struct {
	StartPort uint16
	EndPort   uint16
	Client    *UserPortMapping
}

type UserPortMapping struct {
	ID                 string
	Version            int
	PortRange          *PortRange
	ToUser             chan []byte
	FromUser           chan Packet
	LastPingFromClient time.Time
	EH                 *crypt.SocketWrapper
	Uindex             []byte
	Addr               syscall.Sockaddr
	Created            time.Time
}

var (
	UserPortMappings  [math.MaxUint16 + 1]*UserPortMapping
	PortMappingLock   = sync.Mutex{}
	PortToUserMapping [math.MaxUint16 + 1]*PortRange
)

func CreateClientPortMapping(CRR *ConnectRequestResponse, CR *ConnectRequest, EH *crypt.SocketWrapper) (index int, err error) {
	defer func() {
		r := recover()
		if r != nil {
			ERR(r, string(debug.Stack()))
		}
	}()

	wasAllocated := false
	for i := range UserPortMappings {
		if UserPortMappings[i] == nil {
			wasAllocated = true
			index = i
			PortMappingLock.Lock()
			UserPortMappings[i] = new(UserPortMapping)
			PortMappingLock.Unlock()

			UserPortMappings[i].ID = CR.UserID.Hex()
			UserPortMappings[i].EH = EH
			UserPortMappings[i].Created = time.Now()
			UserPortMappings[i].ToUser = make(chan []byte, 300000)
			UserPortMappings[i].FromUser = make(chan Packet, 300000)
			UserPortMappings[i].LastPingFromClient = time.Now()
			UserPortMappings[i].Uindex = make([]byte, 2)
			binary.BigEndian.PutUint16(UserPortMappings[i].Uindex, uint16(index))

			break
		}
	}
	if !wasAllocated {
		return 0, errors.New("No session slots available on the server")
	}

	var PR *PortRange
	for i := range PortToUserMapping {
		if i < int(Config.StartPort) {
			continue
		}

		if PortToUserMapping[i] == nil {
			// WARN("PORT TO CLIENT MAPPING IS NIL: ", i)
			continue
		}

		if PortToUserMapping[i].Client == nil {
			PortToUserMapping[i].Client = UserPortMappings[index]
			UserPortMappings[index].PortRange = PortToUserMapping[i]
			PR = PortToUserMapping[i]
			break
		}
	}

	if PR == nil {
		WARN("Unable to assign user to port mapping, no available space")
		return 0, errors.New("No port mappings available on the server")
	}

	CRR.StartPort = PR.StartPort
	CRR.EndPort = PR.EndPort
	CRR.Index = index

	return
}

func ExternalTCPListener(SIGNAL *SIGNAL) {
	defer RecoverAndReturnID(SIGNAL, 1)
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
		fmt.Println("tcp LISTENER EXITING")
	}()

	var err error
	rawTCPSockFD, err = syscall.Socket(
		syscall.AF_INET,
		syscall.SOCK_RAW,
		syscall.IPPROTO_TCP,
	)
	if err != nil {
		syscall.Close(rawTCPSockFD)
		fmt.Println("Unable to make raw socket err:", err)
		return
	}

	ipx := InterfaceIP.To4()
	addr := &syscall.SockaddrInet4{
		Addr: [4]byte{
			ipx[0],
			ipx[1],
			ipx[2],
			ipx[3],
		},
	}

	err = syscall.Bind(rawTCPSockFD, addr)
	if err != nil {
		syscall.Close(rawTCPSockFD)
		fmt.Println("Unable to bind net listener socket err:", err)
		return
	}

	var DSTP uint16
	var IHL byte
	var PM *PortRange
	var n int
	var version byte
	buffer := make([]byte, math.MaxUint16)

	for {
		n, _, err = syscall.Recvfrom(rawTCPSockFD, buffer, 0)
		if err != nil {
			fmt.Println("Error reading from raw TCP sock:", err)
			return
		}

		version = buffer[0] >> 4
		if version != 4 {
			fmt.Println("ignoring none v4", version)
			continue
		}
		// fmt.Println(buffer[:n])

		// TODO .. use mask
		IHL = ((buffer[0] << 4) >> 4) * 4
		DSTP = binary.BigEndian.Uint16(buffer[IHL+2 : IHL+4])
		PM = PortToUserMapping[DSTP]
		if PM == nil || PM.Client == nil {
			continue
		}

		if PM.Client.Addr == nil {
			WARN("TCP: no mapping addr: ", DSTP)
			continue
		}

		// fmt.Println(">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>")
		// fmt.Println(n)
		// fmt.Println(buffer[:n])
		// fmt.Println(">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>")
		select {
		case PM.Client.ToUser <- CopySlice(buffer[:n]):
			// fmt.Println("UDPIN:", len(buffer[:n]))
		default:
			WARN("TCP: packet channel full: ", DSTP)
		}
	}
}

func ExternalUDPListener(SIGNAL *SIGNAL) {
	defer RecoverAndReturnID(SIGNAL, 1)
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
		fmt.Println("UPD LISTENER EXITING")
	}()

	var err error
	rawUDPSockFD, err = syscall.Socket(
		syscall.AF_INET,
		syscall.SOCK_RAW,
		syscall.IPPROTO_UDP,
	)
	if err != nil {
		syscall.Close(rawUDPSockFD)
		fmt.Println("Unable to make raw socket err:", err)
		return
	}

	ipx := InterfaceIP.To4()
	addr := &syscall.SockaddrInet4{
		Addr: [4]byte{
			ipx[0],
			ipx[1],
			ipx[2],
			ipx[3],
		},
	}

	err = syscall.Bind(rawUDPSockFD, addr)
	if err != nil {
		syscall.Close(rawUDPSockFD)
		fmt.Println("Unable to bind net listener socket err:", err)
		return
	}

	var DSTP uint16
	var IHL byte
	var PM *PortRange
	var n int
	var version byte
	buffer := make([]byte, math.MaxUint16)

	for {
		n, _, err = syscall.Recvfrom(rawUDPSockFD, buffer, 0)
		if err != nil {
			fmt.Println("Error reading from raw UDP sock:", err)
			return
		}

		version = buffer[0] >> 4
		if version != 4 {
			// fmt.Println("ignoring none v4", version)
			continue
		}
		// fmt.Println(buffer[:n])

		// TODO .. use mask
		IHL = ((buffer[0] << 4) >> 4) * 4
		DSTP = binary.BigEndian.Uint16(buffer[IHL+2 : IHL+4])
		PM = PortToUserMapping[DSTP]
		if PM == nil || PM.Client == nil {
			continue
		}

		if PM.Client.Addr == nil {
			WARN("UDP: no mapping addr: ", DSTP)
			continue
		}

		// fmt.Println(">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>")
		// fmt.Println(n)
		// fmt.Println(buffer[:n])
		// fmt.Println(">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>")
		select {
		case PM.Client.ToUser <- CopySlice(buffer[:n]):
			// fmt.Println("UDPIN:", len(buffer[:n]))
		default:
			WARN("UDP: packet channel full: ", DSTP)
		}
	}
}

func htons(v uint16) uint16 {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, v)
	return binary.LittleEndian.Uint16(b)
}

func DataSocketListener(SIGNAL *SIGNAL) {
	defer RecoverAndReturnID(SIGNAL, 1)

	var err error
	dataSocketFD, err = syscall.Socket(
		unix.AF_INET,
		unix.SOCK_DGRAM,
		unix.IPPROTO_UDP,
	)
	if err != nil {
		panic(err)
	}

	portInt, err := strconv.Atoi(Config.DataPort)
	if err != nil {
		panic(err)
	}
	ip := net.ParseIP(Config.ControlIP)
	if ip != nil {
		ip = ip.To4()
	} else {
		panic("invalid ControlIP")
	}

	addr := &syscall.SockaddrInet4{
		Port: portInt,
		Addr: [4]byte{ip[0], ip[1], ip[2], ip[3]},
	}

	err = syscall.Bind(dataSocketFD, addr)
	if err != nil {
		panic(err)
	}

	buff := make([]byte, math.MaxUint16)
	var id uint16
	for {
		n, addr, err := syscall.Recvfrom(dataSocketFD, buff, 0)
		if err != nil {
			fmt.Println(err)
			return
		}
		// fmt.Println("------------------------------------")
		// fmt.Println(n, addr, err)
		// fmt.Println(buff[:n])
		// fmt.Println("------------------------------------")
		id = binary.BigEndian.Uint16(buff[0:2])

		// IF PROXY
		// --- send forward to next server.
		if UserPortMappings[id] != nil {
			UserPortMappings[id].FromUser <- Packet{
				addr: addr,
				data: CopySlice(buff[:n]),
			}
		}
	}
}

type Packet struct {
	addr syscall.Sockaddr
	data []byte
}

func fromUserChannel(index int) {
	shouldRestart := true
	defer func() {
		if r := recover(); r != nil {
			ERR(3, r, string(debug.Stack()))
		}

		if shouldRestart {
			fromUserChannelMonitor <- index
		} else {
			NukeClient(index)
		}
	}()

	CM := UserPortMappings[index]
	if CM == nil {
		shouldRestart = false
		return
	}

	var payload Packet
	var packet []byte
	var NETIP net.IP
	var err error
	var ok bool
	staging := make([]byte, 70000)

	for {
		payload, ok = <-CM.FromUser
		if !ok {
			shouldRestart = false
			return
		}

		packet, err = CM.EH.SEAL.Open1(
			payload.data[10:],
			payload.data[2:10],
			staging[:0],
			payload.data[0:2],
		)
		if err != nil {
			ERR("Authentication error:", err)
			continue
		}

		CM.Addr = payload.addr
		if len(packet) < 20 {
			CM.LastPingFromClient = time.Now()
			continue
		}

		NETIP = packet[16:20]
		if !Config.LocalNetworkAccess {
			if IS_LOCAL(NETIP) {
				continue
			}
		}

		if !Config.InternetAccess {
			if !IS_LOCAL(NETIP) {
				continue
			}
		}

		if packet[9] == 17 {
			_, err = UDPRWC.Write(packet)
			if err != nil {
				WARN("UDPRWC err:", err)
				continue
			}
		} else {
			_, err = TCPRWC.Write(packet)
			if err != nil {
				WARN("TCPRWC err:", err)
				continue
			}
		}

	}
}

func IS_LOCAL(ip net.IP) bool {
	if ip.IsLinkLocalMulticast() {
		return true
	}
	if ip.IsLinkLocalUnicast() {
		return true
	}
	if ip.IsLoopback() {
		return true
	}
	if ip.IsPrivate() {
		return true
	}
	if ip.IsInterfaceLocalMulticast() {
		return true
	}

	return false
}

func toUserChannel(index int) {
	shouldRestart := true
	defer func() {
		if r := recover(); r != nil {
			ERR(3, r, string(debug.Stack()))
		}

		if shouldRestart {
			toUserChannelMonitor <- index
		} else {
			NukeClient(index)
		}
	}()

	CM := UserPortMappings[index]
	if CM == nil {
		shouldRestart = false
		return
	}

	var PACKET []byte
	var NETIP net.IP
	var err error
	IFipTo4 := InterfaceIP.To4()
	var ok bool
	var out []byte

	for {
		PACKET, ok = <-CM.ToUser
		if !ok {
			shouldRestart = false
			return
		}

		if len(PACKET) < 20 {
			continue
		}

		if PACKET[9] != 6 && PACKET[9] != 17 {
			continue
		}

		NETIP = PACKET[16:20]
		if !bytes.Equal(NETIP, IFipTo4) {
			continue
		}

		out = CM.EH.SEAL.Seal2(PACKET, CM.Uindex)
		// fmt.Println("----- TO USER -----")
		// fmt.Println(dataSocketFD)
		// fmt.Println(CM.Addr)
		// fmt.Println(out)
		// fmt.Println("------------------")

		err = syscall.Sendto(dataSocketFD, out, 0, CM.Addr)
		if err != nil {
			WARN("dataSocketFD sendTo err:", err)
			return
		}
	}
}

func createRawTCPSocket() (
	buffer []byte,
	socket *tunnels.RawSocket,
	err error,
) {
	interfaceString := findInterfaceName()
	if interfaceString == "" {
		err = errors.New("no interface found")
		return
	}

	buffer = make([]byte, math.MaxUint16)
	socket = &tunnels.RawSocket{
		InterfaceName: interfaceString,
		SocketBuffer:  buffer,
		Domain:        syscall.AF_INET,
		Type:          syscall.SOCK_RAW,
		Proto:         syscall.IPPROTO_TCP,
	}

	err = socket.Create()
	if err != nil {
		return
	}

	TCPRWC = socket.RWC

	return
}

func findInterfaceName() (name string) {
	ifs, _ := net.Interfaces()
	for _, v := range ifs {
		// fmt.Println(i, v)
		addrs, _ := v.Addrs()
		for _, vv := range addrs {
			// fmt.Println(ii, vv)
			_, ipnetA, _ := net.ParseCIDR(vv.String())
			// fmt.Println(ipnetA, ipnetA.Contains(INTERFACE_IP))
			if ipnetA.Contains(InterfaceIP) {
				name = v.Name
			}
		}
	}
	return
}

func createRawUDPSocket() (
	buffer []byte,
	socket *tunnels.RawSocket,
	err error,
) {
	interfaceString := findInterfaceName()
	if interfaceString == "" {
		err = errors.New("no interface found")
		return
	}

	buffer = make([]byte, math.MaxUint16)
	socket = &tunnels.RawSocket{
		InterfaceName: interfaceString,
		SocketBuffer:  buffer,
		Domain:        syscall.AF_INET,
		Type:          syscall.SOCK_RAW,
		Proto:         syscall.IPPROTO_UDP,
	}

	err = socket.Create()
	if err != nil {
		return
	}

	UDPRWC = socket.RWC

	return
}
