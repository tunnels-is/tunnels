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

	"github.com/google/uuid"
	"github.com/miekg/dns"
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
	tunnelMapRange(func(tun *TUN) bool {
		tunnel := tun.tunnel.Load()
		if tunnel != nil {
			_ = tunnel.Disconnect(tun)
		}
		return true
	})

	RestoreSaneDNSDefaults()
}

func sendFirewallToServer(serverIP string, DHCPToken string, DHCPIP string, allowedHosts []string, disableFirewall bool, serverCert string) (err error) {
	FR := new(FirewallRequest)
	FR.DHCPToken = DHCPToken
	FR.IP = DHCPIP
	FR.Hosts = allowedHosts

	FR.DisableFirewall = disableFirewall

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

	tc := &tls.Config{
		RootCAs:            CertPool,
		MinVersion:         tls.VersionTLS13,
		CurvePreferences:   []tls.CurveID{tls.X25519MLKEM768},
		InsecureSkipVerify: false,
	}
	if serverCert != "" {
		tc.RootCAs, err = LoadPrivateCert(serverCert)
		if err != nil {
			ERROR("Unable to load private cert: ", err)
			return errors.New("Unable to load private cert: " + serverCert)
		}
	}

	client := http.Client{
		Timeout: time.Duration(5000) * time.Millisecond,
		Transport: &http.Transport{
			TLSClientConfig: tc,
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

func SendRequestToController(method string, route string, data any, timeoutMS int) ([]byte, int, error) {
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

func ForwardToController(FR *FORWARD_REQUEST) (any, int) {
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

	var respJSON any
	if len(responseBytes) != 0 {
		err = json.Unmarshal(responseBytes, &respJSON)
		if err != nil {
			ERROR("Could not parse response data: ", err)
			er.Error = "Unable to open response from controller"
			return er, code
		}
	}

	return respJSON, code
}

var AZ_CHAR_CHECK = regexp.MustCompile(`^[a-zA-Z0-9]*$`)

func validateTunnelMeta(tun *TunnelMETA) (err []string) {
	ifnamemap := make(map[string]struct{})
	ifFail := AZ_CHAR_CHECK.MatchString(tun.IFName)
	if !ifFail {
		err = append(err, "tunnel names can only contain a-z A-Z 0-9, invalid name: "+tun.IFName)
	}

	tunnelMetaMapRange(func(t *TunnelMETA) bool {
		if t.Tag == tun.Tag {
			return true
		}
		ifnamemap[strings.ToLower(t.IFName)] = struct{}{}
		return true
	})

	_, ok := ifnamemap[strings.ToLower(tun.IFName)]
	if ok {
		err = append(err,
			"you cannot have two tunnels with the same interface name: "+tun.IFName,
		)
	}

	if len(tun.IFName) < 3 {
		err = append(err, fmt.Sprintf("tunnel name should not be less then 3 characters (%s)", tun.IFName))
	}

	// this is windows only
	errx := ValidateAdapterID(tun)
	if errx != nil {
		err = append(err, errx.Error())
	}

	return
}

func SetConfig(config *configV2) (err error) {
	defer RecoverAndLogToFile()

	oldConf := CONFIG.Load()

	dnsChange := oldConf.DNSServerIP == config.DNSServerIP ||
		oldConf.DNSServerPort == config.DNSServerPort

	if dnsChange {
		_ = UDPDNSServer.Shutdown()
	}

	apiChange := oldConf.APIPort != config.APIPort ||
		oldConf.APIIP != config.APIIP ||
		oldConf.APICert != config.APICert ||
		oldConf.APIKey != config.APIKey ||
		!slices.Equal(config.APICertDomains, oldConf.APICertDomains) ||
		!slices.Equal(config.APICertIPs, oldConf.APICertIPs)

	if apiChange {
		_ = API_SERVER.Shutdown(context.Background())
	}

	reloadBlockLists(false, false)
	CONFIG.Store(config)
	err = writeConfigToDisk()

	INFO("Config saved")
	DEBUG(fmt.Sprintf("%+v", *config))
	return nil
}

func BandwidthBytesToString(b int64) string {
	if b <= 999 {
		intS := strconv.FormatInt(b, 10)
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

//	func GenerateState() (err error) {
//		defer RecoverAndLogToFile()
//		DEBUG("Generating state object")
//
//		STATEOLD.ActiveConnections = make([]*TunnelMETA, 0)
//		STATEOLD.ConnectionStats = make([]TunnelSTATS, 0)
//		STATEOLD.Version = APP_VERSION
//
//		for i := range TunList {
//			if TunList[i] == nil {
//				continue
//			}
//
//			STATEOLD.ActiveConnections = append(STATEOLD.ActiveConnections, TunList[i].Meta)
//			var n2 uint64 = 0
//			if len(TunList[i].Nonce2Bytes) > 7 {
//				n2 = binary.BigEndian.Uint64(TunList[i].Nonce2Bytes)
//			}
//
//			x := TunnelSTATS{
//				Nonce1:              TunList[i].EH.SEAL.Nonce1U.Load(),
//				Nonce2:              n2,
//				StartPort:           TunList[i].StartPort,
//				EndPort:             TunList[i].EndPort,
//				IngressString:       BandwidthBytesToString(uint64(TunList[i].IngressBytes)),
//				EgressString:        BandwidthBytesToString(uint64(TunList[i].EgressBytes)),
//				IngressBytes:        TunList[i].IngressBytes,
//				EgressBytes:         TunList[i].EgressBytes,
//				StatsTag:            TunList[i].Meta.Tag,
//				DISK:                TunList[i].TunnelSTATS.DISK,
//				MEM:                 TunList[i].TunnelSTATS.MEM,
//				CPU:                 TunList[i].TunnelSTATS.CPU,
//				ServerToClientMicro: TunList[i].TunnelSTATS.ServerToClientMicro,
//				PingTime:            TunList[i].TunnelSTATS.PingTime,
//			}
//
//			if TunList[i].DHCP != nil {
//				x.DHCP = TunList[i].DHCP
//			}
//			if TunList[i].VPLNetwork != nil {
//				x.VPLNetwork = TunList[i].VPLNetwork
//			}
//
//			STATEOLD.ConnectionStats = append(STATEOLD.ConnectionStats, x)
//		}
//
//		if STATEOLD.C.DNSstats {
//
//			for i, v := range DNSBlockedList {
//				STATEOLD.DNSBlocksMap[i] = v
//			}
//			for i, v := range DNSResolvedList {
//				STATEOLD.DNSResolvesMap[i] = v
//			}
//		}
//
//		return
//	}
func InitializeTunnelFromCRR(TUN *TUN) (err error) {
	DNSGlobalBlock.Store(true)
	defer func() {
		RecoverAndLogToFile()
		DNSGlobalBlock.Store(false)
	}()
	go FullCleanDNSCache()

	meta := TUN.meta.Load()

	// This index is used to identify packet streams between server and user.
	TUN.Index = make([]byte, 2)
	binary.BigEndian.PutUint16(TUN.Index, uint16(TUN.CRReponse.Index))

	TUN.localInterfaceNetIP = net.ParseIP(meta.IPv4Address).To4()
	if TUN.localInterfaceNetIP == nil {
		return fmt.Errorf("Interface ip (%s) was malformed", meta.IPv4Address)
	}
	TUN.localInterfaceIP4bytes[0] = TUN.localInterfaceNetIP[0]
	TUN.localInterfaceIP4bytes[1] = TUN.localInterfaceNetIP[1]
	TUN.localInterfaceIP4bytes[2] = TUN.localInterfaceNetIP[2]
	TUN.localInterfaceIP4bytes[3] = TUN.localInterfaceNetIP[3]

	TUN.localDNSClient = new(dns.Client)
	TUN.localDNSClient.Dialer = new(net.Dialer)
	TUN.localDNSClient.Dialer.LocalAddr = &net.UDPAddr{
		IP: TUN.localInterfaceNetIP.To4(),
	}
	TUN.localDNSClient.Dialer.Resolver = DNSClient.Dialer.Resolver
	TUN.localDNSClient.Dialer.Timeout = time.Duration(5 * time.Second)
	TUN.localDNSClient.Timeout = time.Second * 5

	TUN.serverInterfaceNetIP = net.ParseIP(TUN.CRReponse.InterfaceIP).To4()
	if TUN.serverInterfaceNetIP == nil {
		return fmt.Errorf("Interface ip (%s) was malformed", TUN.CRReponse.InterfaceIP)
	}

	TUN.serverInterfaceIP4bytes[0] = TUN.serverInterfaceNetIP[0]
	TUN.serverInterfaceIP4bytes[1] = TUN.serverInterfaceNetIP[1]
	TUN.serverInterfaceIP4bytes[2] = TUN.serverInterfaceNetIP[2]
	TUN.serverInterfaceIP4bytes[3] = TUN.serverInterfaceNetIP[3]

	if TUN.CRReponse.DHCP != nil {
		TUN.serverVPLIP[0] = TUN.CRReponse.DHCP.IP[0]
		TUN.serverVPLIP[1] = TUN.CRReponse.DHCP.IP[1]
		TUN.serverVPLIP[2] = TUN.CRReponse.DHCP.IP[2]
		TUN.serverVPLIP[3] = TUN.CRReponse.DHCP.IP[3]

		TUN.dhcp = TUN.CRReponse.DHCP
		meta.DHCPToken = TUN.CRReponse.DHCP.Token
		_ = writeTunnelsToDisk(meta.Tag)
	}

	if TUN.CRReponse.VPLNetwork != nil {
		TUN.VPLNetwork = TUN.CRReponse.VPLNetwork
	}

	if meta.LocalhostNat {
		NN := new(ServerNetwork)
		NN.Network = "127.0.0.1/32"
		NN.Nat = TUN.serverInterfaceNetIP.String() + "/32"
		TUN.CRReponse.Networks = append(TUN.CRReponse.Networks, NN)
	}

	if len(meta.Networks) > 0 {
		TUN.CRReponse.Networks = meta.Networks
	}
	if len(meta.DNS) > 0 {
		TUN.CRReponse.DNS = meta.DNS
	}
	if len(meta.DNSServers) > 0 {
		TUN.CRReponse.DNSServers = meta.DNSServers
	}

	conf := CONFIG.Load()
	if len(TUN.CRReponse.DNSServers) < 1 {
		TUN.CRReponse.DNSServers = []string{conf.DNS1Default, conf.DNS2Default}
	}

	TUN.startPort = TUN.CRReponse.StartPort
	TUN.endPort = TUN.CRReponse.EndPort
	TUN.TCPEgress = make(map[[10]byte]*Mapping)
	TUN.UDPEgress = make(map[[10]byte]*Mapping)
	TUN.InitPortMap()

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
		meta.IPv4Address,
		TUN.CRReponse.StartPort,
		TUN.CRReponse.EndPort,
		TUN.CRReponse.InterfaceIP,
	))

	if TUN.CRReponse.VPLNetwork != nil && TUN.CRReponse.DHCP != nil {
		DEBUG(fmt.Sprintf(
			"DHCP/VPL info: Addr(%s) Network:(%s) Token(%s) ",
			TUN.CRReponse.DHCP.IP,
			TUN.CRReponse.VPLNetwork.Network,
			TUN.CRReponse.DHCP.Token,
		))
	}

	return nil
}

func PreConnectCheck() (int, error) {
	s := STATE.Load()
	if !s.adminState {
		return 400, errors.New("tunnels does not have the correct access permissions")
	}
	return 0, nil
}

var IsConnecting = atomic.Bool{}

func PublicConnect(ClientCR *ConnectionRequest) (code int, errm error) {
	if !IsConnecting.CompareAndSwap(false, true) {
		INFO("Already connecting to another connection, please wait a moment")
		return 400, errors.New("Already connecting to another connection, please wait a moment")
	}

	start := time.Now()
	defer func() {
		IsConnecting.Store(false)
		DEBUG("Session creation finished in: ", fmt.Sprintf("%.0f", math.Abs(time.Since(start).Seconds())), " seconds")
		runtime.GC()
	}()
	defer RecoverAndLogToFile()

	code, errm = PreConnectCheck()
	if errm != nil {
		return
	}

	state := STATE.Load()
	loadDefaultGateway()
	loadDefaultInterface()

	var meta *TunnelMETA
	if ClientCR.Tag == "" {
		ERROR("No tunnel tag given when connecting")
		return 400, errors.New("no tunnel tag given when connecting")
	}

	tunnelMetaMapRange(func(tun *TunnelMETA) bool {
		if tun.Tag == DefaultTunnelName && ClientCR.Tag == DefaultTunnelName {
			meta = tun
			tun.ServerID = ClientCR.ServerID
			_ = writeTunnelsToDisk(DefaultTunnelName)
			return false
		} else if tun.Tag == ClientCR.Tag {
			meta = tun
			return false
		}
		return true
	})

	if meta == nil {
		ERROR("vpn connection metadata not found for tag: ", ClientCR.Tag)
		return 400, errors.New("error fetching connection meta")
	}

	if meta.PreventIPv6 && IPv6Enabled() {
		return 400, errors.New("IPV6 enabled, please disable before connecting")
	}

	isConnected := false
	tunnelMapRange(func(tun *TUN) bool {
		m := tun.meta.Load()
		if m == nil {
			return true
		}
		if m.Tag == meta.Tag {
			if tun.GetState() >= TUN_Connected {
				isConnected = true
			}
			return false
		}

		return true
	})
	if isConnected {
		ERROR("Already connected to ", ClientCR.Tag)
		return 400, errors.New("Already connected to " + ClientCR.Tag)
	}

	tunnel := new(TUN)
	tunnel.meta.Store(meta)
	tunnel.CR = ClientCR

	if meta.DNSDiscovery != "" {
		DEBUG("looking for connection info @ ", meta.DNSDiscovery)
		dnsInfo, err := certs.ResolveMetaTXT(meta.DNSDiscovery)
		if err != nil {
			ERROR("error looking up connection info: ", err)
			return 400, err
		}

		DEBUG("DNS Info: ", dnsInfo.IP, dnsInfo.Port, dnsInfo.ServerID, "cert length: ", len(dnsInfo.Cert))
		ClientCR.ServerPort = dnsInfo.Port
		ClientCR.ServerIP = dnsInfo.IP
		ClientCR.ServerID = dnsInfo.ServerID
		tunnel.ServerCertBytes = dnsInfo.Cert
		meta.Public = false
	}

	if ClientCR.ServerIP == "" {
		ERROR("No Server IPAddress found when connecting: ", ClientCR)
		return 400, errors.New("no ip address found when connecting")
	}
	if ClientCR.ServerPort == "" {
		ERROR("No Server Port found when connecting: ", ClientCR)
		return 400, errors.New("no server port found when connecting")
	}
	if ClientCR.ServerID == "" {
		ERROR("No Server id found when connecting: ", ClientCR)
		return 400, errors.New("no server id found when connecting")
	}

	FinalCR := new(RemoteConnectionRequest)
	FinalCR.Created = time.Now()
	FinalCR.Version = apiVersion
	FinalCR.UserID = ClientCR.UserID
	FinalCR.EncType = ClientCR.EncType
	FinalCR.DHCPToken = meta.DHCPToken
	FinalCR.SeverID = ClientCR.ServerID
	FinalCR.CurveType = ClientCR.CurveType
	FinalCR.DeviceKey = ClientCR.DeviceKey
	FinalCR.DeviceToken = ClientCR.DeviceToken
	FinalCR.RequestingPorts = meta.RequestVPNPorts

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

	if !meta.Public {
		tc.RootCAs, errm = tunnel.LoadPrivateCerts(meta.ServerCert)
		if errm != nil {
			ERROR("Unable to load private cert: ", errm)
			return 502, errors.New("Unable to load private cert: " + meta.ServerCert)
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
	if !meta.Public {
		FR.Path = "v3/session/private"
	} else {
		FR.Path = "v3/session/public"
	}

	if ClientCR.DeviceKey != "" {
		FR.Path += "/min"
	}

	FR.JSONData = FinalCR
	FR.Timeout = 25000

	fmt.Println("DT:", ClientCR.DeviceToken)
	bytesFromController, code := ForwardConnectToController(FR)
	DEBUG("CodeFromController:", code)
	if code != 200 {
		x.Close(context.Background())
		con.Abort(errors.New(""))
		ERROR("ErrFromController:", string(bytesFromController))
		ER := new(ErrorResponse)
		err := json.Unmarshal(bytesFromController, ER)
		fmt.Println("format err:", err)
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

	tunnel.encWrapper, err = crypt.NewEncryptionHandler(ClientCR.EncType, ClientCR.CurveType)
	if err != nil {
		closeAll()
		ERROR("unable to create encryption handler: ", err)
		return 502, errors.New("Unable to secure connection")
	}

	tunnel.encWrapper.SetHandshakeStream(s)

	err = tunnel.encWrapper.ReceiveHandshake()
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
	tunnel.CRReponse = CRR

	err = InitializeTunnelFromCRR(tunnel)
	if err != nil {
		return 502, err
	}

	DEBUG("Opening data tunnel:", net.JoinHostPort(ClientCR.ServerIP, CRR.DataPort))

	// ensure gateway is not incorrect
	gateway := state.DefaultGateway.Load()
	if gateway != nil {
		if isInterfaceATunnel(*gateway) {
			return 502, errors.New("default gateway is a tunnel, please retry in a moment")
		}
	} else {
		return 502, errors.New("no default gateway, check your connection settings")
	}

	err = IP_AddRoute(ClientCR.ServerIP+"/32", "", gateway.To4().String(), "0")
	if err != nil {
		return 502, errors.New("unable to initialize routes")
	}

	tunnel.connection, err = net.Dial(
		"udp4",
		net.JoinHostPort(ClientCR.ServerIP, CRR.DataPort),
	)
	if err != nil {
		DEBUG("Unable to open data tunnel: ", err)
		return 502, errors.New("unable to open data tunnel")
	}

	inter, err := CreateAndConnectToInterface(tunnel)
	if err != nil {
		ERROR("Unable to initialize interface: ", err)
		return 502, err
	}

	tunnel.tunnel.Store(inter)
	inter.tunnel.Store(&tunnel)

	err = inter.Connect(tunnel)
	if err != nil {
		ERROR("unable to configure tunnel interface: ", err)
		return 502, errors.New("Unable to connect to tunnel interface")
	}

	// Create cross-pointers
	tunnel.SetState(TUN_Connected)
	tunnel.registerPing(time.Now())
	tunnel.id = uuid.NewString()
	TunnelMap.Store(tunnel.id, tunnel)

	// _ = GenerateState()

	out := tunnel.encWrapper.SEAL.Seal1(PingPongStatsBuffer, tunnel.Index)
	_, err = tunnel.connection.Write(out)
	if err != nil {
		return 502, errors.New("unable to send initial ping to server")
	}

	go tunnel.ReadFromServeTunnel()
	go inter.ReadFromTunnelInterface()

	if tunnel.CRReponse.DHCP != nil {
		err = sendFirewallToServer(
			tunnel.CR.ServerIP,
			tunnel.dhcp.Token,
			net.IP(tunnel.dhcp.IP[:]).String(),
			meta.AllowedHosts,
			meta.DisableFirewall,
			meta.ServerCert,
		)
		if err != nil {
			ERROR("unable to update firewall: ", err)
		} else {
			DEBUG("firewall update on server")
		}
	}

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
