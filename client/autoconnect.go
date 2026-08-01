package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

type AutoConnectForm struct {
	Country     string         `json:"Country"`
	UserID      string         `json:"UserID"`
	DeviceToken string         `json:"DeviceToken"`
	Tag         string         `json:"Tag"`
	Server      *ControlServer `json:"Server"`
}

type AutoConnectResponse struct {
	ServerTag string `json:"ServerTag"`
	ServerIP  string `json:"ServerIP"`
	LatencyMS int64  `json:"LatencyMS"`
}

const autoConnectPingTimeout = 3 * time.Second

func (f *AutoConnectForm) authHeaders() map[string]string {
	return map[string]string{
		"X-Device-Token": f.DeviceToken,
		"X-UID":          f.UserID,
	}
}

func fetchServersByCountry(form *AutoConnectForm) ([]*types.Server, error) {
	url := form.Server.GetURL("/client/servers/country")
	reqBody := map[string]any{
		"Country":     form.Country,
		"DeviceToken": form.DeviceToken,
		"UID":         form.UserID,
	}
	INFO("auto-connect: fetching servers by country: ", url, " country: ", form.Country)
	respBytes, code, err := SendRequestToURL(nil, "POST", url, reqBody, 15000, form.Server.ValidateCertificate, form.authHeaders())
	if err != nil {
		return nil, fmt.Errorf("fetch servers: %w", err)
	}
	if code != 200 {
		return nil, fmt.Errorf("fetch servers: code=%d body=%s", code, string(respBytes))
	}
	servers := make([]*types.Server, 0)
	if err := json.Unmarshal(respBytes, &servers); err != nil {
		return nil, fmt.Errorf("decode servers: %w", err)
	}
	INFO("auto-connect: controller returned ", len(servers), " server(s) for country ", form.Country)
	return servers, nil
}

func pingICMP(server *types.Server) (time.Duration, bool) {
	ip := net.ParseIP(server.IP)
	if ip == nil {
		addrs, err := net.LookupIP(server.IP)
		if err != nil || len(addrs) == 0 {
			ERROR("auto-connect: ping failed for ", server.Tag, ": unable to resolve ", server.IP)
			return 0, false
		}
		ip = addrs[0]
	}
	ip = ip.To4()
	if ip == nil {
		ERROR("auto-connect: ping failed for ", server.Tag, ": not an IPv4 address: ", server.IP)
		return 0, false
	}

	var dst net.Addr = &net.IPAddr{IP: ip}
	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		conn, err = icmp.ListenPacket("udp4", "0.0.0.0")
		if err != nil {
			ERROR("auto-connect: unable to open ICMP socket: ", err)
			return 0, false
		}
		dst = &net.UDPAddr{IP: ip}
	}
	defer conn.Close()

	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   os.Getpid() & 0xffff,
			Seq:  1,
			Data: []byte("tunnels-auto-connect"),
		},
	}
	msgBytes, err := msg.Marshal(nil)
	if err != nil {
		ERROR("auto-connect: unable to marshal ICMP echo: ", err)
		return 0, false
	}

	start := time.Now()
	if _, err := conn.WriteTo(msgBytes, dst); err != nil {
		ERROR("auto-connect: ping failed for ", server.Tag, " (", server.IP, "): ", err)
		return 0, false
	}
	_ = conn.SetReadDeadline(start.Add(autoConnectPingTimeout))

	reply := make([]byte, 1500)
	for {
		n, peer, err := conn.ReadFrom(reply)
		if err != nil {
			ERROR("auto-connect: ping timeout for ", server.Tag, " (", server.IP, "): ", err)
			return 0, false
		}

		var peerIP net.IP
		switch a := peer.(type) {
		case *net.IPAddr:
			peerIP = a.IP
		case *net.UDPAddr:
			peerIP = a.IP
		}
		if peerIP == nil || !peerIP.Equal(ip) {
			continue
		}

		parsed, err := icmp.ParseMessage(1, reply[:n])
		if err != nil || parsed.Type != ipv4.ICMPTypeEchoReply {
			continue
		}

		latency := time.Since(start)
		INFO("auto-connect: ping ", server.Tag, " (", server.IP, "): ", latency.Milliseconds(), "ms")
		return latency, true
	}
}

