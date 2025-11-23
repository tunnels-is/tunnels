package main

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net"
	"runtime/debug"
	"strconv"
	"syscall"
	"time"
	"unsafe"

	"github.com/tunnels-is/tunnels/crypt"
	"github.com/tunnels-is/tunnels/types"
	"golang.org/x/crypto/sha3"
	"golang.org/x/sys/unix"
)

// HashIdentifier creates a SHA3-256 hash from an identifier
func HashIdentifier(identifier string) string {
	hash := sha3.Sum256([]byte(identifier))
	return hex.EncodeToString(hash[:])
}

func countConnections(id string) (count int, userCount int) {
	for i := range clientCoreMappings {
		if clientCoreMappings[i] == nil {
			continue
		}

		if clientCoreMappings[i].ID == id {
			userCount++
		}

		count++
	}
	return
}

func CreateClientCoreMapping(CRR *types.ServerConnectResponse, CR *types.ControllerConnectRequest, EH *crypt.SocketWrapper) (index int, err error) {
	defer func() {
		r := recover()
		if r != nil {
			ERR(r, string(debug.Stack()))
		}
	}()

	wasAllocated := false
	for i := range clientCoreMappings {
		if clientCoreMappings[i] == nil {
			index = i

			coreMutex.Lock()
			if clientCoreMappings[i] == nil {
				clientCoreMappings[i] = new(UserCoreMapping)
				wasAllocated = true
			}
			coreMutex.Unlock()
			if !wasAllocated {
				continue
			}

			clientCoreMappings[i].ID = HashIdentifier(CR.UserID.Hex())
			if CR.DeviceToken != "" {
				clientCoreMappings[i].DeviceToken = HashIdentifier(CR.DeviceToken)
			} else {
				clientCoreMappings[i].DeviceToken = HashIdentifier(CR.DeviceKey)
			}

			clientCoreMappings[i].EH = EH
			clientCoreMappings[i].Created = time.Now()
			clientCoreMappings[i].ToUser = make(chan []byte, 500_000)
			clientCoreMappings[i].FromUser = make(chan Packet, 500_000)
			clientCoreMappings[i].LastPingFromClient = time.Now()
			clientCoreMappings[i].Uindex = make([]byte, 2)
			binary.BigEndian.PutUint16(clientCoreMappings[i].Uindex, uint16(index))

			break
		}
	}

	if !wasAllocated {
		return 0, errors.New("No session slots available on the server")
	}

	CRR.Index = index

	if LANEnabled {
		err = assignDHCP(CR, CRR, index)
		if err != nil {
			WARN("Unable to assign DHCP address")
			NukeClient(index)
			return 0, err
		}
		LOG(fmt.Sprintf("Assigned Index (%d)", index))
	}

	if CR.RequestingPorts {
		err := allocatePorts(CRR, index)
		if err != nil {
			NukeClient(index)
			WARN("Unable to assign user to port mapping, no available space")
			return 0, err
		}
	}

	Config := Config.Load()
	CRR.LAN = Config.Lan

	return
}

