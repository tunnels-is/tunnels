package client

import (
	"errors"
	"fmt"
	"math/rand"

	"github.com/tunnels-is/tunnels/crypt"
	"github.com/tunnels-is/tunnels/types"
)

// CreateAndConnectToInterface creates a new tunnel interface
func CreateAndConnectToInterface(t *TUN) (inter *TInterface, err error) {
	meta := t.meta.Load()
	inter, err = CreateNewTunnelInterface(meta)
	if err != nil {
		ERROR("unable to create tunnel interface: ", err)
		return nil, errors.New("Unable to create tunnel interface")
	}

	return
}

// Disconnect disconnects a tunnel by ID
func Disconnect(tunID string, switching bool) (err error) {
	DEBUG("disconnecting from", tunID, switching)
	tunnelMapRange(func(tun *TUN) bool {
		if tun.ID == tunID {
			tun.SetState(TUN_Disconnecting)
			tunnel := tun.tunnel.Load()
			if !switching {
				_ = tunnel.Disconnect(tun)
			} else {
				if tun.connection != nil {
					_ = tun.connection.Close()
				}
			}
			if tun.encWrapper != nil {
				if tun.encWrapper.HStream != nil {
					_ = tun.encWrapper.HStream.Close()
				}
				if tun.encWrapper.HConn != nil {
					_ = tun.encWrapper.HConn.Close()
				}
			}

			TunnelMap.Delete(tun.ID)
			m := tun.meta.Load()
			tun.SetState(TUN_Disconnected)
			if m != nil {
				DEBUG("disconnected from ", m.Tag, tun.ID)
			} else {
				DEBUG("disconnected from ", "(tag unknown)", tun.ID)
			}
			return false
		}
		return true
	})

	return
}

// createRandomTunnel creates a tunnel with random parameters
func createRandomTunnel() (m *TunnelMETA, err error) {
	m = createTunnel()
	TunnelMetaMap.Store(m.Tag, m)
	err = writeTunnelsToDisk(m.Tag)
	return
}

// createTunnel creates a new tunnel metadata object with default values
func createTunnel() (T *TunnelMETA) {
	T = new(TunnelMETA)
	b := make([]rune, 8)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	ifAndTag := string(b)
	T.Tag = ifAndTag
	T.IFName = ifAndTag
	T.EnableDefaultRoute = false
	T.IPv4Address = "777.777.777.777"
	T.NetMask = "255.255.255.255"
	randomPart1 := rand.Intn(0xFFFF)
	randomPart2 := rand.Intn(0xFFFF)
	T.IPv6Address = fmt.Sprintf("fd00:%04x:%04x::1", randomPart1, randomPart2)

	T.EncryptionType = crypt.CHACHA20
	T.DNSBlocking = true
	T.TxQueueLen = 2000
	T.MTU = 1420
	T.AutoReconnect = true
	T.AutoConnect = false
	T.Networks = make([]*types.Network, 0)
	T.DNSServers = make([]string, 0)
	T.DNSRecords = make([]*types.DNSRecord, 0)
	T.Routes = make([]*types.Route, 0)
	T.WindowsGUID = CreateConnectionUUID()
	T.KillSwitch = true
	return
}

// createDefaultTunnelMeta creates default tunnel metadata based on tunnel type
func createDefaultTunnelMeta(t types.TunnelType) (M *TunnelMETA) {
	M = createTunnel()
	M.RequestVPNPorts = true
	M.IPv4Address = "172.22.22.1"
	M.NetMask = "255.255.255.255"

	M.Tag = DefaultTunnelName
	M.IFName = DefaultTunnelName

	switch t {
	case types.DefaultTun:
		M.EnableDefaultRoute = true
	case types.IoTTun:
		M.EnableDefaultRoute = false
		M.RequestVPNPorts = false
		M.LocalhostNat = true
		M.AutoConnect = true
		M.AutoReconnect = true
		M.MTU = 1320 // MTU is set low here due to many 5g networks. The LAN tunnels is mostly used to IoT.
	}

	return
}

// CleanupOnClose disconnects all tunnels and closes files on application exit
func CleanupOnClose() {
	defer RecoverAndLog()
	tunnelMapRange(func(tun *TUN) bool {
		tunnel := tun.tunnel.Load()
		err := tunnel.Disconnect(tun)
		if err != nil {
			ERROR("unable to disconnect tunnel", tun.ID, tunnel.IPv4Address, "error:", err)
		}
		return true
	})
	if TraceFile != nil {
		_ = TraceFile.Close()
	}
	if LogFile != nil {
		_ = LogFile.Close()
	}
}
