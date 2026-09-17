package client

import (
	"errors"
	"fmt"
	"math"
	"net"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/miekg/dns"
	"github.com/tunnels-is/tunnels/types"
	wgtun "golang.zx2c4.com/wireguard/tun"
)

func PreConnectCheck(meta *TunnelMeta) (int, error) {
	s := STATE.Load()
	if !s.adminState {
		return 400, errors.New("tunnels does not have the correct access permissions")
	}
	return 0, nil
}

var IsConnecting = atomic.Bool{}

func lookupTunnelMeta(tag string) (meta *TunnelMeta) {
	tunnelMetaMapRange(func(tun *TunnelMeta) bool {
		if tun.Tag == DefaultTunnelName && tag == DefaultTunnelName {
			meta = tun
			return false
		} else if tun.Tag == tag {
			meta = tun
			return false
		}
		return true
	})
	return meta
}

func findReplaceableTunnel(tag string) (oldTunnel *TUN) {
	tunnelMapRange(func(tun *TUN) bool {
		m := tun.meta.Load()
		if m == nil {
			return true
		}
		if m.Tag == tag {
			if tun.GetState() >= TunnelNotReady {
				oldTunnel = tun
			}
			return false
		}

		return true
	})
	return oldTunnel
}

func pinControllerHostRoute(cr *ConnectionRequest, tunnel *TUN, ifName string, gw4 string) error {
	if isOfficialControllerHost(cr.Server.Host) {
		err := addIPv4Route(DefaultControllerIP+"/32", ifName, gw4, "0")
		if err != nil {
			return errors.New("unable to initialize controller route: " + err.Error())
		}
		registerProtectHost(tunnel, DefaultControllerIP)
	} else {
		netip := net.ParseIP(cr.Server.Host)
		if netip == nil {
			addrs, err := net.LookupHost(cr.Server.Host)
			if err != nil {
				return errors.New("unable to resolve controller host: " + err.Error())
			}
			if len(addrs) == 0 {
				return errors.New("did not find any addresses when resolving controller host")
			}
			err = addIPv4Route(addrs[0]+"/32", ifName, gw4, "0")
			if err != nil {
				return errors.New("unable to initialize controller route: " + err.Error())
			}
			registerProtectHost(tunnel, addrs[0])
		} else {
			err := addIPv4Route(cr.Server.Host+"/32", ifName, gw4, "0")
			if err != nil {
				return errors.New("unable to initialize controller route: " + err.Error())
			}
			registerProtectHost(tunnel, cr.Server.Host)
		}
	}
	return nil
}

func startLiveSession(tunnel *TUN) {
	tunnel.SetState(TunnelConnected)
	tunnel.ID = uuid.NewString()
	TunnelMap.Store(tunnel.ID, tunnel)
	go tunnel.RecordBandwidth()
	go tunnel.announceAllowedHostsWithRetry()
	watchWGDevice(tunnel)
}