func findDeviceByPubKey(form *AutoConnectForm, pubKey string) (*types.Device, error) {
	url := form.Server.GetURL("/client/device/list/user")
	reqBody := map[string]any{
		"DeviceToken": form.DeviceToken,
		"UID":         form.UserID,
	}
	respBytes, code, err := SendRequestToURL(nil, "POST", url, reqBody, 15000, form.Server.ValidateCertificate, form.authHeaders())
	if err != nil || code != 200 {
		return nil, fmt.Errorf("list devices: code=%d err=%v", code, err)
	}
	DEBUG("auto-connect: device list fetched, code: ", code)
	devices := make([]*types.Device, 0)
	if err := json.Unmarshal(respBytes, &devices); err != nil {
		return nil, fmt.Errorf("decode devices: %w", err)
	}
	for _, d := range devices {
		if d.WireGuardKey == pubKey {
			return d, nil
		}
	}
	return nil, nil
}

func deleteDevice(form *AutoConnectForm, device *types.Device) error {
	url := form.Server.GetURL("/client/device/delete")
	reqBody := map[string]any{
		"DeviceID":    device.ID,
		"DeviceToken": form.DeviceToken,
		"UID":         form.UserID,
	}
	_, code, err := SendRequestToURL(nil, "POST", url, reqBody, 15000, form.Server.ValidateCertificate, form.authHeaders())
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("delete device: code=%d", code)
	}
	return nil
}

func ensureDeviceForServer(form *AutoConnectForm, meta *TunnelMETA, server *types.Server) {
	if meta == nil || meta.WireGuardPrivKey == "" {
		return
	}
	pubKey, err := deriveWGPubKey(meta.WireGuardPrivKey)
	if err != nil {
		return
	}
	device, err := findDeviceByPubKey(form, pubKey)
	if err != nil {
		ERROR("auto-connect: device lookup failed: ", err)
		return
	}
	if device == nil {
		INFO("auto-connect: no existing device for tunnel pubkey, one will be created on connect")
		return
	}
	if device.ServerID == server.ID {
		INFO("auto-connect: existing device already bound to ", server.Tag)
		return
	}
	DEBUG("auto-connect: deleting device bound to another server: ", device.ID)
	if err := deleteDevice(form, device); err != nil {
		ERROR("auto-connect: unable to delete mismatched device: ", err)
	}
}

func countryEqual(a, b string) bool {
	norm := func(s string) string {
		s = strings.ToUpper(s)
		if s == "UK" {
			return "GB"
		}
		return s
	}
	return norm(a) == norm(b)
}

func fetchServerByID(form *AutoConnectForm, serverID uuid.UUID) (*types.Server, error) {
	url := form.Server.GetURL("/client/server")
	reqBody := map[string]any{
		"ServerID":    serverID,
		"DeviceToken": form.DeviceToken,
		"UID":         form.UserID,
	}
	respBytes, code, err := SendRequestToURL(nil, "POST", url, reqBody, 15000, form.Server.ValidateCertificate, form.authHeaders())
	if err != nil || code != 200 {
		return nil, fmt.Errorf("fetch server: code=%d err=%v", code, err)
	}
	server := new(types.Server)
	if err := json.Unmarshal(respBytes, server); err != nil {
		return nil, fmt.Errorf("decode server: %w", err)
	}
	return server, nil
}

func findTunnelMetaByTag(tag string) (meta *TunnelMETA) {
	tunnelMetaMapRange(func(tun *TunnelMETA) bool {
		if tun.Tag == tag {
			meta = tun
			return false
		}
		return true
	})
	return
}

