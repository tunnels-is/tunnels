package client

import (
	"bufio"
	"fmt"
	"strings"

	wgdevice "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

func wgDeviceAlive(d *wgdevice.Device) bool {
	if d == nil {
		return false
	}
	select {
	case <-d.Wait():
		return false
	default:
		return true
	}
}

func buildWGIPC(privHex, serverPubHex, endpointIP, endpointPort string) string {
	return fmt.Sprintf(
		"private_key=%s\nreplace_peers=true\npublic_key=%s\nendpoint=%s:%s\nreplace_allowed_ips=true\nallowed_ip=0.0.0.0/0\npersistent_keepalive_interval=25\n\n",
		privHex, serverPubHex, endpointIP, endpointPort,
	)
}

func applyWGIPC(dev *wgdevice.Device, ipcConf string) error {
	return dev.IpcSetOperation(bufio.NewReader(strings.NewReader(ipcConf)))
}

func watchWGDevice(tunnel *TUN) {
	go func() {
		defer RecoverAndLog()
		dev := tunnel.wgDevice
		if dev == nil {
			return
		}
		<-dev.Wait()
		m := tunnel.meta.Load()
		tag := ""
		if m != nil {
			tag = m.Tag
		}
		DEBUG("WireGuard device closed:", tag, tunnel.ID)
		if tunnel.GetState() >= TUN_Connected {
			tunnelMonitor <- tunnel
		}
	}()
}

func wrapCreatedTUN(osTun tun.Device, tunnel *TUN) tun.Device {
	dev := osTun
	if osTun.File() != nil {
		sticky := newStickyTUN(osTun)
		tunnel.osTUN = sticky
		dev = sticky
	}
	pt := newProcessingTUN(dev, tunnel)
	tunnel.procTUN = pt
	return pt
}

func destroyReusablePath(old *TUN) {
	if old == nil {
		return
	}
	if old.wgDevice != nil && wgDeviceAlive(old.wgDevice) {
		old.wgDevice.Close()
	}
	if old.osTUN != nil {
		_ = old.osTUN.Release()
		old.osTUN = nil
	}
	if inter := old.tunnel.Load(); inter != nil {
		_ = inter.Delete()
	}
}
