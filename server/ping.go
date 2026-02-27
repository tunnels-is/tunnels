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
	0, 0, 0, // stats 0-3
	255, 1, 255, 2, 255, 3, 255, 4, // timestamp 3-11
	0, 0, 0, 0, 0, 0, 0, 0, // ping counter 11-19
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
	cm := clientCoreMappings[index].Load()
	if cm == nil {
		ERR("NukeClient on nil index", index)
		return
	}

	if cm.DHCP != nil {
		ip := cm.DHCP.IP
		VPLIPToCore[uint16(ip[2])<<8|uint16(ip[3])].Store(nil)
		for i := range clientCoreMappings[:slots] {
			if i == index {
				continue
			}
			if o := clientCoreMappings[i].Load(); o != nil {
				o.ClearHost(ip)
			}
		}
	}

	close(cm.ToUser)
	close(cm.FromUser)
	if cm.FromSignal != nil {
		cm.FromSignal.ShouldStop.Store(true)
	}
	if cm.ToSignal != nil {
		cm.ToSignal.ShouldStop.Store(true)
	}

	clientCoreMappings[index].Store(nil)
}

func pingActiveUsers() {
	PopulatePingBufferWithStats()

	for index := range clientCoreMappings[:slots] {
		u := clientCoreMappings[index].Load()
		if u == nil {
			continue
		}
		if len(u.Uindex) == 0 {
			continue
		}
		addr, _ := u.Addr.Load().(syscall.Sockaddr)
		if addr == nil {
			continue
		}

		binary.BigEndian.PutUint64(PingPongStatsBuffer[11:], uint64(u.PingInt.Load()))
		out := u.EH.SEAL.Seal2(PingPongStatsBuffer, u.Uindex)
		err := syscall.Sendto(dataSocketFD, out, 0, addr)
		if err != nil {
			LOG("Index ping error: ", index, err)
			u.Delete.Do(func() { NukeClient(index) })
			continue
		}

		if time.Since(u.Created).Seconds() < 30 {
			continue
		}

		cfg := Config.Load()

		if time.Since(u.LastPingFromClient).Minutes() > float64(cfg.PingTimeoutMinutes) {
			LOG("Ping timeout:", index, "last seen:", time.Since(u.LastPingFromClient).Minutes(), "minutes ago")
			u.Delete.Do(func() { NukeClient(index) })
			continue
		}
	}
}
