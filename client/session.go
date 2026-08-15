package client

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net"
	neturl "net/url"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/miekg/dns"
	"github.com/tunnels-is/tunnels/types"
	"github.com/xlzd/gotp"
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

	if err := authorizeControlServer(ClientCR.Server); err != nil {
		return 403, err
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

	if ClientCR.UserID != "" {
		if err := activateAccountByUserID(ClientCR.UserID); err != nil {
			ERROR("unable to activate account workspace:", err)
			return 500, errors.New("unable to activate account workspace")
		}
	}

	loadDefaultGateway()
	loadDefaultInterface()
	state := STATE.Load()
	gateway := state.DefaultGateway.Load()
	if gateway != nil {
		if isInterfaceATunnel(*gateway) {
			return 502, errors.New("default gateway is a tunnel, please retry in a moment")
		}
	} else {
		return 502, errors.New("no default gateway, check your connection settings")
	}

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

	localDev, wgCfg, wgErr := resolveLocalDeviceForServer(ClientCR, ClientCR.ServerID, meta.Tag)
	if wgErr != nil {
		ERROR("unable to resolve local device for server: ", wgErr)
		return 502, fmt.Errorf("unable to resolve local device for server: %w", wgErr)
	}
	if localDev == nil || wgCfg == nil || localDev.WireGuardPrivKey == "" {
		return 502, errors.New("no local device identity for server")
	}
	wgPrivKeyB64 := localDev.WireGuardPrivKey

	if meta.WireGuardPrivKey != "" {
		meta.WireGuardPrivKey = ""
		_ = writeTunnelsToDisk(meta.Tag)
	}

	if ClientCR.ServerIP == "" {
		ClientCR.ServerIP = wgCfg.ServerIP
	}

	if valErr := validateWGServerConfig(wgCfg.WireGuardIP, ClientCR.ServerIP,
		wgCfg.WireGuardSubnet, wgCfg.WireGuardSubnet6, wgCfg.WANCIDR); valErr != nil {
		return 502, valErr
	}

	if valErr := validateWGPort(wgCfg.WireGuardPort); valErr != nil {
		return 502, valErr
	}

	ServerReponse := &types.ServerConnectResponse{
		InterfaceIP:      wgCfg.ServerIP,
		WireGuardIP:      wgCfg.WireGuardIP,
		WireGuardPubKey:  wgCfg.WireGuardPubKey,
		WireGuardPort:    wgCfg.WireGuardPort,
		WireGuardSubnet:  wgCfg.WireGuardSubnet,
		WireGuardSubnet6: wgCfg.WireGuardSubnet6,
		WANCIDR:          wgCfg.WANCIDR,
		EnableFirewall:   wgCfg.EnableFirewall,
	}
	tunnel.ServerResponse = ServerReponse

	if !wgCfg.EnableFirewall && (len(meta.AllowedHosts) > 0 || !meta.AllowAll) {
		SECURITY("server ", ClientCR.ServerID, " has its peer firewall DISABLED — this tunnel's allowed-hosts policy is NOT enforced")
	}

	err = InitializeTunnelFromCRR(tunnel)
	if err != nil {
		return 502, err
	}

	err = IP_AddRoute(ServerReponse.InterfaceIP+"/32", *ifName, gateway.To4().String(), "0")
	if err != nil {
		return 502, errors.New("unable to initialize routes")
	}

	privHex, hexErr := wgB64ToHex(wgPrivKeyB64)
	if hexErr != nil {
		return 502, errors.New("unable to encode WireGuard private key")
	}
	serverPubHex, hexErr := wgB64ToHex(ServerReponse.WireGuardPubKey)
	if hexErr != nil {
		return 502, errors.New("unable to encode WireGuard server public key")
	}
	ipcConf := buildWGIPC(privHex, serverPubHex, ClientCR.ServerIP, ServerReponse.WireGuardPort)

	if oldTunnel != nil && wgDeviceAlive(oldTunnel.wgDevice) {
		if ipcErr := applyWGIPC(oldTunnel.wgDevice, ipcConf); ipcErr != nil {
			ERROR("in-place WireGuard IPC failed, keeping existing session: ", ipcErr)
			return 502, fmt.Errorf("WireGuard IPC configuration failed: %w", ipcErr)
		}
		oldTunnel.SetState(TUN_Disconnecting)
		tunnel.wgDevice = oldTunnel.wgDevice
		tunnel.osTUN = oldTunnel.osTUN
		tunnel.procTUN = oldTunnel.procTUN
		if tunnel.procTUN != nil {
			tunnel.procTUN.bindTunnel(tunnel)
		}
		inter := oldTunnel.tunnel.Load()
		if inter == nil {
			return 502, errors.New("existing tunnel has no interface")
		}
		inter.IPv4Address = wgCfg.WireGuardIP
		inter.Gateway = wgCfg.WireGuardIP
		inter.MTU = meta.MTU
		inter.TxQueuelen = meta.TxQueueLen
		tunnel.tunnel.Store(inter)
		inter.tunnel.Store(&tunnel)
		if err = inter.Connect(tunnel); err != nil {
			ERROR("unable to refresh tunnel interface after in-place replace: ", err)
			return 502, errors.New("unable to connect to tunnel interface")
		}
		tunnel.SetState(TUN_Connected)
		tunnel.ID = uuid.NewString()
		TunnelMap.Store(tunnel.ID, tunnel)
		go tunnel.RecordBandwidth()
		go tunnel.announceAllowedHostsWithRetry()
		watchWGDevice(tunnel)
		Disconnect(oldTunnel.ID, true)
		DEBUG("replaced WireGuard session in place (no TUN recreate)")
		return 200, nil
	}

	if oldTunnel != nil && oldTunnel.osTUN != nil && oldTunnel.osTUN.CanReuse() {
		oldTunnel.SetState(TUN_Disconnecting)
		if oldTunnel.wgDevice != nil {
			oldTunnel.wgDevice.Close()
		}
		if err := oldTunnel.osTUN.ResetForReuse(); err != nil {
			ERROR("sticky TUN reset failed: ", err)
			destroyReusablePath(oldTunnel)
		} else {
			tunnel.osTUN = oldTunnel.osTUN
			tunnel.procTUN = oldTunnel.procTUN
			if tunnel.procTUN != nil {
				tunnel.procTUN.bindTunnel(tunnel)
			}
			inter := oldTunnel.tunnel.Load()
			if inter != nil {
				inter.IPv4Address = wgCfg.WireGuardIP
				inter.Gateway = wgCfg.WireGuardIP
				inter.MTU = meta.MTU
				inter.TxQueuelen = meta.TxQueueLen
				if err := attachWGDevice(tunnel, inter, tunnel.procTUN, ipcConf); err != nil {
					ERROR("reuse TUN attach failed: ", err)
					destroyReusablePath(oldTunnel)
				} else {
					tunnel.SetState(TUN_Connected)
					tunnel.ID = uuid.NewString()
					TunnelMap.Store(tunnel.ID, tunnel)
					go tunnel.RecordBandwidth()
					go tunnel.announceAllowedHostsWithRetry()
					watchWGDevice(tunnel)
					Disconnect(oldTunnel.ID, true)
					DEBUG("reused OS TUN after WireGuard device death")
					return 200, nil
				}
			}
		}
	}

	if oldTunnel != nil {
		oldTunnel.SetState(TUN_Disconnecting)
		destroyReusablePath(oldTunnel)
	}

	osTun, tunErr := wgtun.CreateTUN(resolveTUNCreateName(meta.IFName), int(meta.MTU))
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
	pt := wrapCreatedTUN(osTun, tunnel)
	if err := attachWGDevice(tunnel, inter, pt, ipcConf); err != nil {
		if tunnel.osTUN != nil {
			_ = tunnel.osTUN.Release()
		} else {
			_ = osTun.Close()
		}
		return 502, err
	}

	tunnel.SetState(TUN_Connected)
	tunnel.ID = uuid.NewString()
	TunnelMap.Store(tunnel.ID, tunnel)

	go tunnel.RecordBandwidth()
	go tunnel.announceAllowedHostsWithRetry()
	watchWGDevice(tunnel)

	if oldTunnel != nil {
		Disconnect(oldTunnel.ID, true)
	}

	return 200, nil
}

