package client

import (
	"time"
)

func cliPublicConnect(metaTag string) (err error) {
	conf := CONFIG.Load()
	if conf.CLIConfig == nil {
		return nil
	}
	var cs *ControlServer
	for i := range conf.ControlServers {
		if conf.ControlServers[i].ID == conf.CLIConfig.ControlServerID {
			cs = conf.ControlServers[i]
		}
	}
	if cs == nil {
		DEBUG("No control server found")
	}
	code, err := PublicConnect(&ConnectionRequest{
		Server:    cs,
		Tag:       metaTag,
		ServerID:  conf.CLIConfig.ServerID,
		DeviceKey: conf.CLIConfig.DeviceID,
	})
	if err != nil {
		ERROR("Connecting using cli config failed, code:", code, "err:", err)
	}

	return err
}

func AutoConnect() {
	defer func() {
		time.Sleep(30 * time.Second)
	}()
	defer RecoverAndLog()

	tunnelMetaMapRange(func(meta *TunnelMETA) bool {
		if !meta.AutoConnect {
			return true
		}

		isConnected := false
		tunnelMapRange(func(tun *TUN) bool {
			if tun.CR.Tag == meta.Tag {
				if tun.GetState() >= TUN_Connecting {
					isConnected = true
					return false
				}
				return false
			}
			return true
		})
		if isConnected {
			return true
		}

		var code int
		var err error
		// var user *User
		conf := CONFIG.Load()
		cliConf := conf.CLIConfig
		if cliConf != nil {
			err = cliPublicConnect(meta.Tag)
		} else {
			// TODO
			// user, err = getUser()
			// if err != nil {
			// 	return true
			// }
			// code, err = PublicConnect(&ConnectionRequest{
			// 	Tag:         meta.Tag,
			// 	ServerID:    meta.ServerID,
			// 	DeviceToken: user.DeviceToken.DT,
			// 	// URL:         user.AuthServer,
			// 	// Secure: user.Secure,
			// })
		}

		if err != nil || code != 200 {
			ERROR("Unable to connect, return code: ", code, " // error: ", err)
		}
		return true
	})
}

func PingConnections() {
	defer func() {
		time.Sleep(10 * time.Second)
	}()
	defer RecoverAndLog()

	conf := CONFIG.Load()

	tunnelMapRange(func(tun *TUN) bool {
		meta := tun.meta.Load()
		if meta == nil {
			return true
		}

		// WireGuard keepalive is handled by persistent_keepalive_interval.
		// Reset ping timer so the 45s reconnect threshold doesn't fire.
		tun.registerPing(time.Now())

		ping := tun.pingTime.Load()
		if time.Since(*ping).Seconds() > 45 || tun.needsReconnect.Load() {
			if meta.AutoReconnect {
				DEBUG("45+ Seconds since ping from ", meta.Tag, " attempting reconnection")
				var err error
				if conf.CLIConfig != nil {
					err = cliPublicConnect(meta.Tag)
				} else {
					_, err = PublicConnect(tun.CR)
				}
				if err != nil {
					tun.SetState(TUN_NotReady)
					ERROR("unable to reconnect: ", err)
				} else {
					tun.needsReconnect.Store(false)
				}
			} else {
				DEBUG("30+ Seconds since ping from ", meta.Tag)
				if !meta.KillSwitch {
					_ = Disconnect(tun.ID, false)
				}
				tun.needsReconnect.Store(false)
			}
		}

		return true
	})
}