func CountryAutoConnect(form *AutoConnectForm) (*AutoConnectResponse, int, error) {
	defer RecoverAndLog()

	if form.Server == nil {
		ERROR("auto-connect: no control server given")
		return nil, 400, errors.New("no control server given")
	}

	if err := authorizeControlServer(form.Server); err != nil {
		return nil, 403, err
	}
	if form.Country == "" {
		ERROR("auto-connect: no country given")
		return nil, 400, errors.New("no country given")
	}
	if form.Tag == "" {
		form.Tag = DefaultTunnelName
	}
	INFO("auto-connect: starting, country: ", form.Country, " tag: ", form.Tag, " controller: ", form.Server.Host, ":", form.Server.Port)

	meta := findTunnelMetaByTag(form.Tag)
	if meta == nil {
		INFO("auto-connect: no tunnel meta found for tag ", form.Tag, ", PublicConnect will report if this is fatal")
	}

	if meta != nil && meta.WireGuardPrivKey != "" {
		if pubKey, err := deriveWGPubKey(meta.WireGuardPrivKey); err == nil {
			device, err := findDeviceByPubKey(form, pubKey)
			if err == nil && device != nil {
				server, err := fetchServerByID(form, device.ServerID)
				if err == nil && server != nil && countryEqual(server.Country, form.Country) {
					INFO("auto-connect: existing device already on ", server.Tag, " in ", form.Country, ", connecting directly")
					latency, _ := pingICMP(server)
					cr := &ConnectionRequest{
						UserID:      form.UserID,
						DeviceToken: form.DeviceToken,
						Tag:         form.Tag,
						ServerID:    server.ID.String(),
						Server:      form.Server,
					}
					code, connErr := PublicConnect(cr)
					if connErr == nil {
						return &AutoConnectResponse{
							ServerTag: server.Tag,
							ServerIP:  server.IP,
							LatencyMS: latency.Milliseconds(),
						}, 200, nil
					}
					ERROR("auto-connect: direct connect via existing device failed (code=", code, "): ", connErr, " — falling back to discovery")
				}
			}
		}
	}

	servers, err := fetchServersByCountry(form)
	if err != nil {
		ERROR("auto-connect: ", err)
		return nil, 502, errors.New("unable to fetch servers from controller")
	}
	if len(servers) == 0 {
		return nil, 404, errors.New("no servers available in country: " + form.Country)
	}

	type probe struct {
		server  *types.Server
		latency time.Duration
	}
	reachable := make([]probe, 0, len(servers))
	for _, s := range servers {
		if latency, ok := pingICMP(s); ok {
			reachable = append(reachable, probe{server: s, latency: latency})
		}
	}
	if len(reachable) == 0 {
		ERROR("auto-connect: no servers responded to ping (", len(servers), " tried)")
		return nil, 502, errors.New("no servers responded to ping")
	}
	sort.Slice(reachable, func(i, j int) bool { return reachable[i].latency < reachable[j].latency })

	var lastErr error
	lastCode := 502
	for _, p := range reachable {
		INFO("auto-connect: attempting ", p.server.Tag, " (", p.server.IP, ", ", p.latency.Milliseconds(), "ms)")
		ensureDeviceForServer(form, meta, p.server)

		cr := &ConnectionRequest{
			UserID:      form.UserID,
			DeviceToken: form.DeviceToken,
			Tag:         form.Tag,
			ServerID:    p.server.ID.String(),
			Server:      form.Server,
		}
		code, err := PublicConnect(cr)
		if err == nil {
			INFO("auto-connect: connected to ", p.server.Tag, " (", p.latency.Milliseconds(), "ms)")
			return &AutoConnectResponse{
				ServerTag: p.server.Tag,
				ServerIP:  p.server.IP,
				LatencyMS: p.latency.Milliseconds(),
			}, 200, nil
		}
		ERROR("auto-connect: failed to connect to ", p.server.Tag, ": ", err)
		lastErr = err
		lastCode = code
	}

	return nil, lastCode, fmt.Errorf("unable to connect to any server: %w", lastErr)
}
