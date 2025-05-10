package core

import (
	"errors"
	"math/rand"

	"github.com/tunnels-is/tunnels/crypt"
	"github.com/tunnels-is/tunnels/types"
)

func CreateAndConnectToInterface(t *TUN) (inter *TInterface, err error) {
	meta := t.meta.Load()
	inter, err = CreateNewTunnelInterface(meta)
	if err != nil {
		ERROR("unable to create tunnel interface: ", err)
		return nil, errors.New("Unable to create tunnel interface")
	}

	return
}

func Disconnect(tunID string, switching bool) (err error) {
	DEBUG("disconnecting from", tunID, switching)
	tunnelMapRange(func(tun *TUN) bool {
		if tun.ID == tunID {
			tun.SetState(TUN_Disconnecting)
			tunnel := tun.tunnel.Load()
			_ = tunnel.Disconnect(tun)
			if tun.encWrapper != nil {
				if tun.encWrapper.HStream != nil {
					tun.encWrapper.HStream.Close()
				}
				if tun.encWrapper.HConn != nil {
					tun.encWrapper.HConn.Close()
				}
			}

			TunnelMap.Delete(tun.ID)
			m := tun.meta.Load()
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

func createRandomTunnel() (m *TunnelMETA, err error) {
	m = createTunnel()
	TunnelMetaMap.Store(m.Tag, m)
	err = writeTunnelsToDisk(m.Tag)
	return
}

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

	T.EncryptionType = crypt.CHACHA20
	T.DNSBlocking = true
	T.PreventIPv6 = true
	T.TxQueueLen = 2000
	T.MTU = 1400
	T.ServerID = ""
	T.AutoReconnect = false
	T.AutoConnect = false
	T.Networks = make([]*types.Network, 0)
	T.DNSServers = make([]string, 0)
	T.DNSRecords = make([]*types.DNSRecord, 0)
	T.WindowsGUID = CreateConnectionUUID()
	return
}

func createDefaultTunnelMeta() (M *TunnelMETA) {
	M = createTunnel()
	M.RequestVPNPorts = true
	M.IPv4Address = "172.22.22.1"
	M.NetMask = "255.255.255.255"
	M.Tag = DefaultTunnelName
	M.IFName = DefaultTunnelName
	M.EnableDefaultRoute = true
	return
}
