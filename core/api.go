package core

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tunnels-is/tunnels/certs"
	"github.com/xlzd/gotp"
	"github.com/zveinn/crypt"
	"golang.org/x/net/quic"
)

// func OpenProxyTunnelToRouter() (TcpConn net.Conn, err error) {
// 	dialer := net.Dialer{Timeout: time.Duration(5 * time.Second)}
// 	routerIndexForConnection := 0
// 	retryRouterCount := 0
//
// retryRouter:
// 	IP := GLOBAL_STATE.RouterList[routerIndexForConnection].IP
// 	TcpConn, err = dialer.Dial("tcp", IP+":443")
// 	if err != nil {
// 		CreateErrorLog("", "Could not dial router: ", IP, err)
// 		if retryRouterCount > 2 {
// 			CreateErrorLog("", "Could not dial router (final retry) backing off: ", IP, err)
// 			return
// 		}
// 		retryRouterCount++
// 		routerIndexForConnection++
// 		if routerIndexForConnection == len(GLOBAL_STATE.RouterList) {
// 			routerIndexForConnection = 0
// 		}
// 		goto retryRouter
// 	}
//
// 	return
// }

func ResetEverything() {
	defer RecoverAndLogToFile()

	for _, v := range TunList {
		if v == nil {
			continue
		}
		if v.Interface != nil {
			_ = v.Interface.Disconnect(v)
		}
	}

	RestoreSaneDNSDefaults()
}

