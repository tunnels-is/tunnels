package main

import (
	"context"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"net"
	"runtime/debug"
	"strconv"
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
			ERR(r, string(debug.Stack()))
		}
		if s != nil {
			s.Close()
		}
		if conn != nil {
			conn.Close()
		}
	}()

	buff := make([]byte, 10000)
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

	_, CR, err := validateSignatureAndExtractConnectRequest(buff[:n])
	if err != nil {
		ERR("Payload validation error: ", err)
		return
	}

	totalC, totalUserC := countConnections(CR.UserID.Hex())

	if CR.RequestingPorts {
		if totalC >= slots {
			WARN("Server is full", totalUserC, totalC, slots)
			return
		}
	}

	if totalUserC > Config.UserMaxConnections {
		WARN("User has more then 4 connections", totalUserC)
		return
	}

	if Config.VPL != nil {
		if totalUserC > Config.VPL.MaxDevices {
			WARN("Max devices reached", totalUserC)
			return
		}
	}

	var EH *crypt.SocketWrapper
	EH, err = crypt.NewEncryptionHandler(CR.EncType, CR.CurveType)
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
	index, err := CreateClientCoreMapping(CRR, CR, EH)
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

func countConnections(id string) (count int, userCount int) {
	for i := range ClientCoreMappings {
		if ClientCoreMappings[i] == nil {
			continue
		}

		if ClientCoreMappings[i].ID == id {
			userCount++
		}

		count++
	}
	return
}

func CreateClientCoreMapping(CRR *ConnectRequestResponse, CR *ConnectRequest, EH *crypt.SocketWrapper) (index int, err error) {
	defer func() {
		r := recover()
		if r != nil {
			ERR(r, string(debug.Stack()))
		}
	}()

	wasAllocated := false
	for i := range ClientCoreMappings {
		if ClientCoreMappings[i] == nil {
			index = i

			COREm.Lock()
			if ClientCoreMappings[i] == nil {
				ClientCoreMappings[i] = new(UserCoreMapping)
				wasAllocated = true
			}
			COREm.Unlock()
			if !wasAllocated {
				continue
			}

			ClientCoreMappings[i].ID = CR.UserID.Hex()
			ClientCoreMappings[i].EH = EH
			ClientCoreMappings[i].Created = time.Now()
			ClientCoreMappings[i].ToUser = make(chan []byte, 300000)
			ClientCoreMappings[i].FromUser = make(chan Packet, 300000)
			ClientCoreMappings[i].LastPingFromClient = time.Now()
			ClientCoreMappings[i].Uindex = make([]byte, 2)
			binary.BigEndian.PutUint16(ClientCoreMappings[i].Uindex, uint16(index))

			break
		}
	}

	if !wasAllocated {
		return 0, errors.New("No session slots available on the server")
	}

	CRR.Index = index

	if VPLEnabled {
		err = assignDHCP(CR, CRR, index)
		if err != nil {
			WARN("Unable to assign DHCP address")
			NukeClient(index)
			return 0, err
		}
	}

	if CR.RequestingPorts {
		err := allocatePorts(CRR, index)
		if err != nil {
			NukeClient(index)
			WARN("Unable to assign user to port mapping, no available space")
			return 0, err
		}
	}

	CRR.VPLNetwork = Config.VPL.Network

	return
}

