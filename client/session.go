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
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/miekg/dns"
	"github.com/tunnels-is/tunnels/types"
	"github.com/xlzd/gotp"
	"go.mongodb.org/mongo-driver/bson/primitive"
	wgconn "golang.zx2c4.com/wireguard/conn"
	wgdevice "golang.zx2c4.com/wireguard/device"
	wgtun "golang.zx2c4.com/wireguard/tun"
)

func PreConnectCheck(meta *TunnelMETA) (int, error) {
	s := STATE.Load()
	if !s.adminState {
		return 400, errors.New("tunnels does not have the correct access permissions")
	}
	return 0, nil
}

var IsConnecting = atomic.Bool{}

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

	var err error

	if meta.ServerID != ClientCR.ServerID {
		meta.ServerID = ClientCR.ServerID
		err = writeTunnelsToDisk(meta.Tag)
		if err != nil {
			ERROR("unable to write tunnel meta to drive", err)
			return 400, errors.New("unable to write tunnel meta to drive")
		}
	}

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

	wgPrivKeyB64 := meta.WireGuardPrivKey
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

	pubKey, pubErr := deriveWGPubKey(wgPrivKeyB64)
	if pubErr != nil {
		return 502, errors.New("unable to derive WireGuard public key")
	}

	wgCfg, wgErr := getServerWGConfig(ClientCR, ClientCR.ServerID, pubKey)
	if wgErr != nil {
		ERROR("unable to fetch server WireGuard config: ", wgErr)
		return 502, fmt.Errorf("unable to fetch server WireGuard config: %w", wgErr)
	}
	if wgCfg.WireGuardIP == "" {
		return 400, errors.New("no WireGuard IP assigned; create the device on the controller first")
	}
	if ClientCR.ServerIP == "" {
		ClientCR.ServerIP = wgCfg.ServerIP
	}

	ServerReponse := &types.ServerConnectResponse{
		InterfaceIP:     wgCfg.ServerIP,
		WireGuardIP:     wgCfg.WireGuardIP,
		WireGuardPubKey: wgCfg.WireGuardPubKey,
		WireGuardPort:   wgCfg.WireGuardPort,
	}
	tunnel.ServerResponse = ServerReponse

	err = InitializeTunnelFromCRR(tunnel)
	if err != nil {
		return 502, err
	}

	err = IP_AddRoute(ServerReponse.InterfaceIP+"/32", *ifName, gateway.To4().String(), "0")
	if err != nil {
		return 502, errors.New("unable to initialize routes")
	}

	if oldTunnel != nil {
		oldTunnel.SetState(TUN_Disconnecting)
		if oldTunnel.wgDevice != nil {
			oldTunnel.wgDevice.Close()
		}
	}

	osTun, tunErr := wgtun.CreateTUN(meta.IFName, int(meta.MTU))
	if tunErr != nil {
		return 502, fmt.Errorf("unable to create TUN interface: %w", tunErr)
	}
	tunIfName, _ := osTun.Name()

	inter := &TInterface{
		Name:        tunIfName,
		IPv4Address: wgCfg.WireGuardIP,
		NetMask:     "255.255.255.255",
		MTU:         meta.MTU,
		TxQueuelen:  meta.TxQueueLen,
		Gateway:     wgCfg.WireGuardIP,
	}

	pt := newProcessingTUN(osTun, tunnel)
	wgDev := wgdevice.NewDevice(pt, wgconn.NewDefaultBind(),
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

	tunnel.tunnel.Store(inter)
	inter.tunnel.Store(&tunnel)
	err = inter.Connect(tunnel)
	if err != nil {
		ERROR("unable to configure tunnel interface: ", err)
		wgDev.Close()
		return 502, errors.New("unable to connect to tunnel interface")
	}

	tunnel.SetState(TUN_Connected)
	tunnel.registerPing(time.Now())
	tunnel.ID = uuid.NewString()
	TunnelMap.Store(tunnel.ID, tunnel)

	go tunnel.RecordBandwidth()
	go func() {
		defer RecoverAndLog()
		tunnel.wgDevice.Wait()
		m := tunnel.meta.Load()
		DEBUG("WireGuard device closed:", m.Tag, tunnel.ID)
		if tunnel.GetState() >= TUN_Connected {
			tunnelMonitor <- tunnel
		}
	}()

	if oldTunnel != nil {
		Disconnect(oldTunnel.ID, true)
	}

	return 200, nil
}

type wgServerConfig struct {
	WireGuardPubKey string `json:"WireGuardPubKey"`
	WireGuardPort   string `json:"WireGuardPort"`
	ServerIP        string `json:"ServerIP"`
	WireGuardIP     string `json:"WireGuardIP"`
}