func attachWGDevice(tunnel *TUN, inter *TInterface, pt wgtun.Device, ipcConf string) error {
	wgDev := wgdevice.NewDevice(pt, wgconn.NewDefaultBind(), NewWGLogger())
	if ipcErr := applyWGIPC(wgDev, ipcConf); ipcErr != nil {
		wgDev.Close()
		return fmt.Errorf("WireGuard IPC configuration failed: %w", ipcErr)
	}
	if upErr := wgDev.Up(); upErr != nil {
		wgDev.Close()
		return fmt.Errorf("WireGuard device Up failed: %w", upErr)
	}
	tunnel.wgDevice = wgDev
	tunnel.tunnel.Store(inter)
	inter.tunnel.Store(&tunnel)
	if err := inter.Connect(tunnel); err != nil {
		ERROR("unable to configure tunnel interface: ", err)
		wgDev.Close()
		return errors.New("unable to connect to tunnel interface")
	}
	return nil
}

type wgServerConfig struct {
	WireGuardPubKey  string `json:"WireGuardPubKey"`
	WireGuardPort    string `json:"WireGuardPort"`
	ServerIP         string `json:"ServerIP"`
	WireGuardIP      string `json:"WireGuardIP"`
	WireGuardSubnet  string `json:"WireGuardSubnet"`
	WireGuardSubnet6 string `json:"WireGuardSubnet6"`
	WANCIDR          string `json:"WANCIDR"`
	EnableFirewall   bool   `json:"EnableFirewall"`
}

