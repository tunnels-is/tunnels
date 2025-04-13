package main

import (
	"errors"
	"net"
	"time"

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

func assignDHCP(CR *types.ConnectRequest, CRR *types.ConnectRequestResponse, index int) (err error) {
	Config := Config.Load()
	var assigned bool
	if CR.DHCPToken != "" {
		for i := range DHCPMapping {
			if DHCPMapping[i] == nil {
				continue
			}

			if DHCPMapping[i].Token == CR.DHCPToken {
				DHCPMapping[i].AssignHostname(CR.Hostname, Config.Hostname)
				DHCPMapping[i].Activity = time.Now()

				CRR.DHCP = DHCPMapping[i]

				assigned = true
				ClientCoreMappings[index].DHCP = DHCPMapping[i]

				ip := ClientCoreMappings[index].DHCP.IP
				VPLIPToCore[ip[0]][ip[1]][ip[2]][ip[3]] = ClientCoreMappings[index]

				break
			}
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

			assigned = DHCPMapping[i].Assign()
			if assigned {
				DHCPMapping[i].AssignHostname(CR.Hostname, Config.Hostname)
				CRR.DHCP = DHCPMapping[i]
				ClientCoreMappings[index].DHCP = DHCPMapping[i]

				ip := ClientCoreMappings[index].DHCP.IP
				VPLIPToCore[ip[0]][ip[1]][ip[2]][ip[3]] = ClientCoreMappings[index]

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
