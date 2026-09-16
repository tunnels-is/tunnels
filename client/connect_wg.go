package client

import (
	"encoding/json"
	"errors"
	"fmt"
	neturl "net/url"

	wgdevice "golang.zx2c4.com/wireguard/device"
	wgtun "golang.zx2c4.com/wireguard/tun"
)

func attachWGDevice(tunnel *TUN, inter *adapter, pt wgtun.Device, ipcConf string) error {
	var ifIndex uint32
	if id := STATE.Load().DefaultInterfaceID.Load(); id > 0 {
		ifIndex = uint32(id)
	} else {
		ERROR("no physical interface index; WireGuard socket will not be pinned to the NIC")
	}
	bind := newProtectBind(ifIndex)
	tunnel.wgBind = bind
	startProtectWatcher()
	wgDev := wgdevice.NewDevice(pt, bind, NewWGLogger())
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
	responseBytes, code, err := SendRequestToURL(nil, "GET", url, nil, 10000, cr.Server.ValidateCertificate, cr.Server.CertificatePath, authHeaders)
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