func getServerWGConfig(cr *ConnectionRequest, serverID string, pubKey string) (*wgServerConfig, error) {
	url := cr.Server.GetURL("/client/wg/config") + "?serverID=" + serverID + "&pubKey=" + neturl.QueryEscape(pubKey)
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

func createServerDeviceFull(cr *ConnectionRequest, serverID string, pubKey string, tag string) (*wgServerConfig, *types.Device, error) {
	serverOID, err := uuid.Parse(serverID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid ServerID: %w", err)
	}
	if tag == "" {
		tag = DefaultTunnelName
	}

	deviceTag := tag
	if tag == DefaultTunnelName {
		deviceTag = fmt.Sprintf("tunnel-%d", time.Now().UnixNano())
	}

	url := cr.Server.GetURL("/client/device/create")
	reqBody := &createDeviceRequest{
		DeviceToken: cr.DeviceToken,
		UID:         cr.UserID,
		Device: &types.Device{
			Tag:          deviceTag,
			WireGuardKey: pubKey,
			ServerID:     serverOID,
		},
	}
	authHeaders := map[string]string{
		"X-Device-Token": cr.DeviceToken,
		"X-UID":          cr.UserID,
	}
	responseBytes, code, reqErr := SendRequestToURL(nil, "POST", url, reqBody, 15000, cr.Server.ValidateCertificate, authHeaders)
	if reqErr != nil {
		return nil, nil, fmt.Errorf("create device: %w", reqErr)
	}
	if code != 200 {
		var er ErrorResponse
		_ = json.Unmarshal(responseBytes, &er)
		if er.Error != "" {
			return nil, nil, fmt.Errorf("create device: code=%d: %s", code, er.Error)
		}
		return nil, nil, fmt.Errorf("create device: code=%d", code)
	}

	var resp createDeviceControllerResponse
	if err := json.Unmarshal(responseBytes, &resp); err != nil {
		return nil, nil, fmt.Errorf("decode create device response: %w", err)
	}
	if resp.Device == nil {
		return nil, nil, errors.New("controller returned no device")
	}

	return &wgServerConfig{
		WireGuardPubKey:  resp.ServerPubKey,
		WireGuardPort:    resp.ServerPort,
		ServerIP:         resp.ServerIP,
		WireGuardIP:      resp.Device.WireGuardIP,
		WireGuardSubnet:  resp.ServerSubnet,
		WireGuardSubnet6: resp.ServerSubnet6,
		WANCIDR:          resp.WANCIDR,
		EnableFirewall:   false,
	}, resp.Device, nil
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
		TUN.localDNSClient.Dialer.Timeout = 5 * time.Second
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
	Device        *types.Device `json:"Device"`
	ServerPubKey  string        `json:"ServerPubKey"`
	ServerPort    string        `json:"ServerPort"`
	ServerIP      string        `json:"ServerIP"`
	ServerSubnet  string        `json:"ServerSubnet"`
	ServerSubnet6 string        `json:"ServerSubnet6"`
	WANCIDR       string        `json:"WANCIDR"`
}

type createDeviceWithKeysResult struct {
	WGConfig string        `json:"WGConfig"`
	Device   *types.Device `json:"Device"`
}

func CreateDeviceWithKeys(form *CreateDeviceWithKeysForm) (any, int) {
	if err := authorizeControlServer(form.Server); err != nil {
		return &ErrorResponse{Error: err.Error()}, 403
	}

	privKey, err := generateWGPrivKey()
	if err != nil {
		return &ErrorResponse{Error: "failed to generate WireGuard private key: " + err.Error()}, 500
	}

	pubKey, err := deriveWGPubKey(privKey)
	if err != nil {
		return &ErrorResponse{Error: "failed to derive WireGuard public key: " + err.Error()}, 500
	}

	serverOID, err := uuid.Parse(form.ServerID)
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
		n, cerr := rand.Int(rand.Reader, big.NewInt(int64(len(letterRunes))))
		if cerr != nil {
			return nil, cerr
		}
		b[i] = letterRunes[n.Int64()]
	}

	TOTP := strings.ToUpper(string(b))

	authenticatorAppURL := gotp.NewDefaultTOTP(TOTP).ProvisioningUri(LF.Email, "Tunnels")

	QR = new(QR_CODE)
	QR.Value = authenticatorAppURL

	return QR, nil
}
