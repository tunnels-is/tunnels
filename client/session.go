package client

import (
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/miekg/dns"
	"github.com/tunnels-is/tunnels/crypt"
	"github.com/tunnels-is/tunnels/types"
	"github.com/tunnels-is/tunnels/version"
	"github.com/xlzd/gotp"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// PreConnectCheck validates system state before connecting
func PreConnectCheck(meta *TunnelMETA) (int, error) {
	s := STATE.Load()
	if !s.adminState {
		return 400, errors.New("tunnels does not have the correct access permissions")
	}
	return 0, nil
}

var IsConnecting = atomic.Bool{}

// PublicConnect establishes a VPN connection to a server
func PublicConnect(ClientCR *ConnectionRequest) (code int, errm error) {
	if ClientCR.ServerID == "" {
		ERROR("No Server id found when connecting: ", ClientCR)
		return 400, errors.New("no server id found when connecting")
	}

	if !IsConnecting.CompareAndSwap(false, true) {
		INFO("Already connecting to another connection, please wait a moment")
		return 400, errors.New("Already connecting to another connection, please wait a moment")
	}

	start := time.Now()
	defer func() {
		IsConnecting.Store(false)
		DEBUG("Session creation finished in: ", fmt.Sprintf("%.0f", math.Abs(time.Since(start).Seconds())), " seconds")
		runtime.GC()
	}()
	defer RecoverAndLog()

	state := STATE.Load()
	loadDefaultGateway()
	loadDefaultInterface()

	// Fallback on the default tunnel if non is given
	if ClientCR.Tag == "" {
		ClientCR.Tag = DefaultTunnelName
	}

	var meta *TunnelMETA
	tunnelMetaMapRange(func(tun *TunnelMETA) bool {
		if tun.Tag == DefaultTunnelName && ClientCR.Tag == DefaultTunnelName {
			meta = tun
			_ = writeTunnelsToDisk(DefaultTunnelName)
			return false
		} else if tun.Tag == ClientCR.Tag {
			meta = tun
			return false
		}
		return true
	})

	if meta == nil {
		ERROR("vpn connection metadata not found for tag: ", ClientCR.Tag)
		return 400, errors.New("error fetching connection meta")
	}

	code, errm = PreConnectCheck(meta)
	if errm != nil {
		return code, errm
	}

	// isConnected := false
	var oldTunnel *TUN
	tunnelMapRange(func(tun *TUN) bool {
		m := tun.meta.Load()
		if m == nil {
			return true
		}
		if m.Tag == meta.Tag {
			if tun.GetState() >= TUN_Connected {
				oldTunnel = tun
				// isConnected = true
			}
			return false
		}

		return true
	})

	tunnel := new(TUN)
	tunnel.meta.Store(meta)
	tunnel.CR = ClientCR

	if ClientCR.ServerIP == "" {
		server, err := getServerByID(
			ClientCR.Server,
			ClientCR.DeviceKey,
			ClientCR.DeviceToken,
			ClientCR.UserID,
			ClientCR.ServerID,
		)
		if err != nil {
			ERROR("Error finding server", err)
			return 400, err
		}

		ClientCR.ServerPort = server.Port
		ClientCR.ServerIP = server.IP
		ClientCR.ServerPubKey = server.PubKey
	}

	if ClientCR.ServerIP == "" {
		ERROR("No Server IPAddress found when connecting: ", ClientCR)
		return 400, errors.New("no ip address found when connecting")
	}
	if ClientCR.ServerPort == "" {
		ERROR("No Server Port found when connecting: ", ClientCR)
		return 400, errors.New("no server port found when connecting")
	}

	if ClientCR.DeviceKey != "" {
		ClientCR.UserID = ClientCR.DeviceKey
	}
	UID, err := primitive.ObjectIDFromHex(ClientCR.UserID)
	if err != nil {
		ERROR("Invalid user ID")
		return 400, errors.New("Invalid user ID")
	}
	SID, err := primitive.ObjectIDFromHex(ClientCR.ServerID)
	if err != nil {
		ERROR("Invalid Server ID")
		return 400, errors.New("Invalid Server ID")
	}

	FinalCR := new(types.ControllerConnectRequest)
	FinalCR.Created = time.Now() // The creation time will be over-written by server (we keep this to maintain compatibility with older clients)
	FinalCR.Version = version.ApiVersion
	FinalCR.UserID = UID
	FinalCR.ServerID = SID
	FinalCR.DeviceKey = ClientCR.DeviceKey
	FinalCR.DeviceToken = ClientCR.DeviceToken
	FinalCR.EncType = meta.EncryptionType
	FinalCR.RequestingPorts = meta.RequestVPNPorts
	DEBUG("ConnectRequestFromClient", ClientCR)

	url := ClientCR.Server.GetURL("/v3/session")
	bytesFromController, code, err := SendRequestToURL(
		nil,
		"POST",
		url,
		FinalCR,
		10000,
		ClientCR.Server.ValidateCertificate,
	)
	if code != 200 {
		ERROR("ErrFromController:", err, string(bytesFromController))
		ER := new(ErrorResponse)
		err := json.Unmarshal(bytesFromController, ER)
		if err == nil {
			return code, errors.New(ER.Error)
		} else {
			return code, errors.New("Error code from controller:" + strconv.Itoa(code))
		}
	}
	if err != nil {
		return 500, errors.New("Unknown when contacting controller")
	}
	DEBUG("SignedPayload:", code, string(bytesFromController))

	SignedResponse := new(types.SignedConnectRequest)
	err = json.Unmarshal(bytesFromController, SignedResponse)
	if err != nil {
		ERROR("invalid signed response from controller", err)
		return 502, errors.New("invalid response from controller")
	}

	tunnel.encWrapper = crypt.NewEncryptionHandler(meta.EncryptionType)
	err = tunnel.encWrapper.InitializeClient()
	if err != nil {
		ERROR("unable to create encryption handler: ", err)
		return 502, errors.New("Unable to secure connection")
	}
	SignedResponse.X25519PeerPub = tunnel.encWrapper.SEAL.X25519Pub.Bytes()
	SignedResponse.Mlkem1024Encap = tunnel.encWrapper.SEAL.Mlkem1024Encap.Bytes()

	tc := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		CurvePreferences:   []tls.CurveID{tls.X25519MLKEM768},
		InsecureSkipVerify: !ClientCR.Server.ValidateCertificate,
	}
	tc.RootCAs, errm = tunnel.LoadCertPEMBytes([]byte(ClientCR.ServerPubKey))
	if errm != nil {
		ERROR("Unable to load cert pem from controller: ", errm)
		return 502, errors.New("Unable to load cert pem from controller")
	}
	bytesFromServer, code, err := SendRequestToURL(
		tc,
		"POST",
		"https://"+ClientCR.ServerIP+":"+ClientCR.ServerPort+"/v3/connect",
		SignedResponse,
		10000,
		ClientCR.Server.ValidateCertificate,
	)
	if code != 200 {
		ERROR("ErrFromServer:", code, string(bytesFromServer))
		ER := new(ErrorResponse)
		err := json.Unmarshal(bytesFromServer, ER)
		if err == nil {
			return code, errors.New(ER.Error)
		} else {
			return code, errors.New("Error code from vpn server: " + strconv.Itoa(code))
		}
	}
	if err != nil {
		return 500, errors.New("Unknown when contacting controller")
	}

	ServerReponse := new(types.ServerConnectResponse)
	err = json.Unmarshal(bytesFromServer, ServerReponse)
	if err != nil {
		return 500, errors.New("Unable to decode response from server")
	}

	pubKey, _, err := crypt.LoadPublicKeyBytes([]byte(ClientCR.ServerPubKey))
	if err != nil {
		return 500, errors.New("Unable to load server public key")
	}

	err = crypt.VerifySignature(ServerReponse.X25519Pub, ServerReponse.ServerHandshakeSignature, pubKey)
	if err != nil {
		return 500, errors.New("Unable to verify server signature")
	}

	err = tunnel.encWrapper.FinalizeClient(ServerReponse.X25519Pub, ServerReponse.Mlkem1024Cipher)
	if err != nil {
		return 500, errors.New("Unable to create encryption wrapper seal")
	}

	// clear out handshake data
	SignedResponse.X25519PeerPub = nil
	SignedResponse.Mlkem1024Encap = nil
	ServerReponse.X25519Pub = nil
	ServerReponse.Mlkem1024Cipher = nil
	ServerReponse.ServerHandshakeSignature = nil
	tunnel.encWrapper.SEAL.CleanPostSecretGeneration()

	DEBUG("ConnectionRequestResponse:", ServerReponse)
	tunnel.ServerResponse = ServerReponse

	err = InitializeTunnelFromCRR(tunnel)
	if err != nil {
		return 502, err
	}

	// ensure gateway is not incorrect
	gateway := state.DefaultGateway.Load()
	if gateway != nil {
		if isInterfaceATunnel(*gateway) {
			return 502, errors.New("default gateway is a tunnel, please retry in a moment")
		}
	} else {
		return 502, errors.New("no default gateway, check your connection settings")
	}

	ifName := state.DefaultInterfaceName.Load()
	if ifName == nil {
		return 502, errors.New("no default interface, please check try again")
	}
	err = IP_AddRoute(ServerReponse.InterfaceIP+"/32", *ifName, gateway.To4().String(), "0")
	if err != nil {
		return 502, errors.New("unable to initialize routes")
	}

	raddr, err := net.ResolveUDPAddr("udp4", ServerReponse.InterfaceIP+":"+ServerReponse.DataPort)
	if err != nil {
		return 502, errors.New("unable to resolve data port upd route")
	}

	UDPConn, err := net.DialUDP("udp4", nil, raddr)
	if err != nil {
		DEBUG("Unable to open data tunnel: ", err)
		return 502, errors.New("unable to open data tunnel")
	}
	// EXPERIMENTAL
	// err = setDontFragment(UDPConn)
	// if err != nil {
	// 	DEBUG("unable to disable IP fragmentation", err)
	// }
	tunnel.connection = net.Conn(UDPConn)

	var inter *TInterface
	if oldTunnel != nil {
		inter = oldTunnel.tunnel.Load()
	} else {
		inter, err = CreateAndConnectToInterface(tunnel)
	}
	if err != nil {
		ERROR("Unable to initialize interface: ", err)
		return 502, err
	}

	tunnel.tunnel.Store(inter)
	inter.tunnel.Store(&tunnel)
	err = inter.Connect(tunnel)
	if err != nil {
		ERROR("unable to configure tunnel interface: ", err)
		return 502, errors.New("Unable to connect to tunnel interface")
	}

	tunnel.SetState(TUN_Connected)
	tunnel.registerPing(time.Now())
	tunnel.ID = uuid.NewString()
	TunnelMap.Store(tunnel.ID, tunnel)

	_, err = tunnel.connection.Write(
		tunnel.encWrapper.SEAL.Seal1(PingPongStatsBuffer, tunnel.Index),
	)
	if err != nil {
		return 502, errors.New("unable to send ping to server")
	}

	go tunnel.ReadFromServeTunnel()
	go tunnel.ReadFromTunnelInterface()

	if tunnel.ServerResponse.DHCP != nil {
		FR := &FirewallRequest{
			DHCPToken:       tunnel.dhcp.Token,
			IP:              net.IP(tunnel.dhcp.IP[:]).String(),
			Hosts:           meta.AllowedHosts,
			DisableFirewall: meta.DisableFirewall,
		}
		_, code, err := SendRequestToURL(
			tc,
			"POST",
			"https://"+ClientCR.ServerIP+":"+ClientCR.ServerPort+"/v3/firewall",
			FR,
			10000,
			ClientCR.Server.ValidateCertificate,
		)
		if err != nil {
			ERROR("unable to update firewall: ", err)
		} else if code != 200 {
			ERROR("unable to update firewall: ", code)
		} else {
			DEBUG("firewall updated")
		}
	}

	if oldTunnel != nil {
		Disconnect(oldTunnel.ID, true)
		// oldTunnel.SetState(TUN_Disconnected)
		// oldTunnel.connection.Close()
		// TunnelMap.Delete(oldTunnel.ID)
	}

	return 200, nil
}

