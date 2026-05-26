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
		Server:      cs,
		Tag:         metaTag,
		ServerID:    conf.CLIConfig.ServerID,
		DeviceToken: conf.CLIConfig.DeviceToken,
		UserID:      conf.CLIConfig.UserID,
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

		conf := CONFIG.Load()
		cliConf := conf.CLIConfig
		if cliConf != nil {
			err = cliPublicConnect(meta.Tag)
		} else {
		}

		if err != nil || code != 200 {
			ERROR("Unable to connect, return code: ", code, " // error: ", err)
		}
		return true
	})
}