func getServerWGConfig(cr *ConnectionRequest, serverID string, pubKey string) (*wgServerConfig, error) {
	url := cr.Server.GetURL("/client/wg/config") + "?serverID=" + serverID + "&pubKey=" + pubKey
	authHeaders := map[string]string{
		"X-Device-Token": cr.DeviceToken,
		"X-UID":          cr.UserID,
	}
	responseBytes, code, err := SendRequestToURL(nil, "GET", url, nil, 10000, cr.Server.ValidateCertificate, authHeaders)
	if err != nil {
		return nil, fmt.Errorf("get wg config: %w", err)
	}
	if code != 200 {
		return nil, fmt.Errorf("get wg config: code=%d", code)
	}
	cfg := new(wgServerConfig)
	if err := json.Unmarshal(responseBytes, cfg); err != nil {
		return nil, fmt.Errorf("decode wg config: %w", err)
	}
	return cfg, nil
}

func GetDeviceByID(server *ControlServer, deviceID string) (d *types.Device, err error) {
	DID, _ := primitive.ObjectIDFromHex(deviceID)

	FR := &FORWARD_REQUEST{
		Server:  server,
		Path:    "/client/device",
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

func InitializeTunnelFromCRR(TUN *TUN) (err error) {
	DNSGlobalBlock.Store(true)
	defer func() {
		RecoverAndLog()
		DNSGlobalBlock.Store(false)
	}()
	go FullCleanDNSCache()

	meta := TUN.meta.Load()

	TUN.localInterfaceNetIP = net.ParseIP(TUN.ServerResponse.WireGuardIP).To4()
	if TUN.localInterfaceNetIP == nil {
		return fmt.Errorf("Interface ip (%s) was malformed", TUN.ServerResponse.WireGuardIP)
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
		TUN.ServerResponse.WireGuardIP,
		TUN.ServerResponse.InterfaceIP,
	))

	return nil
}

type createDeviceRequest struct {
	DeviceToken string        `json:"DeviceToken"`
	UID         string        `json:"UID"`
	Device      *types.Device `json:"Device"`
}

type createDeviceControllerResponse struct {
	Device       *types.Device `json:"Device"`
	ServerPubKey string        `json:"ServerPubKey"`
	ServerPort   string        `json:"ServerPort"`
	ServerIP     string        `json:"ServerIP"`
}

type createDeviceWithKeysResult struct {
	WGConfig string        `json:"WGConfig"`
	Device   *types.Device `json:"Device"`
}

func CreateDeviceWithKeys(form *CreateDeviceWithKeysForm) (any, int) {
	privKey, err := generateWGPrivKey()
	if err != nil {
		return &ErrorResponse{Error: "failed to generate WireGuard private key: " + err.Error()}, 500
	}

	pubKey, err := deriveWGPubKey(privKey)
	if err != nil {
		return &ErrorResponse{Error: "failed to derive WireGuard public key: " + err.Error()}, 500
	}

	serverOID, err := primitive.ObjectIDFromHex(form.ServerID)
	if err != nil {
		return &ErrorResponse{Error: "invalid ServerID: " + err.Error()}, 400
	}

	url := form.Server.GetURL("/client/device/create")
	reqBody := &createDeviceRequest{
		DeviceToken: form.DeviceToken,
		UID:         form.UID,
		Device: &types.Device{
			Tag:          form.Tag,
			WireGuardKey: pubKey,
			ServerID:     serverOID,
		},
	}

	authHeaders := map[string]string{
		"X-Device-Token": form.DeviceToken,
		"X-UID":          form.UID,
	}
	responseBytes, code, reqErr := SendRequestToURL(nil, "POST", url, reqBody, 15000, form.Server.ValidateCertificate, authHeaders)
	if reqErr != nil {
		return &ErrorResponse{Error: "controller request failed: " + reqErr.Error()}, 500
	}
	if code != 200 {
		var er ErrorResponse
		_ = json.Unmarshal(responseBytes, &er)
		if er.Error == "" {
			er.Error = fmt.Sprintf("controller returned status %d", code)
		}
		return &er, code
	}

	var resp createDeviceControllerResponse
	if err := json.Unmarshal(responseBytes, &resp); err != nil {
		return &ErrorResponse{Error: "failed to parse controller response: " + err.Error()}, 500
	}

	if resp.Device == nil {
		return &ErrorResponse{Error: "controller returned no device"}, 500
	}

	wgConfig := fmt.Sprintf(
		"[Interface]\nPrivateKey = %s\nAddress = %s/32\nDNS = 1.1.1.1\n\n[Peer]\nPublicKey = %s\nEndpoint = %s:%s\nAllowedIPs = 0.0.0.0/0\nPersistentKeepalive = 25\n",
		privKey,
		resp.Device.WireGuardIP,
		resp.ServerPubKey,
		resp.ServerIP,
		resp.ServerPort,
	)

	return &createDeviceWithKeysResult{
		WGConfig: wgConfig,
		Device:   resp.Device,
	}, 200
}

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