// getServerByID retrieves server information from the controller
func getServerByID(server *ControlServer, deviceKey string, deviceToken string, UserID string, ServerID string) (s *types.Server, err error) {
	SID, _ := primitive.ObjectIDFromHex(ServerID)
	UID, _ := primitive.ObjectIDFromHex(UserID)

	FR := &FORWARD_REQUEST{
		Server:  server,
		Path:    "/v3/server",
		Method:  "POST",
		Timeout: 10000,
		JSONData: &types.FORM_GET_SERVER{
			DeviceToken: deviceToken,
			DeviceKey:   deviceKey,
			UID:         UID,
			ServerID:    SID,
		},
	}
	url := FR.Server.GetURL(FR.Path)
	responseBytes, code, err := SendRequestToURL(
		nil,
		FR.Method,
		url,
		FR.JSONData,
		FR.Timeout,
		FR.Server.ValidateCertificate,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", "error calling controller", err)
	}
	if code != 200 {
		return nil, fmt.Errorf("%s: %d", "invalid code from controller", code)
	}

	s = new(types.Server)
	err = json.Unmarshal(responseBytes, s)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", "invalid response from controller", err)
	}
	return
}

// GetDeviceByID retrieves device information from the controller
func GetDeviceByID(server *ControlServer, deviceID string) (d *types.Device, err error) {
	DID, _ := primitive.ObjectIDFromHex(deviceID)

	FR := &FORWARD_REQUEST{
		Server:  server,
		Path:    "/v3/device",
		Method:  "POST",
		Timeout: 10000,
		JSONData: &types.FORM_GET_DEVICE{
			DeviceID: DID,
		},
	}
	url := FR.Server.GetURL(FR.Path)
	responseBytes, code, err := SendRequestToURL(
		nil,
		FR.Method,
		url,
		FR.JSONData,
		FR.Timeout,
		FR.Server.ValidateCertificate,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", "error calling controller", err)
	}
	if code != 200 {
		return nil, fmt.Errorf("%s: %d", "invalid code from controller", code)
	}

	d = new(types.Device)
	err = json.Unmarshal(responseBytes, d)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", "invalid response from controller", err)
	}
	return
}

