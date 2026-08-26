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

	conf := CONFIG.Load()
	if conf.CLIConfig != nil {
		if err := cliPublicConnect(DefaultTunnelName); err != nil {
			ERROR("Unable to connect using CLI config: ", err)
		}
	}
}
