package client

import (
	"bytes"
	"net"
	"strings"
	"time"

	"github.com/jackpal/gateway"
)

// isInterfaceATunnel checks if the given IP is a tunnel interface
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

// loadDefaultInterface discovers and loads the default network interface
func loadDefaultInterface() {
	defer RecoverAndLog()
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

	DEBUG("new default interface discovered", newInterface.To4())
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

// loadDefaultGateway discovers and loads the default network gateway
func loadDefaultGateway() {
	defer RecoverAndLog()
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
	DEBUG("new default gateway discovered", newGateway.To4())
	s.DefaultGateway.Store(&newGateway)

	DEBUG(
		"Default Gateway",
		s.DefaultGateway.Load(),
	)
}

// GetDefaultGateway is a background task that continuously monitors network gateway
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
