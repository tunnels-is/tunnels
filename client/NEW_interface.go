package client

import (
	"encoding/binary"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
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

var PingPongStatsBuffer = []byte{
	0, 0, 0, 0, // stats
	0, 0, 0, 0, 0, 0, 0, 0, // ping counter
}

func PopulatePingBufferWithStats() {
	cpuPercent, err := cpu.Percent(0, false)
	if err != nil {
		ERROR("Unable to get cpu percent", err)
		return
	}
	PingPongStatsBuffer[1] = byte(int(cpuPercent[0]))

	memStats, err := mem.VirtualMemory()
	if err != nil {
		ERROR("Unable to get mem stats", err)
		return

	}
	PingPongStatsBuffer[2] = byte(int(memStats.UsedPercent))

	diskUsage, err := disk.Usage("/")
	if err != nil {
		ERROR("Unable to get disk usage", err)
		return
	}
	PingPongStatsBuffer[3] = byte(int(diskUsage.UsedPercent))
}

func PingConnections() {
	defer func() {
		time.Sleep(10 * time.Second)
	}()
	defer RecoverAndLog()

	// Only send statistics for minimal clients
	conf := CONFIG.Load()
	if conf.CLIConfig != nil && conf.CLIConfig.SendStats {
		PopulatePingBufferWithStats()
	}

	tunnelMapRange(func(tun *TUN) bool {
		meta := tun.meta.Load()
		if meta == nil {
			return true
		}

		var err error
		if tun.encWrapper != nil {

			tun.PingInt.Add(1)
			binary.BigEndian.PutUint64(PingPongStatsBuffer[4:], uint64(tun.PingInt.Load()))
			out := tun.encWrapper.SEAL.Seal1(PingPongStatsBuffer, tun.Index)
			if len(out) > 0 {
				DEEP("Ping: ", meta.Tag, " ", tun.PingInt.Load())
				_, err = tun.connection.Write(CopySlice(out))
				if err != nil {
					tun.SetState(TUN_NotReady)
					_ = tun.connection.Close()
					ERROR("unable to ping tunnel: ", tun.ID, meta.Tag)
				}
			}

		}

		ping := tun.pingTime.Load()
		if time.Since(*ping).Seconds() > 45 || err != nil || tun.needsReconnect.Load() {
			if meta.AutoReconnect {
				DEBUG("45+ Seconds since ping from ", meta.Tag, " attempting reconnection")
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