func sendFirewallToServer(serverIP string, DHCPToken string, DHCPIP string, allowedHosts []string, disableFirewall bool) (err error) {
	FR := new(FirewallRequest)
	FR.DHCPToken = DHCPToken
	FR.IP = DHCPIP
	FR.Hosts = allowedHosts

	if CLIDisableVPLFirewall {
		FR.DisableFirewall = true
	} else {
		FR.DisableFirewall = disableFirewall
	}

	var body []byte
	body, err = json.Marshal(FR)
	if err != nil {
		return err
	}

	DEBUG("Firewall disabled:", FR.DisableFirewall, ">> allowed hosts:", FR.Hosts)

	DEBUG("Sending firewall info to server: ", serverIP)
	req, err := http.NewRequest("POST", "https://"+serverIP+":444/firewall", bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	client := http.Client{
		Timeout: time.Duration(5000) * time.Millisecond,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				CurvePreferences:   []tls.CurveID{tls.CurveP521},
				RootCAs:            CertPool,
				MinVersion:         tls.VersionTLS13,
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	client.CloseIdleConnections()
	if resp != nil {
		if resp.Body != nil {
			defer resp.Body.Close()
		}

		if resp.StatusCode != 200 {
			return fmt.Errorf("error from server (%s) while applying firewall rules, code: %d", serverIP, resp.StatusCode)
		}
	} else {
		return fmt.Errorf("no response from server(%s) while applying firewall rules", serverIP)
	}

	return nil
}

func SendRequestToController(method string, route string, data interface{}, timeoutMS int) ([]byte, int, error) {
	defer RecoverAndLogToFile()

	var body []byte
	var err error
	if data != nil {
		body, err = json.Marshal(data)
		if err != nil {
			return nil, 0, err
		}
	}

	var req *http.Request
	if method == "POST" {
		req, err = http.NewRequest(method, "https://api.tunnels.is/"+route, bytes.NewBuffer(body))
	} else if method == "GET" {
		req, err = http.NewRequest(method, "https://api.tunnels.is/"+route, nil)
	} else {
		return nil, 0, errors.New("method not supported:" + method)
	}

	if err != nil {
		return nil, 0, err
	}

	req.Header.Add("Content-Type", "application/json")

	client := http.Client{
		Timeout: time.Duration(timeoutMS) * time.Millisecond,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				CurvePreferences:   []tls.CurveID{tls.CurveP521},
				RootCAs:            CertPool,
				MinVersion:         tls.VersionTLS13,
				InsecureSkipVerify: false,
			},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		if resp != nil {
			return nil, resp.StatusCode, err
		} else {
			return nil, 0, err
		}
	}

	client.CloseIdleConnections()
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	var x []byte
	x, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return x, resp.StatusCode, nil
}

func ForwardConnectToController(FR *FORWARD_REQUEST) ([]byte, int) {
	defer RecoverAndLogToFile()

	// The domain being used here is an old domain that needs to be replaced.
	// This method uses a custom dialer which does not DNS resolve.
	responseBytes, code, err := SendRequestToController(
		FR.Method,
		FR.Path,
		FR.JSONData,
		FR.Timeout,
	)
	if err != nil {
		ERROR("Could not forward request (err): ", err)
		if code == 0 {
			return responseBytes, 420
		} else {
			return responseBytes, code
		}
	}

	if code == 0 {
		ERROR("Could not forward request (code 0): ", err)
		return responseBytes, 420
	}

	return responseBytes, code
}

func ForwardToController(FR *FORWARD_REQUEST) (interface{}, int) {
	defer RecoverAndLogToFile()

	// The domain being used here is an old domain that needs to be replaced.
	// This method uses a custom dialer which does not DNS resolve.
	responseBytes, code, err := SendRequestToController(
		FR.Method,
		FR.Path,
		FR.JSONData,
		FR.Timeout,
	)

	er := new(ErrorResponse)
	if err != nil {
		er.Error = err.Error()
		ERROR("Could not forward request (err): ", err)
		return er, 500
	}

	if code == 0 {
		er.Error = "Unable to contact controller"
		ERROR("Could not forward request (code 0): ", err)
		return er, 500
	}

	var respJSON interface{}
	if len(responseBytes) != 0 {
		err = json.Unmarshal(responseBytes, &respJSON)
		if err != nil {
			ERROR("Could not parse response data: ", err)
			er.Error = "Unable to open response from controller"
			return er, code
		}
	}

	if strings.Contains(FR.Path, "logout") {
		if len(responseBytes) != 0 && code == 200 {
			for i := range STATEOLD.ActiveConnections {
				if STATEOLD.ActiveConnections[i] == nil {
					continue
				}
				Disconnect(STATEOLD.ActiveConnections[i].WindowsGUID, true, false)
			}
			STATEOLD.User = User{}
		}
	}

	if strings.Contains(FR.Path, "login") {
		if len(responseBytes) != 0 && code == 200 {
			err = json.Unmarshal(responseBytes, &STATEOLD.User)
			if err != nil {
				ERROR("login detected but not registered in background service: ", err)
			} else {
				DEBUG("login registered in background service")
			}
		}
	}

	return respJSON, code
}

var (
	NEXT_SERVER_REFRESH time.Time
	AZ_CHAR_CHECK       = regexp.MustCompile(`^[a-zA-Z0-9]*$`)
)

func validateConfig(config *Config) (err error) {
	curMeta := findDefaultTunnelMeta()
	if curMeta == nil {
		return errors.New("no current configuration found, please restart tunnels")
	}

	ifnamemap := make(map[string]struct{})
	tagnamemap := make(map[string]struct{})
	var newMeta *TunnelMETA
	for i, v := range config.Connections {
		if v == nil {
			continue
		}

		ifFail := AZ_CHAR_CHECK.MatchString(v.IFName)
		if !ifFail {
			return errors.New("interface names can only contain a-z A-Z 0-9")
		}

		_, ok := ifnamemap[strings.ToLower(v.IFName)]
		if ok {
			return errors.New("you cannot have two connections with the same interface name: " + v.IFName)
		}
		ifnamemap[strings.ToLower(v.IFName)] = struct{}{}

		_, ok = tagnamemap[strings.ToLower(v.Tag)]
		if ok {
			return errors.New("you cannot have two connections with the same tag: " + v.Tag)
		}
		tagnamemap[strings.ToLower(v.Tag)] = struct{}{}

		if strings.EqualFold(v.IFName, DefaultTunnelName) {
			newMeta = config.Connections[i]
		}
	}

	if newMeta == nil {
		return errors.New("your updated configurations do not include a connection with the IFName `tunnels`, please create a default conenction or use the config recovery tool")
	}

	if len(newMeta.IFName) < 3 {
		return errors.New("default connections interface name needs to be at least 3 characters")
	}

	if newMeta.MTU < 1400 {
		return errors.New("connection MTU should not be less then 1400")
	}

	if newMeta.TxQueueLen < 500 {
		return errors.New("connection TxQueueLen should not be less then 500")
	}

	err = ValidateAdapterID(newMeta)
	if err != nil {
		return err
	}

	IP := net.ParseIP(newMeta.IPv4Address)
	if IP == nil {
		return errors.New("IP Address on default connection is invalid")
	}

	return nil
}

func SetConfig(config *Config) error {
	defer RecoverAndLogToFile()

	err := validateConfig(config)
	if err != nil {
		return err
	}

	if !config.DebugLogging || !config.InfoLogging {
	loop:
		for {
			select {
			case <-APILogQueue:
			default:
				break loop
			}
		}
	}

	oldDNSIP := STATEOLD.C.DNSServerIP
	oldDNSPort := STATEOLD.C.DNSServerPort
	oldAPIIP := STATEOLD.C.APIIP
	oldAPIPort := STATEOLD.C.APIPort
	oldAPICert := STATEOLD.C.APICert
	oldAPIKey := STATEOLD.C.APIKey
	oldCertDomains := STATEOLD.C.APICertDomains
	oldCertIPs := STATEOLD.C.APICertIPs
	oldBlocklists := STATEOLD.C.AvailableBlockLists

	if !CLIDisableBlockLists {
		if len(oldBlocklists) != len(config.AvailableBlockLists) || !CheckBlockListsEquality(oldBlocklists, config.AvailableBlockLists) {
			DEBUG("Updating DNS Blocklists...")
			err := ReBuildBlockLists(config)
			if err != nil {
				ERROR("Error updating DNS block lists ", err)
				return err
			}
		}
	}

	err = applyNewFirewallRules(C, config)
	if err != nil {
		return err
	}

	err = SaveConfig(config)
	if err != nil {
		ERROR("Unable to save config: ", err)
		return errors.New("unable to save config")
	}
	SwapConfig(config)

	dnsChange := false
	if config.DNSServerPort != oldDNSPort {
		dnsChange = true
	}
	if config.DNSServerIP != oldDNSIP {
		dnsChange = true
	}
	if dnsChange {
		_ = UDPDNSServer.Shutdown()
	}

	apiChange := false
	if config.APIPort != oldAPIPort {
		apiChange = true
	}
	if config.APIIP != oldAPIIP {
		apiChange = true
	}
	if config.APICert != oldAPICert {
		apiChange = true
	}
	if config.APIKey != oldAPIKey {
		apiChange = true
	}
	if !slices.Equal(oldCertDomains, config.APICertDomains) {
		apiChange = true
	}
	if !slices.Equal(oldCertIPs, config.APICertIPs) {
		apiChange = true
	}
	if apiChange {
		_ = API_SERVER.Shutdown(context.Background())
	}

	INFO(fmt.Sprintf("%+v", *config))
	return nil
}

func BandwidthBytesToString(b uint64) string {
	if b <= 999 {
		intS := strconv.FormatUint(b, 10)
		return intS + " B"
	} else if b <= 999_999 {
		intF := float64(b)
		return fmt.Sprintf("%.0f KB", intF/1000)
	} else if b <= 999_999_999 {
		intF := float64(b)
		return fmt.Sprintf("%.1f MB", intF/1_000_000)
	} else if b <= 999_999_999_999 {
		intF := float64(b)
		return fmt.Sprintf("%.1f GB", intF/1_000_000_000)
	} else if b <= 999_999_999_999_999 {
		intF := float64(b)
		return fmt.Sprintf("%.1f TB", intF/1_000_000_000_000)
	}

	return "???"
}

func applyNewFirewallRules(originalConfig *Config, newConfig *Config) error {
	for _, oc := range originalConfig.Connections {
		for _, nc := range newConfig.Connections {
			if oc.WindowsGUID == nc.WindowsGUID {
				if !slices.Equal(oc.AllowedHosts, nc.AllowedHosts) {
					t := findTunnelByGUID(nc.WindowsGUID)
					if t != nil {
						sendFirewallToServer(
							t.ClientCR.ServerIP,
							t.DHCP.Token,
							net.IP(t.DHCP.IP[:]).String(),
							nc.AllowedHosts,
							nc.DisableFirewall,
						)
					}
				}
			}
		}
	}
	return nil
}

func GenerateState() (err error) {
	defer RecoverAndLogToFile()
	DEBUG("Generating state object")

	STATEOLD.ActiveConnections = make([]*TunnelMETA, 0)
	STATEOLD.ConnectionStats = make([]TunnelSTATS, 0)
	STATEOLD.Version = APP_VERSION

	for i := range TunList {
		if TunList[i] == nil {
			continue
		}

		STATEOLD.ActiveConnections = append(STATEOLD.ActiveConnections, TunList[i].Meta)
		var n2 uint64 = 0
		if len(TunList[i].Nonce2Bytes) > 7 {
			n2 = binary.BigEndian.Uint64(TunList[i].Nonce2Bytes)
		}

		x := TunnelSTATS{
			Nonce1:              TunList[i].EH.SEAL.Nonce1U.Load(),
			Nonce2:              n2,
			StartPort:           TunList[i].StartPort,
			EndPort:             TunList[i].EndPort,
			IngressString:       BandwidthBytesToString(uint64(TunList[i].IngressBytes)),
			EgressString:        BandwidthBytesToString(uint64(TunList[i].EgressBytes)),
			IngressBytes:        TunList[i].IngressBytes,
			EgressBytes:         TunList[i].EgressBytes,
			StatsTag:            TunList[i].Meta.Tag,
			DISK:                TunList[i].TunnelSTATS.DISK,
			MEM:                 TunList[i].TunnelSTATS.MEM,
			CPU:                 TunList[i].TunnelSTATS.CPU,
			ServerToClientMicro: TunList[i].TunnelSTATS.ServerToClientMicro,
			PingTime:            TunList[i].TunnelSTATS.PingTime,
		}

		if TunList[i].DHCP != nil {
			x.DHCP = TunList[i].DHCP
		}
		if TunList[i].VPLNetwork != nil {
			x.VPLNetwork = TunList[i].VPLNetwork
		}

		STATEOLD.ConnectionStats = append(STATEOLD.ConnectionStats, x)
	}

	if STATEOLD.C.DNSstats {

		for i, v := range DNSBlockedList {
			STATEOLD.DNSBlocksMap[i] = v
		}
		for i, v := range DNSResolvedList {
			STATEOLD.DNSResolvesMap[i] = v
		}
	}

	return
}

func InitializeTunnelFromCRR(TUN *Tunnel) (err error) {
	BLOCK_DNS_QUERIES = true
	defer func() {
		RecoverAndLogToFile()
		BLOCK_DNS_QUERIES = false
	}()

	FullCleanDNSCache()

	TUN.Index = make([]byte, 2)
	binary.BigEndian.PutUint16(TUN.Index, uint16(TUN.CRR.Index))

	TUN.AddressNetIP = net.ParseIP(TUN.Meta.IPv4Address).To4()
	TUN.StartPort = TUN.CRR.StartPort
	TUN.EndPort = TUN.CRR.EndPort
	TUN.TCP_EM = make(map[[10]byte]*Mapping)
	TUN.UDP_EM = make(map[[10]byte]*Mapping)
	TUN.InitPortMap()

	ifip := net.ParseIP(TUN.CRR.InterfaceIP)
	if ifip == nil {
		return fmt.Errorf("Interface ip (%s) was malformed", TUN.CRR.InterfaceIP)
	}

	to4 := ifip.To4()
	TUN.EP_VPNSrcIP[0] = to4[0]
	TUN.EP_VPNSrcIP[1] = to4[1]
	TUN.EP_VPNSrcIP[2] = to4[2]
	TUN.EP_VPNSrcIP[3] = to4[3]

	if TUN.CRR.DHCP != nil {
		TUN.VPL_IP[0] = TUN.CRR.DHCP.IP[0]
		TUN.VPL_IP[1] = TUN.CRR.DHCP.IP[1]
		TUN.VPL_IP[2] = TUN.CRR.DHCP.IP[2]
		TUN.VPL_IP[3] = TUN.CRR.DHCP.IP[3]

		TUN.DHCP = TUN.CRR.DHCP
		TUN.Meta.DHCPToken = TUN.CRR.DHCP.Token
		SaveConfig(STATEOLD.C)
	}

	if TUN.CRR.VPLNetwork != nil {
		TUN.VPLNetwork = TUN.CRR.VPLNetwork
	}

	if TUN.Meta.LocalhostNat {
		NN := new(ServerNetwork)
		NN.Network = "127.0.0.1/32"
		NN.Nat = to4.String() + "/32"
		TUN.CRR.Networks = append(TUN.CRR.Networks, NN)
	}

	if len(TUN.Meta.Networks) > 0 {
		TUN.CRR.Networks = TUN.Meta.Networks
	}
	if len(TUN.Meta.DNS) > 0 {
		TUN.CRR.DNS = TUN.Meta.DNS
	}
	if len(TUN.Meta.DNSServers) > 0 {
		TUN.CRR.DNSServers = TUN.Meta.DNSServers
	}
	if len(TUN.CRR.DNSServers) < 1 {
		TUN.CRR.DNSServers = []string{C.DNS1Default, C.DNS2Default}
	}

	err = TUN.InitVPLMap()
	if err != nil {
		return err
	}
	err = TUN.InitNatMaps()
	if err != nil {
		return err
	}

	DEBUG(fmt.Sprintf(
		"Connection info: Addr(%s) StartPort(%d) EndPort(%d) srcIP(%s) ",
		TUN.Meta.IPv4Address,
		TUN.CRR.StartPort,
		TUN.CRR.EndPort,
		TUN.CRR.InterfaceIP,
	))

	if TUN.CRR.VPLNetwork != nil && TUN.CRR.DHCP != nil {
		DEBUG(fmt.Sprintf(
			"DHCP/VPL info: Addr(%s) Network:(%s) Token(%s) ",
			TUN.CRR.DHCP.IP,
			TUN.CRR.VPLNetwork.Network,
			TUN.CRR.DHCP.Token,
		))
	}

	return nil
}

func PreConnectCheck() (int, error) {
	if !STATEOLD.IsAdmin {
		return 400, errors.New("tunnels needs to run as Administrator or root")
	}
	return 0, nil
}

var IsConnecting = atomic.Bool{}

func PublicConnect(ClientCR ConnectionRequest) (code int, errm error) {
	if !IsConnecting.CompareAndSwap(false, true) {
		INFO("Already connecting to another connection, please wait a moment")
		return 400, errors.New("Already connecting to another connection, please wait a moment")
	}

	start := time.Now()
	defer func() {
		IsConnecting.Store(false)
		runtime.GC()
	}()
	defer RecoverAndLogToFile()

	code, errm = PreConnectCheck()
	if errm != nil {
		return
	}

	tunnel := new(Tunnel)
	tunnel.ClientCR = ClientCR
	tunnel.Meta = FindMETAForConnectRequest(&ClientCR)
	if tunnel.Meta == nil {
		ERROR("vpn connection metadata not found for tag: ", ClientCR.Tag)
		return 400, errors.New("error fetching connection meta")
	}

	if tunnel.Meta.DNSDiscovery != "" {
		DEBUG("looking for connection info @ ", tunnel.Meta.DNSDiscovery)
		dnsInfo, err := certs.ResolveMetaTXT(tunnel.Meta.DNSDiscovery)
		if err != nil {
			ERROR("error looking up connection info: ", err)
			return 400, err
		}
		DEBUG("DNS Info: ", dnsInfo.IP, dnsInfo.Port, dnsInfo.ServerID, "cert length: ", len(dnsInfo.Cert))
		ClientCR.ServerPort = dnsInfo.Port
		ClientCR.ServerIP = dnsInfo.IP
		ClientCR.ServerID = dnsInfo.ServerID
		tunnel.Meta.PrivateCertBytes = dnsInfo.Cert
		tunnel.Meta.Private = true
		tunnel.ClientCR = ClientCR

	} else {
		if !tunnel.Meta.Private && ClientCR.ServerID == "" {
			ERROR("No server selected")
			return 400, errors.New("No server selected")
		} else if tunnel.Meta.ServerID != ClientCR.ServerID {
			tunnel.Meta.ServerID = ClientCR.ServerID
		}
	}

	if ClientCR.ServerIP == "" || ClientCR.ServerPort == "" {
		ERROR("Missing server or port in connect request")
		return 400, errors.New("Server IP or Port missing")
	}

	if tunnel.Meta.PreventIPv6 && IPv6Enabled() {
		return 400, errors.New("IPV6 Enabled but should be disabled")
	}

	FinalCR := new(RemoteConnectionRequest)
	FinalCR.Version = API_VERSION
	FinalCR.Created = time.Now()

	// from GUI connect request
	FinalCR.DeviceToken = ClientCR.DeviceToken
	FinalCR.Hostname = tunnel.Meta.Hostname
	FinalCR.UserID = ClientCR.UserID
	FinalCR.SeverID = ClientCR.ServerID
	FinalCR.EncType = ClientCR.EncType
	FinalCR.CurveType = ClientCR.CurveType
	FinalCR.DeviceKey = ClientCR.DeviceKey

	if !tunnel.Meta.Private || tunnel.Meta.RequestVPNPorts {
		FinalCR.RequestingPorts = true
	}
	FinalCR.DHCPToken = tunnel.Meta.DHCPToken

	DEBUG("ConnectRequestFromClient", ClientCR)

	tc := &tls.Config{
		RootCAs:            CertPool,
		MinVersion:         tls.VersionTLS13,
		CurvePreferences:   []tls.CurveID{tls.X25519MLKEM768},
		InsecureSkipVerify: false,
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			return nil
		},
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) > 0 {
				FinalCR.Serial = fmt.Sprintf("%x", cs.PeerCertificates[0].SerialNumber)
			}
			return nil
		},
	}

	if tunnel.Meta.Private {
		tc.RootCAs, errm = tunnel.Meta.LoadPrivateCerts()
		if errm != nil {
			ERROR("Unable to load private cert: ", errm)
			return 502, errors.New("Unable to load private cert: " + tunnel.Meta.PrivateCert)
		}
	}

	qc := &quic.Config{
		TLSConfig:                tc,
		HandshakeTimeout:         time.Duration(15 * time.Second),
		RequireAddressValidation: false,
		KeepAlivePeriod:          0,
		MaxUniRemoteStreams:      10,
		MaxBidiRemoteStreams:     10,
		MaxStreamReadBufferSize:  70000,
		MaxStreamWriteBufferSize: 70000,
		MaxConnReadBufferSize:    70000,
		MaxIdleTimeout:           60 * time.Second,
	}

	x, err := quic.Listen("udp4", "", qc)
	if err != nil {
		ERROR("Unable to open UDP listener:", err)
		return 502, errors.New("Unable to create udp listener")
	}

	DEBUG("ConnectingTo:", net.JoinHostPort(ClientCR.ServerIP, ClientCR.ServerPort))
	con, err := x.Dial(
		context.Background(),
		"udp4",
		net.JoinHostPort(ClientCR.ServerIP, ClientCR.ServerPort),
		qc,
	)
	if err != nil {
		x.Close(context.Background())
		DEBUG("ConnectionError:", err)
		return 502, errors.New("unable to connect to server")
	}

	FR := new(FORWARD_REQUEST)
	FR.Method = "POST"
	if tunnel.Meta.Private {
		FR.Path = "v3/session/private"
	} else {
		FR.Path = "v3/session/public"
	}
	if ClientCR.ServerID != "" && ClientCR.DeviceKey != "" {
		FR.Path += "/min"
	}

	FR.JSONData = FinalCR
	FR.Timeout = 25000

	bytesFromController, code := ForwardConnectToController(FR)
	DEBUG("CodeFromController:", code)
	if code != 200 {
		x.Close(context.Background())
		con.Abort(errors.New(""))
		ERROR("ErrFromController:", string(bytesFromController))
		ER := new(ErrorResponse)
		err := json.Unmarshal(bytesFromController, ER)
		if err != nil {
			return code, errors.New(ER.Error)
		} else {
			return code, errors.New("Unknown error from controller")
		}
	}

	DEBUG("SignedPayload:", string(bytesFromController))

	s, err := con.NewStream(context.Background())
	if err != nil {
		x.Close(context.Background())
		con.Abort(errors.New(""))
		DEBUG("StreamError:", err)
		return 502, errors.New("unable to make stream")
	}

	closeAll := func() {
		s.Close()
		con.Close()
		x.Close(context.Background())
	}

	_, err = s.Write(bytesFromController)
	if err != nil {
		closeAll()
		DEBUG("WriteError:", err)
		return 502, errors.New("unable to write connection request data to server")
	}
	s.Flush()

	tunnel.EH, err = crypt.NewEncryptionHandler(ClientCR.EncType, ClientCR.CurveType)
	if err != nil {
		closeAll()
		ERROR("unable to create encryption handler: ", err)
		return 502, errors.New("Unable to secure connection")
	}

	tunnel.EH.SetHandshakeStream(s)

	err = tunnel.EH.ReceiveHandshake()
	if err != nil {
		con.Abort(errors.New(""))
		ERROR("Handshakte initialization failed", err)
		return 502, errors.New("Unable to finalize handshake")
	}

	CRR := new(ConnectRequestResponse)
	resp := make([]byte, 100000)
	n, err := s.Read(resp)
	DEBUG("(RAW)ConnectionRequestResponse:", string(resp[:n]))
	if err != nil {
		if err != io.EOF {
			closeAll()
			ERROR("Unable to receive connection response", err)
			return 500, errors.New("Did not receive connection response from server")
		}
	}

	err = json.Unmarshal(resp[:n], &CRR)
	if err != nil {
		closeAll()
		ERROR("Unable to parse connection response", err)
		return 502, errors.New("Unable to open data from server.. disconnecting..")
	}

	closeAll()

	DEBUG("ConnectionRequestResponse:", CRR)
	tunnel.CRR = CRR

	err = InitializeTunnelFromCRR(tunnel)
	if err != nil {
		return 502, err
	}

	DEBUG("Opening data tunnel:", net.JoinHostPort(ClientCR.ServerIP, CRR.DataPort))

	// ensure gateway is not incorrect
	if isGatewayATunnel(DEFAULT_GATEWAY) {
		return 502, errors.New("default gateway is a tunnel, please retry in a moment")
	}

	IP_AddRoute(ClientCR.ServerIP+"/32", "", DEFAULT_GATEWAY.To4().String(), "0")
	tunnel.Con, err = net.Dial(
		"udp4",
		net.JoinHostPort(ClientCR.ServerIP, CRR.DataPort),
	)
	if err != nil {
		DEBUG("Unable to open data tunnel: ", err)
		return 502, errors.New("unable to open data tunnel")
	}

	var createdNewInterface bool
	err, createdNewInterface = FindOrCreateInterface(tunnel)
	if err != nil {
		ERROR("Unable to initialize interface: ", err)
		return 502, err
	}

	if createdNewInterface {
		err = tunnel.Interface.Connect(tunnel)
		if err != nil {
			ERROR("unable to configure tunnel interface: ", err)
			return 502, errors.New("Unable to connect to tunnel interface")
		}
		if AddTunnelInterfaceToList(tunnel.Interface) {
			select {
			case interfaceMonitor <- tunnel.Interface:
			default:
				tunnel.Interface.Disconnect(tunnel)
				RemoveTunnelInterfaceFromList(tunnel.Interface)
				ERROR(3, "Interface monitor channel is full!")
				return 502, errors.New("Unable to place new interface on monitor channel")
			}
		}
	} else {
		oldTunnel := *tunnel.Interface.tunnel.Load()
		if oldTunnel != nil {
			// TODO .. leave unchanged if no change is detected
			if oldTunnel.Meta.IFName == tunnel.Meta.IFName {
				oldTunnel.Interface.RemoveRoutes(oldTunnel, tunnel.Meta.EnableDefaultRoute)
				err = tunnel.Interface.ApplyRoutes(tunnel)
				if err != nil {
					ERROR("unable to apply routes: ", err)
					return 502, fmt.Errorf("unable to apply routes: %s", err)
				}
			} else {
				defer func() {
					if err == nil {
						oldTunnel.Interface.Disconnect(oldTunnel)
					} else {
						tunnel.Interface.Disconnect(tunnel)
					}
				}()
			}
		}
	}

	// Create cross-pointers
	tunnel.Interface.tunnel.Store(&tunnel)

	tunnel.Connected = true
	tunnel.TunnelSTATS.PingTime = time.Now()

	AddTunnelToList(tunnel)
	_ = GenerateState()

	out := tunnel.EH.SEAL.Seal1(PingPongStatsBuffer, tunnel.Index)
	_, err = tunnel.Con.Write(out)
	if err != nil {
		return 502, errors.New("unable to send initial ping to server")
	}

	go tunnel.ReadFromServeTunnel()

	if tunnel.CRR.DHCP != nil {
		err = sendFirewallToServer(
			tunnel.ClientCR.ServerIP,
			tunnel.DHCP.Token,
			net.IP(tunnel.DHCP.IP[:]).String(),
			tunnel.Meta.AllowedHosts,
			tunnel.Meta.DisableFirewall,
		)
		if err != nil {
			ERROR("unable to update firewall: ", err)
		}
	}

	DEBUG("Session is ready - it took ", fmt.Sprintf("%.0f", math.Abs(time.Since(start).Seconds())), " seconds to connect")

	return 200, nil
}

func GetQRCode(LF *TWO_FACTOR_CONFIRM) (QR *QR_CODE, err error) {
	if LF.Email == "" {
		return nil, errors.New("email missing")
	}

	b := make([]rune, 16)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}

	TOTP := strings.ToUpper(string(b))

	authenticatorAppURL := gotp.NewDefaultTOTP(TOTP).ProvisioningUri(LF.Email, "Tunnels")

	QR = new(QR_CODE)
	QR.Value = authenticatorAppURL

	return QR, nil
}
