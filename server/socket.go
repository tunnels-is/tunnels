package main

import (
	"context"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"errors"
	"log"
	"math"
	"net"
	"runtime/debug"
	"strconv"
	"strings"
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

	if !CR.UserID.IsZero() {
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
	} else {
		// this might not be needed.
		// if Config.VPL != nil {
		// 	if totalUserC > Config.VPL.MaxDevices {
		// 		WARN("Max devices reached", totalUserC)
		// 		return
		// 	}
		// }
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
			ClientCoreMappings[i].DeviceToken = CR.DeviceToken
			ClientCoreMappings[i].EH = EH
			ClientCoreMappings[i].Created = time.Now()
			ClientCoreMappings[i].ToUser = make(chan []byte, 500000)
			ClientCoreMappings[i].FromUser = make(chan Packet, 500000)
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

func ExternalSocketListener(SIGNAL *SIGNAL) {
	defer RecoverAndReturnID(SIGNAL, 1)

	var ETH_P_ALL uint16 = 0x0003 // Listen for all Ethernet protocols
	var err error
	rawSock, err = syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(htons(ETH_P_ALL)))
	defer syscall.Close(rawSock)
	if err != nil {
		ERR("Unable to make raw socket err:", err)
		return
	}

	var iface *net.Interface
	ifList, _ := net.Interfaces()
	for _, v := range ifList {
		addrs, e := v.Addrs()
		if e != nil {
			continue
		}
		for _, iv := range addrs {
			if strings.Split(iv.String(), "/")[0] == InterfaceIP.To4().String() {
				iface = &v
			}
		}
	}
	if iface == nil {
		ERR("unable to find interface for ip:", InterfaceIP.To16())
		return
	}

	// Construct a sockaddr_ll structure for binding to the interface
	var addr syscall.SockaddrLinklayer
	addr.Protocol = htons(ETH_P_ALL)
	addr.Ifindex = iface.Index
	rawSockAddr = syscall.SockaddrLinklayer{
		Protocol: htons(0x0800), // Protocol in network byte order
		Ifindex:  iface.Index,
		Hatype:   0, // ARP Hardware Type (Ethernet is 1, 0 should be ok)
		Pkttype:  0, // Packet Type (PACKET_OUTGOING)
		Halen:    0,
	}

	err = syscall.Bind(rawSock, &addr)
	if err != nil {
		ERR("unable to bind raw socket listener to interface:", iface, err)
		return
	}

	portInt, err := strconv.Atoi(Config.DataPort)
	if err != nil {
		panic(err)
	}
	datadstport := uint16(portInt)

	var DSTP uint16
	var IHL int
	var PM *PortRange
	var n int

	buffer := make([]byte, 1500) // Adjust buffer size as needed
	var out []byte

	for {
		n, _, err = syscall.Recvfrom(rawSock, buffer, 0)
		if err != nil {
			log.Printf("Error receiving packet: %v", err)
			time.Sleep(1 * time.Millisecond)
			continue
		}

		// too small to be ipv4
		if n < 60 {
			continue
		}
		out = buffer[14:n]

		// IP v4 protocol
		if (out[0] >> 4) != 4 {
			continue
		}

		IHL = (int(out[0]&0x0F) << 2)
		SRCP := binary.BigEndian.Uint16(out[IHL : IHL+2])
		DSTP = binary.BigEndian.Uint16(out[IHL+2 : IHL+4])
		if SRCP == 22 || DSTP == 22 {
			continue
		}
		if DSTP == datadstport {
			continue
		} else if DSTP < uint16(Config.StartPort) || DSTP > uint16(Config.EndPort) {
			// fmt.Println("ignore:", DSTP)
			continue
		}
		// fmt.Println("IH:", IHL, DSTP, out[9])

		PM = PortToCoreMapping[DSTP]
		if PM == nil || PM.Client == nil {
			continue
		}

		if PM.Client.Addr == nil {
			WARN("TCP: no mapping addr: ", DSTP)
			continue
		}
		// fmt.Println(PM.Client.ID)

		select {
		case PM.Client.ToUser <- CopySlice(out):
		default:
			WARN("TCP: packet channel full: ", DSTP)
		}
	}

}

func htons(i uint16) uint16 {
	b := make([]byte, 2)
	b[0] = byte(i & 0xFF)
	b[1] = byte(i >> 8)
	return uint16(b[0])<<8 | uint16(b[1])
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
	var NIP net.IP
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

		PACKET, err := CM.EH.SEAL.Open1(
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
	var S4 [4]byte
	var S4Port [2]byte
	var FIN byte
	var RST byte
	var originCM *UserCoreMapping
	var skipFirewall bool

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
			S4[0] = PACKET[12]
			S4[1] = PACKET[13]
			S4[2] = PACKET[14]
			S4[3] = PACKET[15]
			originCM = VPLIPToCore[S4[0]][S4[1]][S4[2]][S4[3]]
			skipFirewall = false
			if originCM != nil {
				for _, entity := range Config.AdminEntities {
					if entity == originCM.DeviceToken || entity == originCM.ID {
						skipFirewall = true
						break
					}
				}
			}

			if !AllowAll && !CM.DisableFirewall && !skipFirewall {

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

		err = syscall.Sendto(dataSocketFD,
			CM.EH.SEAL.Seal2(PACKET, CM.Uindex),
			0,
			CM.Addr,
		)
		if err != nil {
			WARN("dataSocketFD sendTo err:", err)
			return
		}
	}
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
