package core

import (
	"bytes"
	"context"
	"crypto/tls"
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
	"github.com/tunnels-is/tunnels/crypt"
	"github.com/tunnels-is/tunnels/types"
	"github.com/xlzd/gotp"
	"golang.org/x/sys/unix"
)

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
		// RootCAs:            CertPool,
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

func SendRequestToURL(tc *tls.Config, method string, url string, data any, timeoutMS int, skipVerify bool) ([]byte, int, error) {
	defer RecoverAndLogToFile()

	var body []byte
	var err error
	if data != nil {
		body, err = json.Marshal(data)
		if err != nil {
			return nil, 400, err
		}
	}

	var req *http.Request
	if method == "POST" {
		req, err = http.NewRequest(method, url, bytes.NewBuffer(body))
	} else if method == "GET" {
		req, err = http.NewRequest(method, url, nil)
	} else {
		return nil, 400, errors.New("method not supported:" + method)
	}

	if err != nil {
		return nil, 400, err
	}

	req.Header.Add("Content-Type", "application/json")

	client := http.Client{Timeout: time.Duration(timeoutMS) * time.Millisecond}
	if tc != nil {
		client.Transport = &http.Transport{
			TLSClientConfig: tc,
		}
	} else {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion:         tls.VersionTLS12,
				InsecureSkipVerify: skipVerify,
			},
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		if resp != nil {
			return nil, resp.StatusCode, err
		} else {
			return nil, 400, err
		}
	}

	client.CloseIdleConnections()
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	var respBodyBytes []byte
	respBodyBytes, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return respBodyBytes, resp.StatusCode, nil
}

