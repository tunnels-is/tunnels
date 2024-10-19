package main

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"time"

	"github.com/zveinn/crypt"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/net/quic"
)

var (
	Config                       = new(Server)
	InterfaceIP                  net.IP
	PortMappingResponseDurations = time.Duration(30 * time.Second)
	serverConfigPath             = "./server.json"
	publicPath                   string
	privatePath                  string
	publicSigningCert            *x509.Certificate
	publicSigningKey             *rsa.PublicKey
	controlCertificate           tls.Certificate
	controlConfig                *tls.Config
	quicConfig                   *quic.Config
	dataSocketFD                 int
	rawUDPSockFD                 int
	rawTCPSockFD                 int
	slots                        int

	TCPRWC                 io.ReadWriteCloser
	UDPRWC                 io.ReadWriteCloser
	toUserChannelMonitor   = make(chan int, 10000)
	fromUserChannelMonitor = make(chan int, 10000)
)

type Server struct {
	ID                 primitive.ObjectID `json:"ID"`
	ControlIP          string             `json:"ControlIP"`
	ControlPort        string             `json:"ControlPort"`
	UserMaxConnections int                `json:"UserMaxConnections"`
	InterfaceIP        string             `json:"InterfaceIP"`
	DataPort           string             `json:"DataPort"`
	StartPort          int                `json:"StartPort"`
	EndPort            int                `json:"EndPort"`
	AvailableMbps      int                `json:"AvailableMbps"`
	AvailableUserMbps  int                `json:"AvailableUserMbps"`
	InternetAccess     bool               `json:"InternetAccess,required"`
	LocalNetworkAccess bool               `json:"LocalNetworkAccess"`
	DNSAllowCustomOnly bool               `json:"DNSAllowCustomOnly"`
	DNS                []*ServerDNS       `json:"DNS"`
	Networks           []*ServerNetwork   `json:"Networks"`
	DNSServers         []string           `json:"DNSServers"`

	ControlCert string `json:"ControlCert"`
	ControlKey  string `json:"ControlKey"`
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
	EncType     crypt.EncType      `json:"EncType"`
	UserID      primitive.ObjectID `json:"UserID"`
	SeverID     primitive.ObjectID `json:"ServerID"`
	Serial      string             `json:"Serial"`

	Version int       `json:"Version"`
	Created time.Time `json:"Created"`
}

type ErrorResponse struct {
	Error string `json:"Error"`
}
