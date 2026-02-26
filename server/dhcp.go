package main

import (
	"errors"
	"net"

	"github.com/tunnels-is/tunnels/types"
)

func generateDHCPMap() (err error) {
	var ip net.IP
	ip, VPLNetwork, err = net.ParseCIDR("10.0.0.0/16")
	if err != nil {
		return err
	}

	ip = ip.Mask(VPLNetwork.Mask)

	index := 0
	for VPLNetwork.Contains(ip) {
		DHCPMapping[index] = new(types.DHCPRecord)
		DHCPMapping[index].IP = [4]byte{ip[0], ip[1], ip[2], ip[3]}
		inc(ip)
		index++
	}

	return err
}

func assignDHCP(CR *types.ControllerConnectRequest, CRR *types.ServerConnectResponse, index int) (err error) {
	Config := Config.Load()
	var assigned bool
	for i := range DHCPMapping {
		if DHCPMapping[i] == nil {
			continue
		}
		if DHCPMapping[i].Token == "" {
			continue
		}

		if DHCPMapping[i].Token == CR.DeviceKey || DHCPMapping[i].Token == CR.DeviceToken {
			DHCPMapping[i].AssignHostname(Config.Hostname)
			CRR.DHCP = DHCPMapping[i]

			assigned = true
			cm := clientCoreMappings[index].Load()
			cm.DHCP = DHCPMapping[i]

			ip := cm.DHCP.IP
			VPLIPToCore[uint16(ip[2])<<8|uint16(ip[3])].Store(cm)

			break
		}
	}

	if !assigned {
		for i := range DHCPMapping {
			if DHCPMapping[i] == nil {
				continue
			}

			// Ignore .1 and .0
			if DHCPMapping[i].IP[3] == 1 || DHCPMapping[i].IP[3] == 0 {
				continue
			}

			token := CR.DeviceToken
			if token == "" {
				token = CR.DeviceKey
			}

			assigned = DHCPMapping[i].Assign(float64(Config.DHCPTimeoutHours), token)
			if assigned {
				DHCPMapping[i].AssignHostname(Config.Hostname)
				CRR.DHCP = DHCPMapping[i]
				cm := clientCoreMappings[index].Load()
				cm.DHCP = DHCPMapping[i]

				ip := cm.DHCP.IP
				VPLIPToCore[uint16(ip[2])<<8|uint16(ip[3])].Store(cm)

				break
			}
		}
	}

	if !assigned {
		return errors.New("No DHCP ip address available")
	}

	return err
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
