package main

import (
	"errors"
	"net"

	"github.com/tunnels-is/tunnels/types"
)

func generateDHCPMap() (err error) {
	Config := Config.Load()
	var ip net.IP
	ip, VPLNetwork, err = net.ParseCIDR(Config.Lan.Network)
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

	return
}

func assignDHCP(CR *types.ControllerConnectRequest, CRR *types.ServerConnectResponse, index int) (err error) {
	Config := Config.Load()
	var assigned bool
	for i := range DHCPMapping {
		if DHCPMapping[i] == nil {
			continue
		}

		if DHCPMapping[i].Token == CR.DeviceKey || DHCPMapping[i].Token == CR.DeviceToken {
			DHCPMapping[i].AssignHostname(Config.Hostname)
			CRR.DHCP = DHCPMapping[i]

			assigned = true
			clientCoreMappings[index].DHCP = DHCPMapping[i]

			ip := clientCoreMappings[index].DHCP.IP
			VPLIPToCore[ip[0]][ip[1]][ip[2]][ip[3]] = clientCoreMappings[index]

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
				clientCoreMappings[index].DHCP = DHCPMapping[i]

				ip := clientCoreMappings[index].DHCP.IP
				VPLIPToCore[ip[0]][ip[1]][ip[2]][ip[3]] = clientCoreMappings[index]

				break
			}
		}
	}

	if !assigned {
		return errors.New("No DHCP ip address available")
	}

	return
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
