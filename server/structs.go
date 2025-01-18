package main

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"math"
	"net"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/zveinn/crypt"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/net/quic"
)

var (
	serverConfigPath = "./server.json"
	Config           = new(Server)
	slots            int

	publicPath         string
	privatePath        string
	publicSigningCert  *x509.Certificate
	publicSigningKey   *rsa.PublicKey
	controlCertificate tls.Certificate
	controlConfig      *tls.Config
	quicConfig         *quic.Config

	dataSocketFD int
	rawUDPSockFD int
	rawTCPSockFD int
	InterfaceIP  net.IP
	TCPRWC       io.ReadWriteCloser
	UDPRWC       io.ReadWriteCloser

	toUserChannelMonitor   = make(chan int, 10000)
	fromUserChannelMonitor = make(chan int, 10000)

	PortMappingResponseDurations = time.Duration(30 * time.Second)

	ClientCoreMappings [math.MaxUint16 + 1]*UserCoreMapping
	PortToCoreMapping  [math.MaxUint16 + 1]*PortRange
	COREm              = sync.Mutex{}

	VPLNetwork      *net.IPNet
	DHCPMapping     [math.MaxUint16 + 1]*DHCPRecord
	IPToCoreMapping = make(map[[4]byte]*UserCoreMapping)
	IPm             = sync.Mutex{}

	// Routing Settings
	AllowAll bool
)

type DeviceListResponse struct {
	Devices      []*listDevice
	DHCPAssigned int
	DHCPFree     int
}

type listDevice struct {
	DHCP         *DHCPRecord
	AllowedIPs   []string
	CPU          byte
	RAM          byte
	Disk         byte
	IngressQueue int
	EgressQueue  int
	Created      time.Time
	StartPort    uint16
	EndPort      uint16
}

type DHCPRecord struct {
	m        sync.Mutex `json:"-"`
	IP       [4]byte
	Hostname string
	Token    string
	Activity time.Time `json:"-"`
}

func (d *DHCPRecord) AssignHostname(host string) {
	if host == "" {
		host = fmt.Sprintf("%d-%d-%d-%d",
			d.IP[0],
			d.IP[1],
			d.IP[2],
			d.IP[3],
		)
	}

	if defaultHostname != "" {
		d.Hostname = host + "." + defaultHostname
	} else {
		d.Hostname = host
	}
}

func (d *DHCPRecord) Assign() (ok bool) {
	if d.Token != "" {
		return
	}
	d.m.Lock()
	defer d.m.Unlock()
	if d.Token == "" {
		d.Token = uuid.NewString()
		// fmt.Println("NDHCP:", d.Token)
		d.Activity = time.Now()
		ok = true
		return
	}
	return
}

const (
	ping byte = 0
	// ok   byte = 1
	// fail byte = 2
)

type Server struct {
	ID                 primitive.ObjectID `json:"ID"`
	ControlIP          string             `json:"ControlIP"`
	ControlPort        string             `json:"ControlPort"`
	APIPort            string             `json:"APIPort"`
	UserMaxConnections int                `json:"UserMaxConnections"`
	InterfaceIP        string             `json:"InterfaceIP"`
	DataPort           string             `json:"DataPort"`
	StartPort          int                `json:"StartPort"`
	EndPort            int                `json:"EndPort"`
	AvailableMbps      int                `json:"AvailableMbps"`
	AvailableUserMbps  int                `json:"AvailableUserMbps"`
	InternetAccess     bool               `json:"InternetAccess,required"`
	LocalNetworkAccess bool               `json:"LocalNetworkAccess"`

	ControlCert string           `json:"ControlCert"`
	ControlKey  string           `json:"ControlKey"`
	APIKey      string           `json:"APIKey"`
	Networks    []*ServerNetwork `json:"Networks"`

	// Shared Settings
	DNSAllowCustomOnly bool         `json:"DNSAllowCustomOnly"`
	DNS                []*ServerDNS `json:"DNS"`
	DNSServers         []string     `json:"DNSServers"`

	// Virtual Private Lan/Layer
	VPL *VPLSettings `json:"VPL"`
}

type VPLSettings struct {
	Network    *ServerNetwork `json:"VPLNetwork"`
	MaxDevices int            `json:"MaxDevices"`
	AllowAll   bool           `json:"AllowAll"`
}

type ServerDNS struct {
	Domain   string   `json:"Domain" bson:"Domain"`
	Wildcard bool     `json:"Wildcard" bson:"Wildcard"`
	IP       []string `json:"IP" bson:"IP"`
	TXT      []string `json:"TXT" bson:"TXT"`
	CNAME    string   `json:"CNAME" bson:"CNAME"`
}

type ServerNetwork struct {
	Tag     string   `json:"Tag" bson:"Tag"`
	Network string   `json:"Network" bson:"Network"`
	Nat     string   `json:"Nat" bson:"Nat"`
	Routes  []*Route `json:"Routes" bson:"Routes"`
}

