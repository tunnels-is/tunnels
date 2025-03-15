package core

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
		if !meta.AutoConnect {
			return true
		}
		code, err := PublicConnect(&ConnectionRequest{
			Tag:        meta.Tag,
			DeviceKey:  meta.deviceKey,
			ServerIP:   meta.ServerIP,
			ServerPort: meta.ServerPort,
		})
		if err != nil {
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
		time.Sleep(30 * time.Second)
	}()
	defer RecoverAndLogToFile()

	conf := CONFIG.Load()

	// Only send statistics for minimal clients
	if conf.Minimal {
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
					ERROR("unable to ping tunnel: ", tun.id, meta.Tag)
				}
			}

		}

		ping := tun.pingTime.Load()
		if time.Since(*ping).Seconds() > 30 || err != nil {
			if meta.AutoReconnect {
				DEBUG("30+ Seconds since ping from ", meta.Tag, " attempting reconnection")
				_, _ = PublicConnect(tun.cr)
			} else {
				DEBUG("30+ Seconds since ping from ", meta.Tag, " disconnecting")
				_ = Disconnect(tun.id, true)
			}
		}

		return true
	})
}

func isGatewayATunnel(gateway net.IP) (isTunnel bool) {
	tunnelMapRange(func(tun *TUN) bool {
		tunnel := tun.tunnel.Load()
		if tunnel == nil {
			return true
		}

		if tunnel.IPv4Address == gateway.To4().String() {
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
	copy(oldInterface, s.DefaultInterface.To4())

	var err error
	newInterface, err = gateway.DiscoverInterface()
	if err != nil {
		ERROR("Error looking for default interface", err)
		return
	}

	if bytes.Equal(oldInterface, newInterface.To4()) {
		return
	}

	DEBUG("new defailt interface discovered", newInterface.To4())
	s.DefaultInterface = newInterface

	ifList, _ := net.Interfaces()

LOOP:
	for _, v := range ifList {
		addrs, e := v.Addrs()
		if e != nil {
			continue
		}
		for _, iv := range addrs {
			if strings.Split(iv.String(), "/")[0] == s.DefaultInterface.To4().String() {
				s.DefaultInterfaceID = v.Index
				s.DefaultInterfaceName = v.Name
				break LOOP
			}
		}
	}

	DEBUG(
		"Default interface",
		s.DefaultInterfaceName,
		s.DefaultInterfaceID,
		s.DefaultInterface,
	)
}

func loadDefaultGateway() {
	defer RecoverAndLogToFile()
	s := STATE.Load()

	var err error
	oldGateway := make([]byte, 4)
	var newGateway net.IP
	copy(oldGateway, s.DefaultGateway.To4())

	newGateway, err = gateway.DiscoverGateway()
	if err != nil {
		ERROR("Error looking for default gateway:", err)
		return
	}

	if isGatewayATunnel(newGateway.To4()) {
		return
	}

	if bytes.Equal(oldGateway, newGateway.To4()) {
		return
	}

	DEBUG("new defailt gateway discovered", newGateway.To4())
	s.DefaultGateway = newGateway

	DEBUG(
		"Default Gateway",
		s.DefaultGateway,
	)
}

func GetDefaultGateway() {
	s := STATE.Load()
	defer func() {
		if s.DefaultGateway != nil {
			time.Sleep(5 * time.Second)
		} else {
			time.Sleep(2 * time.Second)
		}
	}()
	loadDefaultGateway()
	loadDefaultInterface()
}
