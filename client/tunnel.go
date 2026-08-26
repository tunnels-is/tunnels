package client

import (
	"crypto/rand"
	"errors"
	"math/big"
	"time"

	"github.com/tunnels-is/tunnels/types"
)

func announceClearAndFlush(tun *TUN) {
	if err := tun.AnnounceAllowedHosts(nil, false); err == nil {
		time.Sleep(200 * time.Millisecond)
	}
}

func Disconnect(tunID string, switching bool) (err error) {
	DEBUG("disconnecting from", tunID, switching)
	tunnelMapRange(func(tun *TUN) bool {
		if tun.ID == tunID {
			tun.SetState(TUN_Disconnecting)
			tunnel := tun.tunnel.Load()
			if !switching {

				announceClearAndFlush(tun)
				if tun.osTUN != nil {
					_ = tun.osTUN.Release()
				}
				_ = tunnel.Disconnect(tun)
				_ = applyConfiguredKillSwitch()
			}
			TunnelMap.Delete(tun.ID)
			m := tun.meta.Load()
			tun.SetState(TUN_Disconnected)
			if m != nil {
				DEBUG("disconnected from ", m.Tag, tun.ID)
			} else {
				DEBUG("disconnected from ", "(tag unknown)", tun.ID)
			}
			removeEndpointProtect()
			return false
		}
		return true
	})

	return
}

func persistTunnelServerID(meta *TunnelMETA, serverID string) error {
	if meta == nil {
		return errors.New("tunnel metadata is required")
	}
	if serverID == "" || meta.ServerID == serverID {
		return nil
	}
	meta.ServerID = serverID
	return writeTunnelsToDisk(meta.Tag)
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
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(letterRunes))))
		if err != nil {
			return nil
		}
		b[i] = letterRunes[n.Int64()]
	}
	ifAndTag := string(b)
	T.Tag = ifAndTag
	T.IFName = ifAndTag
	T.EnableDefaultRoute = false

	T.DNSBlocking = true
	T.TxQueueLen = 2000
	T.MTU = 1420
	T.AutoReconnect = true
	T.AutoConnect = false
	T.Networks = make([]*types.Network, 0)
	T.DNSServers = make([]string, 0)
	T.DNSRecords = make([]*types.DNSRecord, 0)
	T.Routes = make([]*types.Route, 0)
	T.EnableWAN = true
	return
}

func createDefaultTunnelMeta(t types.TunnelType) (M *TunnelMETA) {
	M = createTunnel()
	M.ConfigFormat = tunnelFileSuffix

	M.Tag = DefaultTunnelName
	M.IFName = DefaultTunnelName

	switch t {
	case types.DefaultTun:
		M.EnableDefaultRoute = true
	case types.IoTTun:
		M.EnableDefaultRoute = false
		M.LocalhostNat = true
		M.AutoConnect = true
		M.AutoReconnect = true
		M.MTU = 1320
	}

	return
}

func CleanupOnClose() {
	defer RecoverAndLog()

	stopAllReconnects()
	tunnelMapRange(func(tun *TUN) bool {
		announceClearAndFlush(tun)
		if tun.osTUN != nil {
			_ = tun.osTUN.Release()
		}
		tunnel := tun.tunnel.Load()
		err := tunnel.Disconnect(tun)
		if err != nil {
			ERROR("unable to disconnect tunnel", tun.ID, "error:", err)
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