func ExternalTCPListener() {
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
		PM = portToCoreMapping[DSTP]
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

func ExternalUDPListener() {
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
	// cfg := Config.Load()

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
		// if DSTP < uint16(cfg.StartPort) {
		// 	continue
		// }
		PM = portToCoreMapping[DSTP]
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

func DataSocketListener() {
	Config := Config.Load()
	var err error
	dataSocketFD, err = syscall.Socket(
		unix.AF_INET,
		unix.SOCK_DGRAM,
		unix.IPPROTO_UDP,
	)
	if err != nil {
		panic(err)
	}

	portInt, err := strconv.Atoi(Config.VPNPort)
	if err != nil {
		panic(err)
	}
	ip := net.ParseIP(Config.VPNIP)
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
		if clientCoreMappings[id] != nil {
			clientCoreMappings[id].FromUser <- Packet{
				addr: addr,
				data: CopySlice(buff[:n]),
			}
		} else {
			WARN("no index found:", id, addr)
		}
	}
}

func fromUserChannel(index int) {
	CM := clientCoreMappings[index]
	if CM == nil {
		return
	}

	shouldRestart := true
	defer func() {
		if r := recover(); r != nil {
			ERR(r, string(debug.Stack()))
		}

		if !shouldRestart {
			CM.Delete.Do(func() {
				NukeClient(index)
			})
		}
	}()

	var payload Packet
	var PACKET []byte
	var NIP net.IP
	var err error
	var ok bool
	staging := make([]byte, 100000)
	var D4 [4]byte
	var D4Port [2]byte
	var RST byte
	var FIN byte
	var SYN byte
	var targetCM *UserCoreMapping
	Config := Config.Load()

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
				if len(PACKET) > 11 {
					CM.CPU = PACKET[1]
					CM.RAM = PACKET[2]
					CM.Disk = PACKET[3]
					CM.PingInt.Store(int64(binary.BigEndian.Uint64(PACKET[4:])))
				}
				INFO("ping from:", index, " count:", CM.PingInt.Load())
			default:
				CM.LastPingFromClient = time.Now()
				INFO("ping from:", index)
			}
			continue
		}

		NIP = PACKET[16:20]
		if LANEnabled && (NIP[0] == 10 && NIP[1] == 0) {
			D4[0] = NIP[0]
			D4[1] = NIP[1]
			D4[2] = NIP[2]
			D4[3] = NIP[3]

			targetCM = VPLIPToCore[D4[0]][D4[1]][D4[2]][D4[3]]
			if targetCM == nil {
				CM.DelHost(D4, "auto")
				continue
			}

			l := (PACKET[0] & 0x0F) * 4
			D4Port[0] = PACKET[l+2]
			D4Port[1] = PACKET[l+3]
			RST = PACKET[l+13] & 0x4
			FIN = PACKET[l+13] & 0x1
			SYN = PACKET[l+13] & 0x2

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
	CM := clientCoreMappings[index]
	if CM == nil {
		return
	}
	shouldRestart := true

	defer func() {
		if r := recover(); r != nil {
			ERR(r, string(debug.Stack()))
		}

		if !shouldRestart {
			CM.Delete.Do(func() {
				NukeClient(index)
			})
		}
	}()

	var PACKET []byte
	var err error
	var ok bool
	var S4 [4]byte
	var S4Port [2]byte
	var FIN byte
	var RST byte
	var originCM *UserCoreMapping
	var isAdmin bool
	var headLength byte
	var activeHost *AllowedHost
	Config := Config.Load()

	for {
		PACKET, ok = <-CM.ToUser
		if !ok {
			shouldRestart = false
			return
		}

		if PACKET[9] != 6 && PACKET[9] != 17 {
			continue
		}

		// Server LAN feature is hardcoded to 10.0.X.X
		// We might change this later
		if LANEnabled && (PACKET[12] == 10 && PACKET[13] == 0) {
			originCM = VPLIPToCore[PACKET[12]][PACKET[13]][PACKET[14]][PACKET[15]]
			if !lanFirewallDisabled && !CM.DisableFirewall {
				isAdmin = false
				if originCM != nil {
					for _, entity := range Config.NetAdmins {
						if entity == originCM.DeviceToken || entity == originCM.ID {
							isAdmin = true
							break
						}
					}
				}

				if !isAdmin {

					headLength = (PACKET[0] & 0x0F) * 4
					S4Port[0] = PACKET[headLength]
					S4Port[1] = PACKET[headLength+1]

					S4[0] = PACKET[12]
					S4[1] = PACKET[13]
					S4[2] = PACKET[14]
					S4[3] = PACKET[15]

					activeHost = CM.IsHostAllowed(S4, S4Port)
					if activeHost == nil {
						continue
					}

					RST = PACKET[headLength+13] & 0x4
					FIN = PACKET[headLength+13] & 0x1
					if RST > 0 {
						CM.DelHost(S4, "auto")
					} else if FIN > 0 {
						if activeHost.FFIN {
							CM.DelHost(S4, "auto")
						} else {
							CM.SetFin(S4, S4Port, false)
						}
					}
				}
			}
		}

		err = syscall.Sendto(dataSocketFD,
			CM.EH.SEAL.Seal2(PACKET, CM.Uindex),
			0, CM.Addr)
		if err != nil {
			WARN("dataSocketFD sendTo err:", err)
			return
		}
	}
}