// InitializeTunnelFromCRR initializes tunnel state from connection response
func InitializeTunnelFromCRR(TUN *TUN) (err error) {
	DNSGlobalBlock.Store(true)
	defer func() {
		RecoverAndLog()
		DNSGlobalBlock.Store(false)
	}()
	go FullCleanDNSCache()

	meta := TUN.meta.Load()

	// This index is used to identify packet streams between server and user.
	TUN.Index = make([]byte, 2)
	binary.BigEndian.PutUint16(TUN.Index, uint16(TUN.ServerResponse.Index))

	TUN.localInterfaceNetIP = net.ParseIP(meta.IPv4Address).To4()
	if TUN.localInterfaceNetIP == nil {
		return fmt.Errorf("Interface ip (%s) was malformed", meta.IPv4Address)
	}
	TUN.localInterfaceIP4bytes[0] = TUN.localInterfaceNetIP[0]
	TUN.localInterfaceIP4bytes[1] = TUN.localInterfaceNetIP[1]
	TUN.localInterfaceIP4bytes[2] = TUN.localInterfaceNetIP[2]
	TUN.localInterfaceIP4bytes[3] = TUN.localInterfaceNetIP[3]

	if DNSClient.Dialer != nil {
		TUN.localDNSClient = new(dns.Client)
		TUN.localDNSClient.Dialer = new(net.Dialer)
		TUN.localDNSClient.Dialer.LocalAddr = &net.UDPAddr{
			IP: TUN.localInterfaceNetIP.To4(),
		}
		TUN.localDNSClient.Dialer.Resolver = DNSClient.Dialer.Resolver
		TUN.localDNSClient.Dialer.Timeout = time.Duration(5 * time.Second)
		TUN.localDNSClient.Timeout = time.Second * 5
	}

	TUN.serverInterfaceNetIP = net.ParseIP(TUN.ServerResponse.InterfaceIP).To4()
	if TUN.serverInterfaceNetIP == nil {
		return fmt.Errorf("Interface ip (%s) was malformed", TUN.ServerResponse.InterfaceIP)
	}

	TUN.serverInterfaceIP4bytes[0] = TUN.serverInterfaceNetIP[0]
	TUN.serverInterfaceIP4bytes[1] = TUN.serverInterfaceNetIP[1]
	TUN.serverInterfaceIP4bytes[2] = TUN.serverInterfaceNetIP[2]
	TUN.serverInterfaceIP4bytes[3] = TUN.serverInterfaceNetIP[3]

	if TUN.ServerResponse.DHCP != nil {
		TUN.serverVPLIP[0] = TUN.ServerResponse.DHCP.IP[0]
		TUN.serverVPLIP[1] = TUN.ServerResponse.DHCP.IP[1]
		TUN.serverVPLIP[2] = TUN.ServerResponse.DHCP.IP[2]
		TUN.serverVPLIP[3] = TUN.ServerResponse.DHCP.IP[3]
		TUN.dhcp = TUN.ServerResponse.DHCP
	}

	if TUN.ServerResponse.LAN != nil {
		TUN.VPLNetwork = TUN.ServerResponse.LAN
	}

	if meta.LocalhostNat {
		NN := new(types.Network)
		NN.Network = "127.0.0.1/32"
		NN.Nat = TUN.serverInterfaceNetIP.String() + "/32"
		TUN.ServerResponse.Networks = append(TUN.ServerResponse.Networks, NN)
	}

	if len(meta.Networks) > 0 {
		TUN.ServerResponse.Networks = meta.Networks
	}
	if len(meta.Routes) > 0 {
		TUN.ServerResponse.Routes = meta.Routes
	}
	if len(meta.DNSRecords) > 0 {
		TUN.ServerResponse.DNSRecords = meta.DNSRecords
	}
	if len(meta.DNSServers) > 0 {
		TUN.ServerResponse.DNSServers = meta.DNSServers
	}

	conf := CONFIG.Load()
	if len(TUN.ServerResponse.DNSServers) < 1 {
		TUN.ServerResponse.DNSServers = []string{conf.DNS1Default, conf.DNS2Default}
	}

	TUN.startPort = TUN.ServerResponse.StartPort
	TUN.endPort = TUN.ServerResponse.EndPort
	TUN.InitPortMap()

	err = TUN.InitVPLMap()
	if err != nil {
		return err
	}
	err = TUN.InitNatMaps()
	if err != nil {
		return err
	}

	DEBUG(fmt.Sprintf(
		"Connection info: Addr(%s) StartPort(%d) EndPort(%d) srcIP(%s) ",
		meta.IPv4Address,
		TUN.ServerResponse.StartPort,
		TUN.ServerResponse.EndPort,
		TUN.ServerResponse.InterfaceIP,
	))

	if TUN.ServerResponse.LAN != nil && TUN.ServerResponse.DHCP != nil {
		DEBUG(fmt.Sprintf(
			"DHCP/VPL info: Addr(%s) Network:(%s)",
			TUN.ServerResponse.DHCP.IP,
			TUN.ServerResponse.LAN.Network,
		))
	}

	return nil
}

// GetQRCode generates a TOTP QR code for 2FA
func GetQRCode(LF *TWO_FACTOR_CONFIRM) (QR *QR_CODE, err error) {
	if LF.Email == "" {
		return nil, errors.New("email missing")
	}

	b := make([]rune, 16)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}

	TOTP := strings.ToUpper(string(b))

	authenticatorAppURL := gotp.NewDefaultTOTP(TOTP).ProvisioningUri(LF.Email, "Tunnels")

	QR = new(QR_CODE)
	QR.Value = authenticatorAppURL

	return QR, nil
}
