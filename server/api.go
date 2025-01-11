package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
)

func startAPI(SIGNAL *SIGNAL) {
	defer RecoverAndReturnID(SIGNAL, 1)

	mux := http.NewServeMux()

	mux.HandleFunc("/", HTTP_HealthCheck)
	if VPLEnabled {
		mux.HandleFunc("/firewall", HTTP_Firewall)
	}
	if APIEnabled {
		mux.HandleFunc("/devices", HTTP_ListDevices)
	}

	apiServer := http.Server{
		Handler: mux,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS13,
		},
	}

	addr := fmt.Sprintf("%s:%s",
		Config.ControlIP,
		Config.APIPort,
	)

	ln, err := net.Listen("tcp4", addr)
	if err != nil {
		ERR("HTTP/s API Unable to listen @", addr, " err:", err)
		return
	}

	err = apiServer.ServeTLS(ln, Config.ControlCert, Config.ControlKey)
	if err != nil {
		ERR("HTTP/s API error:", err)
	}
}

func HTTP_validateKey(w http.ResponseWriter, r *http.Request) (ok bool) {
	key := r.Header.Get("X-API-KEY")
	if key != Config.APIKey {
		w.WriteHeader(401)
		return false
	}
	return true
}

func HTTP_ListDevices(w http.ResponseWriter, r *http.Request) {
	if !HTTP_validateKey(w, r) {
		return
	}

	response := new(DeviceListResponse)
	response.Devices = make([]*listDevice, 0)
	for i := range ClientCoreMappings {
		if ClientCoreMappings[i] == nil {
			continue
		}
		d := new(listDevice)
		d.AllowedIPs = make([]string, 0)
		for _, v := range ClientCoreMappings[i].AllowedHosts {
			d.AllowedIPs = append(d.AllowedIPs,
				fmt.Sprintf("%d-%d-%d-%d",
					v.IP[0],
					v.IP[1],
					v.IP[2],
					v.IP[3],
				))
		}

		d.RAM = ClientCoreMappings[i].RAM
		d.CPU = ClientCoreMappings[i].CPU
		d.Disk = ClientCoreMappings[i].Disk
		if ClientCoreMappings[i].DHCP != nil {
			response.DHCPAssigned++
			d.DHCP = ClientCoreMappings[i].DHCP
		}

		d.IngressQueue = len(ClientCoreMappings[i].ToUser)
		d.EgressQueue = len(ClientCoreMappings[i].FromUser)
		d.Created = ClientCoreMappings[i].Created
		if ClientCoreMappings[i].PortRange != nil {
			d.StartPort = ClientCoreMappings[i].PortRange.StartPort
			d.EndPort = ClientCoreMappings[i].PortRange.EndPort
		}
		response.Devices = append(response.Devices, d)
	}

	response.DHCPFree = len(DHCPMapping) - response.DHCPAssigned

	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Encoding error: %s", err)
	}
	r.Body.Close()
	return
}

type FirewallRequest struct {
	DHCPToken string
	IP        string
	Hosts     []string
}

func HTTP_Firewall(w http.ResponseWriter, r *http.Request) {
	fr := new(FirewallRequest)
	err := json.NewDecoder(r.Body).Decode(fr)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Decoding error: %s", err)
		return
	}

	mapping := validateDHCPTokenAndIP(fr)
	if mapping == nil {
		w.WriteHeader(401)
		return
	}

	out := syncFirewallState(fr, mapping)
	if len(out) > 0 {
		outb, err := json.Marshal(out)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Encoding error: %s", err)
			return
		}
		w.WriteHeader(400)
		w.Write(outb)
		return
	}

	w.WriteHeader(200)
	r.Body.Close()
	return
}

func HTTP_HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	r.Body.Close()
	return
}