func createRawTCPSocket() (
	buffer []byte,
	socket *RawSocket,
	err error,
) {
	interfaceString := findInterfaceName()
	if interfaceString == "" {
		err = errors.New("no interface found")
		return
	}

	buffer = make([]byte, math.MaxUint16)
	socket = &RawSocket{
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
	socket *RawSocket,
	err error,
) {
	interfaceString := findInterfaceName()
	if interfaceString == "" {
		err = errors.New("no interface found")
		return
	}

	buffer = make([]byte, math.MaxUint16)
	socket = &RawSocket{
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

func (r *RawSocket) Create() (err error) {
	fd, sockErr := syscall.Socket(
		r.Domain,
		r.Type,
		r.Proto,
	)
	if sockErr != nil {
		syscall.Close(fd)
		return sockErr
	}

	err = syscall.SetNonblock(fd, false)
	if err != nil {
		syscall.Close(fd)
		return err
	}

	if err := syscall.SetsockoptInt(
		fd,
		syscall.SOL_SOCKET,
		syscall.SO_REUSEADDR,
		1,
	); err != nil {
		syscall.Close(fd)
		return err
	}

	sfd, sockErr := syscall.Socket(
		syscall.AF_INET,
		syscall.SOCK_RAW,
		syscall.IPPROTO_RAW,
	)
	if sockErr != nil {
		syscall.Close(fd)
		return sockErr
	}

	err = syscall.BindToDevice(fd, r.InterfaceName)
	if err != nil {
		syscall.Close(fd)
		panic(err)
	}

	addr := syscall.RawSockaddrInet4{
		Family: syscall.AF_INET,
	}

	r.RWC = &RWC{
		fd:         fd,
		fdPtr:      uintptr(fd),
		buffPtr:    uintptr(unsafe.Pointer(&r.SocketBuffer[0])),
		buffLenPtr: uintptr(len(r.SocketBuffer)),

		sfd:        sfd,
		sfdPtr:     uintptr(sfd),
		addr:       &addr,
		addrLenPtr: uintptr(0x10),
		addrPtr:    uintptr(unsafe.Pointer(&addr)),
	}

	return nil
}

type RWC struct {
	fd    int
	fdPtr uintptr

	// used for reading
	r0         uintptr
	e1         syscall.Errno
	buffPtr    uintptr
	buffLenPtr uintptr
	// msg specific reading

	// user for writing
	sfd        int
	sfdPtr     uintptr
	addr       *syscall.RawSockaddrInet4
	addrPtr    uintptr
	addrLenPtr uintptr
}

func (rwc *RWC) Read(data []byte) (n int, err error) {
	rwc.r0, _, rwc.e1 = syscall.Syscall6(
		syscall.SYS_RECVFROM,
		rwc.fdPtr,
		rwc.buffPtr,
		rwc.buffLenPtr,
		0,
		0,
		0,
	)
	n = int(rwc.r0)

	return
}

func (rwc *RWC) Write(data []byte) (n int, err error) {
	rwc.addr.Addr[0] = data[16]
	rwc.addr.Addr[1] = data[17]
	rwc.addr.Addr[2] = data[18]
	rwc.addr.Addr[3] = data[19]
	// IHL := ((data[0] << 4) >> 4) * 4
	IHL := (data[0] & 0x0F) * 4
	rwc.addr.Port = binary.BigEndian.Uint16(data[IHL+2 : IHL+4])
	_, _, e1 := syscall.Syscall6(
		syscall.SYS_SENDTO,
		rwc.sfdPtr,
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)),
		0,
		rwc.addrPtr,
		rwc.addrLenPtr,
	)
	if e1 != 0 {
		return 0, e1
	}
	return 0, nil
}

func (rwc *RWC) Close() error {
	return syscall.Close(rwc.fd)
}