type Route struct {
	Address string
	Metric  string
}

type ConnectRequestResponse struct {
	Index             int `json:"Index"`
	AvailableMbps     int `json:"AvailableMbps"`
	AvailableUserMbps int `json:"AvailableUserMbps"`

	InternetAccess     bool `json:"InternetAccess,required"`
	LocalNetworkAccess bool `json:"LocalNetworkAccess"`

	InterfaceIP string `json:"InterfaceIP"`
	DataPort    string `json:"DataPort"`
	StartPort   uint16 `json:"StartPort"`
	EndPort     uint16 `json:"EndPort"`

	DNS                []*ServerDNS     `json:"DNS"`
	Networks           []*ServerNetwork `json:"Networks"`
	DNSServers         []string         `json:"DNSServers"`
	DNSAllowCustomOnly bool             `json:"DNSAllowCustomOnly"`

	DHCP       *DHCPRecord    `json:"DHCP"`
	VPLNetwork *ServerNetwork `json:"VPLNetwork"`
}

func CreateCRRFromServer(S *Server) (CRR *ConnectRequestResponse) {
	return &ConnectRequestResponse{
		Index:              0,
		StartPort:          0,
		EndPort:            0,
		DataPort:           S.DataPort,
		AvailableMbps:      S.AvailableMbps,
		AvailableUserMbps:  S.AvailableUserMbps,
		InternetAccess:     S.InternetAccess,
		LocalNetworkAccess: S.LocalNetworkAccess,
		InterfaceIP:        S.InterfaceIP,
		DNS:                S.DNS,
		Networks:           S.Networks,
		DNSServers:         S.DNSServers,
		DNSAllowCustomOnly: S.DNSAllowCustomOnly,
	}
}

type ConnectRequest struct {
	DeviceToken string             `json:"DeviceToken"`
	APIToken    string             `json:"APIToken"`
	EncType     crypt.EncType      `json:"EncType"`
	UserID      primitive.ObjectID `json:"UserID"`
	SeverID     primitive.ObjectID `json:"ServerID"`
	Serial      string             `json:"Serial"`

	Version int       `json:"Version"`
	Created time.Time `json:"Created"`

	// DHCP
	Hostname        string `json:"Hostname"`
	RequestingPorts bool   `json:"RequestingPorts"`
	DHCPToken       string `json:"DHCPToken"`
}

type ErrorResponse struct {
	Error string `json:"Error"`
}

type PortRange struct {
	StartPort uint16
	EndPort   uint16
	Client    *UserCoreMapping
}

type UserCoreMapping struct {
	ID                 string
	Version            int
	PortRange          *PortRange
	LastPingFromClient time.Time
	EH                 *crypt.SocketWrapper
	Uindex             []byte
	Created            time.Time

	ToUser   chan []byte
	FromUser chan Packet

	Addr syscall.Sockaddr

	// VPL
	APIToken     string
	Allowedm     sync.Mutex
	AllowedHosts []*AllowedHost
	DHCP         *DHCPRecord

	// IOT Client Only
	CPU  byte
	RAM  byte
	Disk byte
}

type AllowedHost struct {
	IP   [4]byte
	PORT [2]byte
	Type string
}

func (u *UserCoreMapping) IsHostAllowed(host [4]byte, port [2]byte) bool {
	for _, v := range u.AllowedHosts {
		if v.IP == host {
			if v.Type == "manual" {
				return true
			} else if v.PORT == port {
				return true
			}
		}
	}
	return false
}

func (u *UserCoreMapping) AddHost(host [4]byte, port [2]byte, t string) {
	found := false
	for i := range u.AllowedHosts {
		if u.AllowedHosts[i].IP == host {
			if u.AllowedHosts[i].Type == "manual" {
				found = true
			} else if u.AllowedHosts[i].PORT == port {
				found = true
			}
			break
		}
	}

	if !found {
		u.Allowedm.Lock()
		u.AllowedHosts = append(u.AllowedHosts,
			&AllowedHost{
				IP:   host,
				PORT: port,
				Type: t,
			})
		u.Allowedm.Unlock()
	}
}

func (u *UserCoreMapping) DelHost(host [4]byte, t string) {
	u.Allowedm.Lock()
	defer u.Allowedm.Unlock()
	for i := range u.AllowedHosts {
		if u.AllowedHosts[i].IP == host && u.AllowedHosts[i].Type == t {
			if len(u.AllowedHosts) < 2 {
				u.AllowedHosts = make([]*AllowedHost, 0)
				break
			} else {
				u.AllowedHosts[i] = u.AllowedHosts[len(u.AllowedHosts)-1]
				u.AllowedHosts = u.AllowedHosts[:len(u.AllowedHosts)-1]
				break
			}
		}
	}
}
