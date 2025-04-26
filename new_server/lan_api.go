package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/tunnels-is/tunnels/types"
)

func API_Firewall(w http.ResponseWriter, r *http.Request) {
	fr := new(types.FirewallRequest)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(&fr)
	if err != nil {
		senderr(w, 400, fmt.Sprintf("Invalid request body: %v", err), slog.Any("error", err))
		return
	}

	mapping := validateDHCPTokenAndIP(fr)
	if mapping == nil {
		senderr(w, 401, "Unauthorized")
		return
	}

	syncFirewallState(fr, mapping)

	w.WriteHeader(200)
	return
}
func API_ListDevices(w http.ResponseWriter, r *http.Request) {
	if !HTTP_validateKey(r) {
		senderr(w, 401, "Unauthorized")
		return
	}

	response := new(types.DeviceListResponse)
	response.Devices = make([]*types.ListDevice, 0)
outerloop:
	for i := range clientCoreMappings {
		if clientCoreMappings[i] == nil {
			continue
		}

		if clientCoreMappings[i].DHCP != nil {
			for _, v := range response.Devices {
				if v.DHCP.Token == clientCoreMappings[i].DHCP.Token {
					continue outerloop
				}
			}
		}

		d := new(types.ListDevice)
		d.AllowedIPs = make([]string, 0)
		for _, v := range clientCoreMappings[i].AllowedHosts {
			if v.Type == "auto" {
				continue
			}
			d.AllowedIPs = append(d.AllowedIPs,
				fmt.Sprintf("%d-%d-%d-%d",
					v.IP[0],
					v.IP[1],
					v.IP[2],
					v.IP[3],
				))
		}

		d.RAM = clientCoreMappings[i].RAM
		d.CPU = clientCoreMappings[i].CPU
		d.Disk = clientCoreMappings[i].Disk
		if clientCoreMappings[i].DHCP != nil {
			response.DHCPAssigned++
			d.DHCP = *clientCoreMappings[i].DHCP
		}

		d.IngressQueue = len(clientCoreMappings[i].ToUser)
		d.EgressQueue = len(clientCoreMappings[i].FromUser)
		d.Created = clientCoreMappings[i].Created
		if clientCoreMappings[i].PortRange != nil {
			d.StartPort = clientCoreMappings[i].PortRange.StartPort
			d.EndPort = clientCoreMappings[i].PortRange.EndPort
		}
		response.Devices = append(response.Devices, d)
	}

	response.DHCPFree = len(DHCPMapping) - response.DHCPAssigned

	for i := range response.Devices {
		response.Devices[i].DHCP.Token = "redacted"
	}

	w.WriteHeader(200)
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		senderr(w, 500, "encoding error", err)
		return
	}
	return
}