func ExternalTCPListener(SIGNAL *SIGNAL) {
	defer RecoverAndReturnID(SIGNAL, 1)
	defer func() {
		r := recover()
		if r != nil {
			ERR(r, string(debug.Stack()))
		}
	}()

	var err error
	rawTCPSockFD, err = syscall.Socket(
		syscall.AF_INET,
		syscall.SOCK_RAW,
		syscall.IPPROTO_TCP,
	)
	if err != nil {
		syscall.Close(rawTCPSockFD)
		ERR("Unable to make raw socket err:", err)
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
		ERR("Unable to bind net listener socket err:", err)
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
			ERR("Error reading from raw TCP sock:", err)
			return
		}

		version = buffer[0] >> 4
		if version != 4 {
			continue
		}

		// TODO .. use mask
		IHL = ((buffer[0] << 4) >> 4) * 4
		DSTP = binary.BigEndian.Uint16(buffer[IHL+2 : IHL+4])
		PM = PortToCoreMapping[DSTP]
		if PM == nil || PM.Client == nil {
			continue
		}

		if PM.Client.Addr == nil {
			WARN("TCP: no mapping addr: ", DSTP)
			continue
		}

		select {
		case PM.Client.ToUser <- CopySlice(buffer[:n]):
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
			ERR(r, string(debug.Stack()))
		}
		ERR("UPD LISTENER EXITING")
	}()

	var err error
	rawUDPSockFD, err = syscall.Socket(
		syscall.AF_INET,
		syscall.SOCK_RAW,
		syscall.IPPROTO_UDP,
	)
	if err != nil {
		syscall.Close(rawUDPSockFD)
		ERR("Unable to make raw socket err:", err)
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
		ERR("Unable to bind net listener socket err:", err)
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
			ERR("Error reading from raw UDP sock:", err)
			return
		}

		version = buffer[0] >> 4
		if version != 4 {
			continue
		}

		// TODO .. use mask
		IHL = ((buffer[0] << 4) >> 4) * 4
		DSTP = binary.BigEndian.Uint16(buffer[IHL+2 : IHL+4])
		PM = PortToCoreMapping[DSTP]
		if PM == nil || PM.Client == nil {
			continue
		}

		if PM.Client.Addr == nil {
			WARN("UDP: no mapping addr: ", DSTP)
			continue
		}

		select {
		case PM.Client.ToUser <- CopySlice(buffer[:n]):
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
			ERR(err)
			return
		}
		id = binary.BigEndian.Uint16(buff[0:2])
		if ClientCoreMappings[id] != nil {
			ClientCoreMappings[id].FromUser <- Packet{
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

	CM := ClientCoreMappings[index]
	if CM == nil {
		shouldRestart = false
		return
	}

	var payload Packet
	var PACKET []byte
	var NIP net.IP
	var err error
	var ok bool
	staging := make([]byte, 100000)
	// clientCache := make(map[[4]byte]*UserCoreMapping)
	var D4 [4]byte
	var D4Port [2]byte
	var RST byte
	var FIN byte
	var SYN byte
	var targetCM *UserCoreMapping

	for {
		payload, ok = <-CM.FromUser
		if !ok {
			shouldRestart = false
			return
		}

		if len(payload.data) > len(staging) {
			panic("PAYLOAD BIGGER THEN STAGING .. THIS SHOULD NEVR HAPPEN")
		}

		PACKET, err = CM.EH.SEAL.Open1(
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
		if len(PACKET) < 20 {
			switch PACKET[0] {
			case ping:
				CM.LastPingFromClient = time.Now()
				if CM.DHCP != nil {
					CM.DHCP.Activity = time.Now()
				}
				if len(PACKET) > 4 {
					CM.CPU = PACKET[1]
					CM.RAM = PACKET[2]
					CM.Disk = PACKET[3]
				}
			default:
				CM.LastPingFromClient = time.Now()
			}
			continue
		}

		NIP = PACKET[16:20]
		if VPLEnabled {
			D4[0] = NIP[0]
			D4[1] = NIP[1]
			D4[2] = NIP[2]
			D4[3] = NIP[3]
			l := (PACKET[0] & 0x0F) * 4
			D4Port[0] = PACKET[l+2]
			D4Port[1] = PACKET[l+3]

			RST = PACKET[l+13] & 0x4
			FIN = PACKET[l+13] & 0x1
			SYN = PACKET[l+13] & 0x2

			targetCM = VPLIPToCore[D4[0]][D4[1]][D4[2]][D4[3]]
			if targetCM == nil {
				CM.DelHost(D4, "auto")
				return
			}

			if RST > 0 {
				CM.DelHost(D4, "auto")
			} else if SYN > 0 {
				CM.AddHost(D4, D4Port, "auto")
			} else if FIN > 0 {
				CM.SetFin(D4, D4Port, true)
			}

			select {
			case targetCM.ToUser <- CopySlice(PACKET):
			default:
				WARN("Client channel full:", PACKET[12:16], ">", D4)
			}
			continue
		}

		if !Config.LocalNetworkAccess {
			if IS_LOCAL(NIP) {
				continue
			}
		}

		if !Config.InternetAccess {
			if !IS_LOCAL(NIP) {
				continue
			}
		}

		if PACKET[9] == 17 {
			_, err = UDPRWC.Write(PACKET)
			if err != nil {
				WARN("UDPRWC err:", err)
				continue
			}
		} else {
			_, err = TCPRWC.Write(PACKET)
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

	CM := ClientCoreMappings[index]
	if CM == nil {
		shouldRestart = false
		return
	}

	var PACKET []byte
	var err error
	var ok bool
	var out []byte
	var S4 [4]byte
	var S4Port [2]byte
	var FIN byte
	var RST byte

	for {
		PACKET, ok = <-CM.ToUser
		if !ok {
			shouldRestart = false
			return
		}

		if PACKET[9] != 6 && PACKET[9] != 17 {
			continue
		}

		if VPLEnabled {
			if !AllowAll && !CM.DisableFirewall {
				S4[0] = PACKET[12]
				S4[1] = PACKET[13]
				S4[2] = PACKET[14]
				S4[3] = PACKET[15]
				l := (PACKET[0] & 0x0F) * 4
				S4Port[0] = PACKET[l]
				S4Port[1] = PACKET[l+1]

				RST = PACKET[l+13] & 0x4
				FIN = PACKET[l+13] & 0x1

				host := CM.IsHostAllowed(S4, S4Port)
				if host == nil {
					continue
				}
				if RST > 0 {
					CM.DelHost(S4, "auto")
				} else if FIN > 0 {
					if host.FFIN {
						CM.DelHost(S4, "auto")
					} else {
						CM.SetFin(S4, S4Port, false)
					}
				}
			}
		}

		out = CM.EH.SEAL.Seal2(PACKET, CM.Uindex)
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
		addrs, _ := v.Addrs()
		for _, vv := range addrs {
			_, ipnetA, _ := net.ParseCIDR(vv.String())
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