func PublicConnect(cr *ConnectionRequest) (code int, errm error) {
	if cr.ServerID == "" {
		ERROR("No Server id found when connecting: ", cr)
		return 400, errors.New("no server id found when connecting")
	}

	if err := authorizeControlServer(cr.Server); err != nil {
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

	if cr.UserID != "" {
		if err := activateAccountByUserID(cr.UserID); err != nil {
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

	if cr.Tag == "" {
		cr.Tag = DefaultTunnelName
	}

	meta := lookupTunnelMeta(cr.Tag)

	if meta == nil {
		ERROR("vpn connection metadata not found for tag: ", cr.Tag)
		return 400, errors.New("error fetching connection meta")
	}

	if err := persistTunnelServerID(meta, cr.ServerID); err != nil {
		ERROR("unable to write tunnel meta to drive", err)
		return 400, errors.New("unable to write tunnel meta to drive")
	}

	code, errm = PreConnectCheck(meta)
	if errm != nil {
		ERROR("pre connection check:", errm)
		return code, errm
	}

	oldTunnel := findReplaceableTunnel(meta.Tag)

	tunnel := new(TUN)
	tunnel.meta.Store(meta)
	tunnel.CR = cr

	var err error

	ifName := state.DefaultInterfaceName.Load()
	if ifName == nil {
		return 502, errors.New("no default interface, please check try again")
	}

	gw4 := gateway.To4().String()
	err = pinControllerHostRoute(cr, tunnel, *ifName, gw4)
	if err != nil {
		return 502, err
	}

	localDev, wgCfg, wgErr := resolveLocalDeviceForServer(cr, cr.ServerID, meta.Tag)
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

	if cr.ServerIP == "" {
		cr.ServerIP = wgCfg.ServerIP
	}

	if valErr := validateWGServerConfig(wgCfg.WireGuardIP, cr.ServerIP,
		wgCfg.WireGuardSubnet, wgCfg.WireGuardSubnet6, wgCfg.WANCIDR); valErr != nil {
		return 502, valErr
	}

	if valErr := validateWGPort(wgCfg.WireGuardPort); valErr != nil {
		return 502, valErr
	}

	serverResp := &types.ServerConnectResponse{
		InterfaceIP:      cr.ServerIP,
		WireGuardIP:      wgCfg.WireGuardIP,
		WireGuardPubKey:  wgCfg.WireGuardPubKey,
		WireGuardPort:    wgCfg.WireGuardPort,
		WireGuardSubnet:  wgCfg.WireGuardSubnet,
		WireGuardSubnet6: wgCfg.WireGuardSubnet6,
		WANCIDR:          wgCfg.WANCIDR,
		EnableFirewall:   wgCfg.EnableFirewall,
	}
	tunnel.ServerResponse = serverResp

	if !wgCfg.EnableFirewall && (len(meta.AllowedHosts) > 0 || !meta.AllowAll) {
		SECURITY("server ", cr.ServerID, " has its peer firewall DISABLED — this tunnel's allowed-hosts policy is NOT enforced")
	}

	err = InitializeTunnelFromCRR(tunnel)
	if err != nil {
		return 502, err
	}

	if err = pinServerProtectRoutes(cr, tunnel, wgCfg, *ifName, gateway.To4()); err != nil {
		return 502, err
	}
	startProtectWatcher()
	success := false
	defer func() {
		if !success {
			removeEndpointProtect()
		}
	}()

	ipcConf, hexErr := encodeWGSessionIPC(wgPrivKeyB64, serverResp.WireGuardPubKey, cr.ServerIP, serverResp.WireGuardPort)
	if hexErr != nil {
		return 502, hexErr
	}

	if handled, code, err := tryReplaceLiveWG(oldTunnel, tunnel, wgCfg, meta, ipcConf, state); handled {
		if err == nil {
			success = true
		}
		return code, err
	}

	if replaced, code, err := tryReuseStickyTUN(oldTunnel, tunnel, wgCfg, meta, ipcConf); replaced {
		if err == nil {
			success = true
		}
		return code, err
	}

	if oldTunnel != nil {
		oldTunnel.SetState(TunnelDisconnecting)
		destroyReusablePath(oldTunnel)
	}

	if err := createFreshWG(tunnel, wgCfg, meta, ipcConf); err != nil {
		return 502, err
	}

	startLiveSession(tunnel)

	if oldTunnel != nil {
		Disconnect(oldTunnel.ID, true)
	}

	success = true
	return 200, nil
}

func pinServerProtectRoutes(cr *ConnectionRequest, tunnel *TUN, wgCfg *wgServerConfig, ifName string, gw4ip net.IP) error {
	gw4 := gw4ip.String()
	if err := addIPv4Route(cr.ServerIP+"/32", ifName, gw4, "0"); err != nil {
		return errors.New("unable to initialize routes")
	}
	registerProtectHost(tunnel, cr.ServerIP)
	if wgCfg.ServerIP != "" && wgCfg.ServerIP != cr.ServerIP {
		if rerr := addIPv4Route(wgCfg.ServerIP+"/32", ifName, gw4, "0"); rerr != nil {
			ERROR("unable to add extra server IP route: ", rerr)
		}
		registerProtectHost(tunnel, wgCfg.ServerIP)
	}
	if protErr := applyEndpointProtect(ifName, gw4ip); protErr != nil {
		ERROR("unable to install WireGuard socket protect route: ", protErr)
	}
	return nil
}

func encodeWGSessionIPC(privB64, pubB64, serverIP, port string) (string, error) {
	privHex, err := wgB64ToHex(privB64)
	if err != nil {
		return "", errors.New("unable to encode WireGuard private key")
	}
	serverPubHex, err := wgB64ToHex(pubB64)
	if err != nil {
		return "", errors.New("unable to encode WireGuard server public key")
	}
	return buildWGIPC(privHex, serverPubHex, serverIP, port), nil
}

func refreshAdapterFromSession(inter *adapter, wgCfg *wgServerConfig, meta *TunnelMeta) {
	inter.IPv4Address = wgCfg.WireGuardIP
	inter.Gateway = wgCfg.WireGuardIP
	inter.MTU = meta.MTU
	inter.TxQueuelen = meta.TxQueueLen
}

func tryReplaceLiveWG(oldTunnel, tunnel *TUN, wgCfg *wgServerConfig, meta *TunnelMeta, ipcConf string, state *State) (handled bool, code int, err error) {
	if oldTunnel == nil || !wgDeviceAlive(oldTunnel.wgDevice) {
		return false, 0, nil
	}
	if ipcErr := applyWGIPC(oldTunnel.wgDevice, ipcConf); ipcErr != nil {
		ERROR("in-place WireGuard IPC failed, keeping existing session: ", ipcErr)
		return true, 502, fmt.Errorf("WireGuard IPC configuration failed: %w", ipcErr)
	}
	oldTunnel.SetState(TunnelDisconnecting)
	tunnel.wgDevice = oldTunnel.wgDevice
	tunnel.wgBind = oldTunnel.wgBind
	tunnel.osTUN = oldTunnel.osTUN
	tunnel.procTUN = oldTunnel.procTUN
	if tunnel.procTUN != nil {
		tunnel.procTUN.bindTunnel(tunnel)
	}
	inter := oldTunnel.tunnel.Load()
	if inter == nil {
		return true, 502, errors.New("existing tunnel has no interface")
	}
	refreshAdapterFromSession(inter, wgCfg, meta)
	tunnel.tunnel.Store(inter)
	inter.tunnel.Store(&tunnel)
	if err = inter.Connect(tunnel); err != nil {
		ERROR("unable to refresh tunnel interface after in-place replace: ", err)
		return true, 502, errors.New("unable to connect to tunnel interface")
	}
	startLiveSession(tunnel)
	Disconnect(oldTunnel.ID, true)
	if id := state.DefaultInterfaceID.Load(); id > 0 {
		if pinErr := pinProtectBind(tunnel.wgBind, uint32(id)); pinErr != nil {
			ERROR("unable to refresh WireGuard socket pin after in-place replace: ", pinErr)
		}
	}
	DEBUG("replaced WireGuard session in place (no TUN recreate)")
	return true, 200, nil
}

func tryReuseStickyTUN(oldTunnel, tunnel *TUN, wgCfg *wgServerConfig, meta *TunnelMeta, ipcConf string) (replaced bool, code int, err error) {
	if oldTunnel == nil || oldTunnel.osTUN == nil || !oldTunnel.osTUN.CanReuse() {
		return false, 0, nil
	}
	oldTunnel.SetState(TunnelDisconnecting)
	if oldTunnel.wgDevice != nil {
		oldTunnel.wgDevice.Close()
	}
	if err := oldTunnel.osTUN.ResetForReuse(); err != nil {
		ERROR("sticky TUN reset failed: ", err)
		destroyReusablePath(oldTunnel)
		return false, 0, nil
	}
	tunnel.osTUN = oldTunnel.osTUN
	tunnel.procTUN = oldTunnel.procTUN
	if tunnel.procTUN != nil {
		tunnel.procTUN.bindTunnel(tunnel)
	}
	inter := oldTunnel.tunnel.Load()
	if inter == nil {
		return false, 0, nil
	}
	refreshAdapterFromSession(inter, wgCfg, meta)
	if err := attachWGDevice(tunnel, inter, tunnel.procTUN, ipcConf); err != nil {
		ERROR("reuse TUN attach failed: ", err)
		destroyReusablePath(oldTunnel)
		return false, 0, nil
	}
	startLiveSession(tunnel)
	Disconnect(oldTunnel.ID, true)
	DEBUG("reused OS TUN after WireGuard device death")
	return true, 200, nil
}

func createFreshWG(tunnel *TUN, wgCfg *wgServerConfig, meta *TunnelMeta, ipcConf string) error {
	osTun, tunErr := wgtun.CreateTUN(resolveTUNCreateName(meta.IFName), int(meta.MTU))
	if tunErr != nil {
		return fmt.Errorf("unable to create TUN interface: %w", tunErr)
	}
	tunIfName, _ := osTun.Name()

	inter := &adapter{
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
		return err
	}
	return nil
}

func bindLocalDNSClient(t *TUN) {
	if DNSClient.Dialer == nil {
		return
	}
	t.localDNSClient = new(dns.Client)
	t.localDNSClient.Dialer = new(net.Dialer)
	t.localDNSClient.Dialer.LocalAddr = &net.UDPAddr{
		IP: t.localInterfaceNetIP.To4(),
	}
	t.localDNSClient.Dialer.Resolver = DNSClient.Dialer.Resolver
	t.localDNSClient.Dialer.Timeout = 5 * time.Second
	t.localDNSClient.Timeout = time.Second * 5
}

func applyTunnelMetaOverlays(t *TUN, meta *TunnelMeta) {
	if meta.LocalhostNat {
		NN := new(types.Network)
		NN.Network = "127.0.0.1/32"
		NN.Nat = t.serverInterfaceNetIP.String() + "/32"
		t.ServerResponse.Networks = append(t.ServerResponse.Networks, NN)
	}

	if len(meta.Networks) > 0 {
		t.ServerResponse.Networks = meta.Networks
	}
	if len(meta.Routes) > 0 {
		t.ServerResponse.Routes = meta.Routes
	}
	if len(meta.DNSRecords) > 0 {
		t.ServerResponse.DNSRecords = meta.DNSRecords
	}
	if len(meta.DNSServers) > 0 {
		t.ServerResponse.DNSServers = meta.DNSServers
	}

	conf := CONFIG.Load()
	if len(t.ServerResponse.DNSServers) < 1 {
		t.ServerResponse.DNSServers = []string{conf.DNS1Default, conf.DNS2Default}
	}
}

func copyIPv4Bytes(dst *[4]byte, ip net.IP) {
	dst[0] = ip[0]
	dst[1] = ip[1]
	dst[2] = ip[2]
	dst[3] = ip[3]
}

func InitializeTunnelFromCRR(t *TUN) (err error) {
	DNSGlobalBlock.Store(true)
	defer func() {
		RecoverAndLog()
		DNSGlobalBlock.Store(false)
	}()
	go FullCleanDNSCache()

	meta := t.meta.Load()

	t.localInterfaceNetIP = net.ParseIP(t.ServerResponse.WireGuardIP).To4()
	if t.localInterfaceNetIP == nil {
		return fmt.Errorf("Interface ip (%s) was malformed", t.ServerResponse.WireGuardIP)
	}
	copyIPv4Bytes(&t.localInterfaceIP4bytes, t.localInterfaceNetIP)

	bindLocalDNSClient(t)

	t.serverInterfaceNetIP = net.ParseIP(t.ServerResponse.InterfaceIP).To4()
	if t.serverInterfaceNetIP == nil {
		return fmt.Errorf("Interface ip (%s) was malformed", t.ServerResponse.InterfaceIP)
	}

	copyIPv4Bytes(&t.serverInterfaceIP4bytes, t.serverInterfaceNetIP)
	t.wgEndpointSet = true

	applyTunnelMetaOverlays(t, meta)

	t.InitBlockedPorts(t.meta.Load().BlockedPorts)

	err = t.InitNatMaps()
	if err != nil {
		return err
	}

	DEBUG(fmt.Sprintf(
		"Connection info: Addr(%s) srcIP(%s)",
		t.ServerResponse.WireGuardIP,
		t.ServerResponse.InterfaceIP,
	))

	return nil
}
