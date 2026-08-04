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
	Country   string `json:"Country"`
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

func fetchAllServers(form *AutoConnectForm) ([]*types.Server, error) {
	url := form.Server.GetURL("/client/servers")
	reqBody := map[string]any{
		"StartIndex":  0,
		"DeviceToken": form.DeviceToken,
		"UID":         form.UserID,
	}
	INFO("auto-connect: fetching full server list: ", url)
	respBytes, code, err := SendRequestToURL(nil, "POST", url, reqBody, 15000, form.Server.ValidateCertificate, form.authHeaders())
	if err != nil {
		return nil, fmt.Errorf("fetch all servers: %w", err)
	}
	if code != 200 {
		return nil, fmt.Errorf("fetch all servers: code=%d body=%s", code, string(respBytes))
	}
	servers := make([]*types.Server, 0)
	if err := json.Unmarshal(respBytes, &servers); err != nil {
		return nil, fmt.Errorf("decode all servers: %w", err)
	}
	INFO("auto-connect: controller returned ", len(servers), " server(s) total")
	return servers, nil
}

type latencyProbe struct {
	server  *types.Server
	latency time.Duration
}

func probeServersByLatency(servers []*types.Server) []latencyProbe {
	reachable := make([]latencyProbe, 0, len(servers))
	for _, s := range servers {
		if s == nil {
			continue
		}
		if latency, ok := pingICMP(s); ok {
			reachable = append(reachable, latencyProbe{server: s, latency: latency})
		}
	}
	sort.Slice(reachable, func(i, j int) bool { return reachable[i].latency < reachable[j].latency })
	return reachable
}