func ForwardToController(FR *FORWARD_REQUEST) (any, int) {
	defer RecoverAndLogToFile()

	responseBytes, code, err := SendRequestToURL(
		nil,
		FR.Method,
		FR.Path,
		FR.JSONData,
		FR.Timeout,
		false,
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
	binary.BigEndian.PutUint16(TUN.Index, uint16(TUN.ServerReponse.Index))

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

	TUN.serverInterfaceNetIP = net.ParseIP(TUN.ServerReponse.InterfaceIP).To4()
	if TUN.serverInterfaceNetIP == nil {
		return fmt.Errorf("Interface ip (%s) was malformed", TUN.ServerReponse.InterfaceIP)
	}

	TUN.serverInterfaceIP4bytes[0] = TUN.serverInterfaceNetIP[0]
	TUN.serverInterfaceIP4bytes[1] = TUN.serverInterfaceNetIP[1]
	TUN.serverInterfaceIP4bytes[2] = TUN.serverInterfaceNetIP[2]
	TUN.serverInterfaceIP4bytes[3] = TUN.serverInterfaceNetIP[3]

	if TUN.ServerReponse.DHCP != nil {
		TUN.serverVPLIP[0] = TUN.ServerReponse.DHCP.IP[0]
		TUN.serverVPLIP[1] = TUN.ServerReponse.DHCP.IP[1]
		TUN.serverVPLIP[2] = TUN.ServerReponse.DHCP.IP[2]
		TUN.serverVPLIP[3] = TUN.ServerReponse.DHCP.IP[3]

		TUN.dhcp = TUN.ServerReponse.DHCP
		meta.DHCPToken = TUN.ServerReponse.DHCP.Token
		_ = writeTunnelsToDisk(meta.Tag)
	}

	if TUN.ServerReponse.LAN != nil {
		TUN.VPLNetwork = TUN.ServerReponse.LAN
	}

	if meta.LocalhostNat {
		NN := new(types.Network)
		NN.Network = "127.0.0.1/32"
		NN.Nat = TUN.serverInterfaceNetIP.String() + "/32"
		TUN.ServerReponse.Networks = append(TUN.ServerReponse.Networks, NN)
	}

	if len(meta.Networks) > 0 {
		TUN.ServerReponse.Networks = meta.Networks
	}
	if len(meta.Routes) > 0 {
		TUN.ServerReponse.Routes = meta.Routes
	}
	if len(meta.DNSRecords) > 0 {
		TUN.ServerReponse.DNSRecords = meta.DNSRecords
	}
	if len(meta.DNSServers) > 0 {
		TUN.ServerReponse.DNSServers = meta.DNSServers
	}

	conf := CONFIG.Load()
	if len(TUN.ServerReponse.DNSServers) < 1 {
		TUN.ServerReponse.DNSServers = []string{conf.DNS1Default, conf.DNS2Default}
	}

	TUN.startPort = TUN.ServerReponse.StartPort
	TUN.endPort = TUN.ServerReponse.EndPort
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
		TUN.ServerReponse.StartPort,
		TUN.ServerReponse.EndPort,
		TUN.ServerReponse.InterfaceIP,
	))

	if TUN.ServerReponse.LAN != nil && TUN.ServerReponse.DHCP != nil {
		DEBUG(fmt.Sprintf(
			"DHCP/VPL info: Addr(%s) Network:(%s) Token(%s) ",
			TUN.ServerReponse.DHCP.IP,
			TUN.ServerReponse.LAN.Network,
			TUN.ServerReponse.DHCP.Token,
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

	FinalCR := new(types.ControllerConnectRequest)
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

	// TODO.. think about private certs ? or just make auto let's encrypt service ?
	bytesFromController, code, err := SendRequestToURL(
		nil,
		"POST",
		"https://api.tunnels.is/session",
		FinalCR,
		10000,
		false,
	)
	if code != 200 {
		ERROR("ErrFromController:", string(bytesFromController))
		ER := new(ErrorResponse)
		err := json.Unmarshal(bytesFromController, ER)
		if err == nil {
			return code, errors.New(ER.Error)
		} else {
			return code, errors.New("Error code from controller")
		}
	}
	if err != nil {
		return 500, errors.New("Unknown when contacting controller")
	}
	DEBUG("SignedPayload:", code, string(bytesFromController))

	SignedResponse := new(types.SignedConnectRequest)
	err = json.Unmarshal(bytesFromController, SignedResponse)
	if err != nil {
		ERROR("invalid signed response from controller", err)
		return 502, errors.New("invalid response from controller")
	}

	tunnel.encWrapper, err = crypt.NewEncryptionHandler(ClientCR.EncType, ClientCR.CurveType)
	if err != nil {
		ERROR("unable to create encryption handler: ", err)
		return 502, errors.New("Unable to secure connection")
	}
	SignedResponse.UserHandshake = tunnel.encWrapper.GetPublicKey()

	tc := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		CurvePreferences:   []tls.CurveID{tls.X25519MLKEM768},
		InsecureSkipVerify: false,
	}
	tc.RootCAs, errm = tunnel.LoadCertPEMBytes(SignedResponse.ServerPubKey)
	if errm != nil {
		ERROR("Unable to load cert pem from controller: ", errm)
		return 502, errors.New("Unable to load cert pem from controller")
	}
	bytesFromServer, code, err := SendRequestToURL(
		tc,
		"POST",
		"https://server.tunnels.is/connect",
		SignedResponse,
		10000,
		false,
	)
	if code != 200 {
		ERROR("ErrFromServer:", string(bytesFromServer))
		ER := new(ErrorResponse)
		err := json.Unmarshal(bytesFromServer, ER)
		if err == nil {
			return code, errors.New(ER.Error)
		} else {
			return code, errors.New("Error code from controller")
		}
	}
	if err != nil {
		return 500, errors.New("Unknown when contacting controller")
	}

	ServerReponse := new(types.ServerConnectResponse)
	err = json.Unmarshal(bytesFromServer, ServerReponse)
	if err != nil {
		return 500, errors.New("Unable to decode reponse from server")
	}

	DEBUG("ConnectionRequestResponse:", ServerReponse)
	tunnel.ServerReponse = ServerReponse

	err = InitializeTunnelFromCRR(tunnel)
	if err != nil {
		return 502, err
	}

	// ensure gateway is not incorrect
	gateway := state.DefaultGateway.Load()
	if gateway != nil {
		if isInterfaceATunnel(*gateway) {
			return 502, errors.New("default gateway is a tunnel, please retry in a moment")
		}
	} else {
		return 502, errors.New("no default gateway, check your connection settings")
	}

	ifName := state.DefaultInterfaceName.Load()
	if ifName == nil {
		return 502, errors.New("no default interface, please check try again")
	}
	err = IP_AddRoute(ServerReponse.InterfaceIP+"/32", *ifName, gateway.To4().String(), "0")
	if err != nil {
		return 502, errors.New("unable to initialize routes")
	}

	raddr, err := net.ResolveUDPAddr("udp4", ServerReponse.InterfaceIP+":"+ServerReponse.DataPort)
	if err != nil {
		return 502, errors.New("unable to resolve data port upd route")
	}

	UDPConn, err := net.DialUDP("udp4", nil, raddr)
	if err != nil {
		DEBUG("Unable to open data tunnel: ", err)
		return 502, errors.New("unable to open data tunnel")
	}
	// EXPERIMENTAL
	// err = setDontFragment(UDPConn)
	// if err != nil {
	// 	DEBUG("unable to disable IP fragmentation", err)
	// }
	tunnel.connection = net.Conn(UDPConn)

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

	tunnel.SetState(TUN_Connected)
	tunnel.registerPing(time.Now())
	tunnel.ID = uuid.NewString()
	TunnelMap.Store(tunnel.ID, tunnel)

	_, err = tunnel.connection.Write(
		tunnel.encWrapper.SEAL.Seal1(PingPongStatsBuffer, tunnel.Index),
	)
	if err != nil {
		return 502, errors.New("unable to send ping to server")
	}

	go tunnel.ReadFromServeTunnel()
	go inter.ReadFromTunnelInterface()

	if tunnel.ServerReponse.DHCP != nil {
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

func setDontFragment(conn *net.UDPConn) error {
	// Get the underlying file descriptor
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return fmt.Errorf("failed to get raw connection: %w", err)
	}

	var sockOptErr error
	err = rawConn.Control(func(fd uintptr) {
		// --------- Platform Specific ---------
		switch runtime.GOOS {
		case "linux":
			// IP_PMTUDISC_DO = 2: Always set DF flag. Never fragment locally.
			sockOptErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_MTU_DISCOVER, unix.IP_PMTUDISC_DO)
		default:
			sockOptErr = fmt.Errorf("setting DF bit not supported on GOOS=%s", runtime.GOOS)
		}
		// --------- End Platform Specific ---------
	})

	if err != nil {
		return fmt.Errorf("rawconn control error: %w", err)
	}

	if sockOptErr != nil {
		return fmt.Errorf("setsockopt error: %w", sockOptErr)
	}

	return nil
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
