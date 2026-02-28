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
}

func API_ListDevices(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	hasAPIKey := HTTP_validateKey(r)
	if !hasAPIKey {
		F := new(FORM_LIST_DEVICE)
		err := decodeBody(r, F)
		if err != nil {
			senderr(w, 400, "Invalid request body", slog.Any("error", err))
			return
		}

		user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
		if err != nil {
			senderr(w, 500, err.Error())
			return
		}
		if !user.IsAdmin {
			if !user.IsManager {
				senderr(w, 401, "You are not allowed to create groups")
				return
			}
		}
	}

	response := new(types.DeviceListResponse)
	response.Devices = make([]*types.ListDevice, 0)
outerloop:
	for i := range clientCoreMappings[:slots] {
		cm := clientCoreMappings[i].Load()
		if cm == nil {
			continue
		}

		if cm.DHCP != nil {
			for _, v := range response.Devices {
				if v.DHCP.Token == cm.DHCP.Token {
					continue outerloop
				}
			}
		}

		d := new(types.ListDevice)
		d.AllowedIPs = make([]string, 0)
		cm.initHosts()
		cm.ManualHosts.Range(func(_ [4]byte, v *AllowedHost) bool {
			d.AllowedIPs = append(d.AllowedIPs,
				fmt.Sprintf("%d-%d-%d-%d",
					v.IP[0],
					v.IP[1],
					v.IP[2],
					v.IP[3],
				))
			return true
		})

		d.RAM = cm.RAM
		d.CPU = cm.CPU
		d.Disk = cm.Disk
		if cm.DHCP != nil {
			response.DHCPAssigned++
			d.DHCP = types.DHCPRecord{
				IP:       cm.DHCP.IP,
				Hostname: cm.DHCP.Hostname,
				Token:    cm.DHCP.Token,
				Activity: cm.DHCP.Activity,
			}
		}

		d.IngressQueue = 0
		d.EgressQueue = 0
		d.Created = cm.Created
		response.Devices = append(response.Devices, d)
	}

	for i := range DHCPMapping {
		if DHCPMapping[i] != nil && DHCPMapping[i].Token == "" {
			response.DHCPFree++
		}
	}

	w.WriteHeader(200)
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		senderr(w, 500, "encoding error", err)
		return
	}
}
