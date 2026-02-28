package client

import (
	"bufio"
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
	"github.com/tunnels-is/tunnels/types"
	"github.com/tunnels-is/tunnels/version"
	"github.com/xlzd/gotp"
	"go.mongodb.org/mongo-driver/bson/primitive"
	wgconn "golang.zx2c4.com/wireguard/conn"
	wgdevice "golang.zx2c4.com/wireguard/device"
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
		ERROR("pre connection check:", errm)
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
			if tun.GetState() >= TUN_NotReady {
				oldTunnel = tun
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

	if meta.ServerID != ClientCR.ServerID {
		meta.ServerID = ClientCR.ServerID
		err = writeTunnelsToDisk(meta.Tag)
		if err != nil {
			ERROR("unable to write tunnel meta to drive", err)
			return 400, errors.New("unable to write tunnel meta to drive")
		}
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

	if strings.Contains(ClientCR.Server.Host, "api.tunnels.is") {
		err = IP_AddRoute(DefaultControllerIP+"/32", *ifName, gateway.To4().String(), "0")
		if err != nil {
			return 502, errors.New("unable to initialize controller route: " + err.Error())
		}
	} else {
		netip := net.ParseIP(ClientCR.Server.Host)
		if netip == nil {
			addrs, err := net.LookupHost(ClientCR.Server.Host)
			if err != nil {
				return 502, errors.New("unable to resolve controller host: " + err.Error())
			}
			if len(addrs) == 0 {
				return 502, errors.New("did not find any addresses when resolving controller host")
			}
			err = IP_AddRoute(addrs[0]+"/32", *ifName, gateway.To4().String(), "0")
			if err != nil {
				return 502, errors.New("unable to initialize controller route: " + err.Error())
			}
		} else {
			err = IP_AddRoute(ClientCR.Server.Host+"/32", *ifName, gateway.To4().String(), "0")
			if err != nil {
				return 502, errors.New("unable to initialize controller route: " + err.Error())
			}
		}
	}

	FinalCR := new(types.ControllerConnectRequest)
	FinalCR.Created = time.Now() // The creation time will be over-written by server (we keep this to maintain compatibility with older clients)
	FinalCR.Version = version.ApiVersion
	FinalCR.UserID = UID
	FinalCR.ServerID = SID
	FinalCR.DeviceKey = ClientCR.DeviceKey
	FinalCR.DeviceToken = ClientCR.DeviceToken
	// Load or generate a persistent WireGuard keypair for this device.
	// The public key is sent to the controller which registers it against the device record.
	var wgPrivKeyB64 string
	if ClientCR.DeviceKey != "" {
		wgPrivKeyB64 = meta.WireGuardPrivKey
		if wgPrivKeyB64 == "" {
			wgPrivKeyB64, err = generateWGPrivKey()
			if err != nil {
				ERROR("unable to generate WireGuard private key: ", err)
				return 502, errors.New("unable to generate WireGuard private key")
			}
			meta.WireGuardPrivKey = wgPrivKeyB64
			if writeErr := writeTunnelsToDisk(meta.Tag); writeErr != nil {
				ERROR("unable to persist WireGuard private key: ", writeErr)
			}
		}
		wgPubKeyB64, pubErr := deriveWGPubKey(wgPrivKeyB64)
		if pubErr != nil {
			ERROR("unable to derive WireGuard public key: ", pubErr)
			return 502, errors.New("unable to derive WireGuard public key")
		}
		FinalCR.WireGuardPubKey = wgPubKeyB64
	}

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
	DEBUG("SessionResponse:", code, string(bytesFromController))

	ServerReponse := new(types.ServerConnectResponse)
	err = json.Unmarshal(bytesFromController, ServerReponse)
	if err != nil {
		ERROR("invalid response from controller", err)
		return 502, errors.New("invalid response from controller")
	}

	// Update TUN interface IP to the WG-assigned IP so egress src IPs match the
	// wg-server peer's AllowedIP entry.
	if ServerReponse.WireGuardIP != "" {
		meta.IPv4Address = ServerReponse.WireGuardIP
		if writeErr := writeTunnelsToDisk(meta.Tag); writeErr != nil {
			ERROR("unable to persist WireGuard IP: ", writeErr)
		}
	}

	DEBUG("ConnectionRequestResponse:", ServerReponse)
	tunnel.ServerResponse = ServerReponse

	err = InitializeTunnelFromCRR(tunnel)
	if err != nil {
		return 502, err
	}

	err = IP_AddRoute(ServerReponse.InterfaceIP+"/32", *ifName, gateway.To4().String(), "0")
	if err != nil {
		return 502, errors.New("unable to initialize routes")
	}

	// WireGuard transport: create userspace WG device backed by chanTUN.
	ct := newChanTUN(wgdevice.DefaultMTU)
	wgDev := wgdevice.NewDevice(ct, wgconn.NewDefaultBind(),
		wgdevice.NewLogger(wgdevice.LogLevelError, "[wg-client] "))
	privHex, hexErr := wgB64ToHex(wgPrivKeyB64)
	if hexErr != nil {
		wgDev.Close()
		return 502, errors.New("unable to encode WireGuard private key")
	}
	serverPubHex, hexErr := wgB64ToHex(ServerReponse.WireGuardPubKey)
	if hexErr != nil {
		wgDev.Close()
		return 502, errors.New("unable to encode WireGuard server public key")
	}
	ipcConf := fmt.Sprintf(
		"private_key=%s\npublic_key=%s\nendpoint=%s:%s\nallowed_ip=0.0.0.0/0\npersistent_keepalive_interval=25\n\n",
		privHex, serverPubHex, ClientCR.ServerIP, ServerReponse.WireGuardPort,
	)
	if ipcErr := wgDev.IpcSetOperation(bufio.NewReader(strings.NewReader(ipcConf))); ipcErr != nil {
		wgDev.Close()
		return 502, fmt.Errorf("WireGuard IPC configuration failed: %w", ipcErr)
	}
	if upErr := wgDev.Up(); upErr != nil {
		wgDev.Close()
		return 502, fmt.Errorf("WireGuard device Up failed: %w", upErr)
	}
	tunnel.wgDevice = wgDev
	tunnel.wgTun = ct

	var inter *TInterface
	if oldTunnel != nil {
		inter = oldTunnel.tunnel.Load()
		// Stop old goroutines before reconfiguring the interface.
		// On Windows, the wintun session must be ended before a new
		// one can be started on the same adapter.
		oldTunnel.SetState(TUN_Disconnecting)
		if oldTunnel.wgDevice != nil {
			oldTunnel.wgDevice.Close()
		}
		inter.PrepareForSwitch()
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

	go tunnel.ReadFromServeTunnel()
	go tunnel.ReadFromTunnelInterface()
	go tunnel.RecordBandwidth()

	if oldTunnel != nil {
		Disconnect(oldTunnel.ID, true)
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

	TUN.InitBlockedPorts(TUN.meta.Load().BlockedPorts)

	err = TUN.InitNatMaps()
	if err != nil {
		return err
	}

	DEBUG(fmt.Sprintf(
		"Connection info: Addr(%s) srcIP(%s)",
		meta.IPv4Address,
		TUN.ServerResponse.InterfaceIP,
	))

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
