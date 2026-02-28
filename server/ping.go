package main

import (
	"time"
)

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

	clientCoreMappings[index].Store(nil)
}

func pingActiveUsers() {
	for index := range clientCoreMappings[:slots] {
		u := clientCoreMappings[index].Load()
		if u == nil {
			continue
		}
		if len(u.Uindex) == 0 {
			continue
		}
		if time.Since(u.Created).Seconds() < 30 {
			continue
		}
		cfg := Config.Load()
		if time.Since(u.LastPingFromClient).Minutes() > float64(cfg.PingTimeoutMinutes) {
			LOG("Ping timeout:", index, "last seen:", time.Since(u.LastPingFromClient).Minutes(), "minutes ago")
			u.Delete.Do(func() { NukeClient(index) })
		}
	}
}
