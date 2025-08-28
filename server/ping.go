package main

import (
	"encoding/binary"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
)

var PingPongStatsBuffer = []byte{
	0, 0, 0, // stats
	0, 0, 0, 0, 0, 0, 0, 0, // timestamp
	0, 0, 0, 0, 0, 0, 0, 0, // ping counter
}

func PopulatePingBufferWithStats() {
	cpuPercent, err := cpu.Percent(0, false)
	if err != nil {
		ERR("Unable to get cpu percent", err)
		return
	}
	PingPongStatsBuffer[0] = byte(int(cpuPercent[0]))

	memStats, err := mem.VirtualMemory()
	if err != nil {
		ERR("Unable to get mem stats", err)
		return

	}
	PingPongStatsBuffer[1] = byte(int(memStats.UsedPercent))

	diskUsage, err := disk.Usage("/")
	if err != nil {
		ERR("Unable to get disk usage", err)
		return
	}
	PingPongStatsBuffer[2] = byte(int(diskUsage.UsedPercent))
}

func NukeClient(index int) {
	LOG("Removing index:", index)
	cm := clientCoreMappings[index]
	if cm == nil {
		ERR("Nuke client on nill index", index)
		return
	}

	if cm.PortRange != nil {
		for i, v := range portToCoreMapping {
			if v == nil {
				continue
			}

			if v.StartPort == cm.PortRange.StartPort {
				WARN("removing port range:", v.StartPort)
				portToCoreMapping[i].Client = nil
			}
		}
	}

	if clientCoreMappings[index].DHCP != nil {
		// ip := clientCoreMappings[index].DHCP.IP
		// VPLIPToCore[ip[0]][ip[1]][ip[2]][ip[3]] = nil
	}

	close(clientCoreMappings[index].ToUser)
	close(clientCoreMappings[index].FromUser)
	clientCoreMappings[index].FromSignal.ShouldStop.Store(true)
	clientCoreMappings[index].ToSignal.ShouldStop.Store(true)
	clientCoreMappings[index] = nil
}

func pingActiveUsers() {
	PopulatePingBufferWithStats()

	for index, u := range clientCoreMappings {
		if u == nil {
			continue
		}
		if len(u.Uindex) == 0 {
			continue
		}

		binary.BigEndian.PutUint64(PingPongStatsBuffer[11:], uint64(clientCoreMappings[index].PingInt.Load()))
		binary.BigEndian.PutUint64(PingPongStatsBuffer[3:11], uint64(time.Now().UnixNano()))
		out := u.EH.SEAL.Seal2(PingPongStatsBuffer, u.Uindex)
		err := syscall.Sendto(dataSocketFD, out, 0, u.Addr)
		if err != nil {
			LOG("Index ping error: ", index, err)
			NukeClient(index)
			continue
		}

		if time.Since(u.Created).Seconds() < 30 {
			continue
		}

		cfg := Config.Load()

		if time.Since(u.LastPingFromClient).Minutes() > float64(cfg.PingTimeoutMinutes) {
			LOG("Ping timeout:", index, "last seen:", time.Since(u.LastPingFromClient).Minutes(), "minutes ago")
			NukeClient(index)
			continue
		}
	}
}
