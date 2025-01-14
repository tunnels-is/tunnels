package main

import (
	"errors"
	"net"
	"time"
)

func generateDHCPMap() (err error) {
	var ip net.IP
	ip, VPLNetwork, err = net.ParseCIDR(Config.VPL.Network.Network)
	if err != nil {
		return err
	}

	ip = ip.Mask(VPLNetwork.Mask)

	index := 0
	for VPLNetwork.Contains(ip) {
		DHCPMapping[index] = new(DHCPRecord)
		DHCPMapping[index].IP = [4]byte{ip[0], ip[1], ip[2], ip[3]}
		inc(ip)
		index++
	}

	return
}

func assignDHCP(CR *ConnectRequest, CRR *ConnectRequestResponse, index int) (err error) {
	var assigned bool
	if CR.DHCPToken != "" {
		for i := range DHCPMapping {
			if DHCPMapping[i] == nil {
				continue
			}

			if DHCPMapping[i].Token == CR.DHCPToken {
				DHCPMapping[i].AssignHostname(CR.Hostname)
				DHCPMapping[i].Activity = time.Now()

				CRR.DHCP = DHCPMapping[i]

				assigned = true
				ClientCoreMappings[index].DHCP = DHCPMapping[i]

				IPm.Lock()
				IPToCoreMapping[ClientCoreMappings[index].DHCP.IP] = ClientCoreMappings[index]
				IPm.Unlock()

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
				DHCPMapping[i].AssignHostname(CR.Hostname)
				CRR.DHCP = DHCPMapping[i]
				ClientCoreMappings[index].DHCP = DHCPMapping[i]

				IPm.Lock()
				IPToCoreMapping[ClientCoreMappings[index].DHCP.IP] = ClientCoreMappings[index]
				IPm.Unlock()

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