func connectInLatencyOrder(form *AutoConnectForm, servers []*types.Server) (*AutoConnectResponse, int, error) {
	ordered := probeServersByLatency(servers)
	if len(ordered) == 0 {
		ERROR("auto-connect: no servers responded to ping (", len(servers), " tried)")
		return nil, 502, errors.New("no servers responded to ping")
	}

	var lastErr error
	lastCode := 502
	for _, p := range ordered {
		latencyMS := p.latency.Milliseconds()
		INFO("auto-connect: attempting ", p.server.Tag, " (", p.server.IP, ", ", latencyMS, "ms)")

		cr := &ConnectionRequest{
			UserID:      form.UserID,
			DeviceToken: form.DeviceToken,
			Tag:         form.Tag,
			ServerID:    p.server.ID.String(),
			Server:      form.Server,
		}
		code, err := PublicConnect(cr)
		if err == nil {
			INFO("auto-connect: connected to ", p.server.Tag, " (", latencyMS, "ms)")
			return &AutoConnectResponse{
				ServerTag: p.server.Tag,
				ServerIP:  p.server.IP,
				Country:   p.server.Country,
				LatencyMS: latencyMS,
			}, 200, nil
		}
		ERROR("auto-connect: failed to connect to ", p.server.Tag, ": ", err)
		lastErr = err
		lastCode = code
	}
	return nil, lastCode, fmt.Errorf("unable to connect to any server: %w", lastErr)
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

// preferLocalServerInCountry tries servers we already have a local device for
// in the requested country (avoids probing when we can reconnect quickly).
func preferLocalServerInCountry(form *AutoConnectForm, country string) (*types.Server, int64, bool) {
	locals, err := listLocalDevices()
	if err != nil || len(locals) == 0 {
		return nil, 0, false
	}
	for _, d := range locals {
		if d.ServerID == "" {
			continue
		}
		sid, err := uuid.Parse(d.ServerID)
		if err != nil {
			continue
		}
		server, err := fetchServerByID(form, sid)
		if err != nil || server == nil || !countryEqual(server.Country, country) {
			continue
		}
		latency, ok := pingICMP(server)
		if !ok {
			continue
		}
		INFO("auto-connect: local device ", d.ID, " already for ", server.Tag, " in ", country)
		return server, latency.Milliseconds(), true
	}
	return nil, 0, false
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

// discoverBestServer picks the best server without connecting.
// Order: country match (local device → country list → subset of full list) then full-list ping probe.
func discoverBestServer(form *AutoConnectForm) (*types.Server, int64, int, error) {
	if form.Country != "" {
		if server, latencyMS, ok := preferLocalServerInCountry(form, form.Country); ok {
			return server, latencyMS, 200, nil
		}

		countryServers, countryErr := fetchServersByCountry(form)
		if countryErr != nil {
			ERROR("auto-connect: country fetch failed: ", countryErr, " — falling back to full list")
		} else if len(countryServers) > 0 {
			INFO("auto-connect: country match — probing ", len(countryServers), " server(s) in ", form.Country)
			ordered := probeServersByLatency(countryServers)
			if len(ordered) > 0 {
				return ordered[0].server, ordered[0].latency.Milliseconds(), 200, nil
			}
			ERROR("auto-connect: no country servers responded to ping — falling back to full list")
		} else {
			INFO("auto-connect: no servers in country ", form.Country, " — full list ping probe")
		}
	}

	allServers, err := fetchAllServers(form)
	if err != nil {
		ERROR("auto-connect: ", err)
		return nil, 0, 502, errors.New("unable to fetch servers from controller")
	}
	if len(allServers) == 0 {
		return nil, 0, 404, errors.New("no servers available")
	}

	if form.Country != "" {
		matched := make([]*types.Server, 0)
		for _, s := range allServers {
			if s != nil && countryEqual(s.Country, form.Country) {
				matched = append(matched, s)
			}
		}
		if len(matched) > 0 {
			INFO("auto-connect: country match from full list — probing ", len(matched), " server(s) in ", form.Country)
			ordered := probeServersByLatency(matched)
			if len(ordered) > 0 {
				return ordered[0].server, ordered[0].latency.Milliseconds(), 200, nil
			}
			ERROR("auto-connect: country subset from full list failed ping — probing all servers")
		}
	}

	INFO("auto-connect: ping probing ", len(allServers), " server(s)")
	ordered := probeServersByLatency(allServers)
	if len(ordered) == 0 {
		return nil, 0, 502, errors.New("no servers responded to ping")
	}
	return ordered[0].server, ordered[0].latency.Milliseconds(), 200, nil
}

func prepareAutoConnectForm(form *AutoConnectForm) (int, error) {
	if form.Server == nil {
		ERROR("auto-connect: no control server given")
		return 400, errors.New("no control server given")
	}
	if err := authorizeControlServer(form.Server); err != nil {
		return 403, err
	}
	if form.Tag == "" {
		form.Tag = DefaultTunnelName
	}
	if form.UserID != "" {
		if err := activateAccountByUserID(form.UserID); err != nil {
			ERROR("auto-connect: unable to activate account workspace: ", err)
			return 500, errors.New("unable to activate account workspace")
		}
	}
	return 0, nil
}

// ProbeBestServer runs country match + ping probe and returns the winner without connecting.
func ProbeBestServer(form *AutoConnectForm) (*AutoConnectResponse, int, error) {
	defer RecoverAndLog()
	if code, err := prepareAutoConnectForm(form); err != nil {
		return nil, code, err
	}
	INFO("auto-connect probe: country=", form.Country, " controller=", form.Server.Host, ":", form.Server.Port)

	server, latencyMS, code, err := discoverBestServer(form)
	if err != nil {
		return nil, code, err
	}
	return &AutoConnectResponse{
		ServerTag: server.Tag,
		ServerIP:  server.IP,
		Country:   server.Country,
		LatencyMS: latencyMS,
	}, 200, nil
}

func CountryAutoConnect(form *AutoConnectForm) (*AutoConnectResponse, int, error) {
	defer RecoverAndLog()

	if code, err := prepareAutoConnectForm(form); err != nil {
		return nil, code, err
	}
	INFO("auto-connect: starting, country: ", form.Country, " tag: ", form.Tag, " controller: ", form.Server.Host, ":", form.Server.Port)

	server, latencyMS, code, err := discoverBestServer(form)
	if err != nil {
		return nil, code, err
	}

	// Connect to the winner first; if that fails, try remaining candidates in latency order via full reconnect path.
	cr := &ConnectionRequest{
		UserID:      form.UserID,
		DeviceToken: form.DeviceToken,
		Tag:         form.Tag,
		ServerID:    server.ID.String(),
		Server:      form.Server,
	}
	connCode, connErr := PublicConnect(cr)
	if connErr == nil {
		return &AutoConnectResponse{
			ServerTag: server.Tag,
			ServerIP:  server.IP,
			Country:   server.Country,
			LatencyMS: latencyMS,
		}, 200, nil
	}
	ERROR("auto-connect: failed to connect to best server ", server.Tag, ": ", connErr, " — trying remaining fleet")

	// Fall back: re-probe fleet and try each reachable server until one connects.
	allServers, ferr := fetchAllServers(form)
	if ferr != nil {
		return nil, connCode, fmt.Errorf("unable to connect to best server: %w", connErr)
	}
	// Skip the one we already failed.
	remaining := make([]*types.Server, 0, len(allServers))
	for _, s := range allServers {
		if s != nil && s.ID != server.ID {
			remaining = append(remaining, s)
		}
	}
	if len(remaining) == 0 {
		return nil, connCode, fmt.Errorf("unable to connect to best server: %w", connErr)
	}
	return connectInLatencyOrder(form, remaining)
}
