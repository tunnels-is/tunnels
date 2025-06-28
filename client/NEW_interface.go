package client

import (
	"bytes"
	"net"
	"strings"
	"time"

	"github.com/jackpal/gateway"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
)

func AutoConnect() {
	defer func() {
		time.Sleep(30 * time.Second)
	}()
	defer RecoverAndLogToFile()

	tunnelMetaMapRange(func(meta *TunnelMETA) bool {
		if !meta.AutoConnect || meta.ServerID == "" {
			return true
		}

		isConnected := false
		tunnelMapRange(func(tun *TUN) bool {
			if tun.CR.Tag == meta.Tag {
				isConnected = true
				return false
			}
			return true
		})
		if isConnected {
			return true
		}
		// TODO: update when multi-user support is enabled
		// if meta.Tag != DefaultTunnelName && meta.UserID == "" {
		// 	return true
		// }

		var code int
		var err error
		var user *User
		cliConfig := CLIConfig.Load()
		if cliConfig.Enabled {
			code, err = PublicConnect(&ConnectionRequest{
				Secure:    cliConfig.Secure,
				URL:       cliConfig.AuthServer,
				Tag:       meta.Tag,
				ServerID:  meta.ServerID,
				DeviceKey: cliConfig.DeviceID,
				Hostname:  cliConfig.Hostname,
			})
		} else {
			user, err = loadUser()
			if err != nil {
				return true
			}
			code, err = PublicConnect(&ConnectionRequest{
				Tag:         meta.Tag,
				ServerID:    meta.ServerID,
				DeviceToken: user.DeviceToken.DT,
				URL:         user.AuthServer,
				Secure:      user.Secure,
			})
		}

		if err != nil || code != 200 {
			ERROR("Unable to connect, return code: ", code, " // error: ", err)
		}
		return true
	})
}

var PingPongStatsBuffer = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

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

var prevAllowedHosts = []string{}

func PingConnections() {
	defer func() {
		time.Sleep(20 * time.Second)
	}()
	defer RecoverAndLogToFile()

	cli := CLIConfig.Load()

	// Only send statistics for minimal clients
	if cli.Enabled && cli.SendStats {
		PopulatePingBufferWithStats()
	}

	tunnelMapRange(func(tun *TUN) bool {
		meta := tun.meta.Load()
		if meta == nil {
			return true
		}

		var err error
		if tun.encWrapper != nil {
			out := tun.encWrapper.SEAL.Seal1(PingPongStatsBuffer, tun.Index)
			if len(out) > 0 {
				_, err = tun.connection.Write(out)
				if err != nil {
					ERROR("unable to ping tunnel: ", tun.ID, meta.Tag)
				}
			}

		}

		ping := tun.pingTime.Load()
		if time.Since(*ping).Seconds() > 30 || err != nil {
			if meta.AutoReconnect {
				DEBUG("30+ Seconds since ping from ", meta.Tag, " attempting reconnection")
				_, _ = PublicConnect(tun.CR)
			} else {
				DEBUG("30+ Seconds since ping from ", meta.Tag)
				if !meta.KillSwitch {
					_ = Disconnect(tun.ID, false)
				}
			}
		}

		return true
	})
}

func isInterfaceATunnel(interf net.IP) (isTunnel bool) {
	tunnelMapRange(func(tun *TUN) bool {
		tunnel := tun.tunnel.Load()
		if tunnel == nil {
			return true
		}

		if tunnel.IPv4Address == interf.To4().String() {
			isTunnel = true
			return false
		}
		return true
	})

	return
}

func loadDefaultInterface() {
	defer RecoverAndLogToFile()
	s := STATE.Load()
	oldInterface := make([]byte, 4)
	var newInterface net.IP
	def := s.DefaultInterface.Load()
	if def != nil {
		copy(oldInterface, def.To4())
	}

	var err error
	newInterface, err = gateway.DiscoverInterface()
	if err != nil {
		ERROR("Error looking for default interface", err)
		return
	}

	if bytes.Equal(oldInterface, newInterface.To4()) {
		return
	}

	if isInterfaceATunnel(newInterface.To4()) {
		return
	}

	DEBUG("new defailt interface discovered", newInterface.To4())
	s.DefaultInterface.Store(&newInterface)

	ifList, _ := net.Interfaces()

LOOP:
	for _, v := range ifList {
		addrs, e := v.Addrs()
		if e != nil {
			continue
		}
		for _, iv := range addrs {
			if strings.Split(iv.String(), "/")[0] == newInterface.To4().String() {
				s.DefaultInterfaceID.Store(int32(v.Index))
				name := v.Name
				s.DefaultInterfaceName.Store(&name)
				break LOOP
			}
		}
	}

	DEBUG(
		"Default interface >>",
		s.DefaultInterfaceName.Load(),
		s.DefaultInterfaceID.Load(),
		s.DefaultInterface.Load(),
	)
}

func loadDefaultGateway() {
	defer RecoverAndLogToFile()
	s := STATE.Load()

	var err error
	oldGateway := make([]byte, 4)
	var newGateway net.IP
	def := s.DefaultGateway.Load()
	if def != nil {
		copy(oldGateway, def.To4())
	}

	newGateway, err = gateway.DiscoverGateway()
	if err != nil {
		ERROR("Error looking for default gateway:", err)
		return
	}

	if bytes.Equal(oldGateway, newGateway.To4()) {
		return
	}

	if isInterfaceATunnel(newGateway.To4()) {
		return
	}
	DEBUG("new defailt gateway discovered", newGateway.To4())
	s.DefaultGateway.Store(&newGateway)

	DEBUG(
		"Default Gateway",
		s.DefaultGateway.Load(),
	)
}

func GetDefaultGateway() {
	s := STATE.Load()
	defer func() {
		if s.DefaultGateway.Load() != nil {
			time.Sleep(5 * time.Second)
		} else {
			time.Sleep(2 * time.Second)
		}
	}()
	loadDefaultGateway()
	loadDefaultInterface()
}
