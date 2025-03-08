package core

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/zveinn/crypt"
)

func FindOrCreateInterface(TUN *Tunnel) (err error, created bool) {
	TUN.Interface = FindTunnelInterfaceByName(TUN.Meta.IFName)
	if TUN.Interface == nil {
		TUN.Interface, err = CreateNewTunnelInterface(TUN)
		if err != nil {
			ERROR("unable to create tunnel interface: ", err)
			return errors.New("Unable to create tunnel interface"), false
		}
		created = true
	} else {
		DEBUG("Interface already exists: ", TUN.Meta.IFName)
	}

	metaIP := net.ParseIP(TUN.Meta.IPv4Address).To4()
	if metaIP == nil {
		return fmt.Errorf("invalid IP (%s) in tunnel (%s) options", TUN.Meta.IPv4Address, TUN.Meta.Tag), false
	}
	TUN.LOCAL_IF_IP[0] = metaIP[0]
	TUN.LOCAL_IF_IP[1] = metaIP[1]
	TUN.LOCAL_IF_IP[2] = metaIP[2]
	TUN.LOCAL_IF_IP[3] = metaIP[3]

	return
}

func FindTunnelInterfaceByName(name string) *TunnelInterface {
	for i := range IFList {
		if IFList[i] != nil {
			if IFList[i].Name == name {
				return IFList[i]
			}
		}
	}

	return nil
}

func RemoveTunnelInterfaceFromList(T *TunnelInterface) {
	for i := range IFList {
		if IFList[i] != nil {
			if IFList[i].Name == T.Name {
				IFList[i] = nil
			}
		}
	}
}

func AddTunnelInterfaceToList(T *TunnelInterface) (assigned bool) {
	IFLock.Lock()
	defer IFLock.Unlock()

	for i := range IFList {
		if IFList[i] == nil {
			DEBUG("New Tunnel Interface @ index (", i, ") Name (", T.Name, ")")
			IFList[i] = T
			return true
		}
	}

	return false
}

func RemoveTunnelFromList(GUID string) {
	for i := range TunList {
		if TunList[i] == nil {
			continue
		}
		if TunList[i].Meta.WindowsGUID == GUID {
			DEBUG("RemovingConnection:", GUID)
			TunList[i] = nil
		}
	}
}

func AddTunnelToList(T *Tunnel) (assigned bool) {
	ConLock.Lock()
	defer ConLock.Unlock()

	for i := range TunList {
		if TunList[i] != nil {
			if TunList[i].Meta.WindowsGUID == T.Meta.WindowsGUID {
				DEBUG("RemovingConnection:", T.Meta.WindowsGUID)
				TunList[i] = nil
			}
		}
	}

	for i := range TunList {
		if TunList[i] == nil {
			DEBUG("New Connection @ index (", i, ") GUID (", T.Meta.WindowsGUID, ")")
			TunList[i] = T
			return true
		}
	}

	return false
}

func Disconnect(GUID string, remove bool, switching bool) (err error) {
	DEBUG("Disconnect:", GUID, "RemovingConnection:", remove, "Switching:", switching)
	CON := findTunnelByGUID(GUID)
	if CON == nil {
		return
	}

	if !switching && !CON.Meta.Persistent {
		IF := FindTunnelInterfaceByName(CON.Interface.Name)
		if IF != nil {
			DEBUG("RemovingInterface:", IF.Name)
			IF.Disconnect(CON)
		}
	}

	CON.Connected = false
	CON.Con.Close()

	if remove {
		RemoveTunnelFromList(GUID)
	}

	return
}

func createRandomTunnel() (Cfg *Config, err error) {
	M := new(TunnelMETA)
	M = createTunnel()
	C.Connections = append(C.Connections, M)
	Cfg = C

	err = SaveConfig(Cfg)
	if err != nil {
		return nil, err
	}

	return
}

func createTunnel() (T *TunnelMETA) {
	T = new(TunnelMETA)
	ls := strconv.Itoa(len(C.Connections))
	ifAndTag := "newtunnel" + ls
	T.Tag = ifAndTag
	T.IFName = ifAndTag
	T.EnableDefaultRoute = false
	T.IPv4Address = "777.777.777.777"
	T.NetMask = "255.255.255.255"

	T.EncryptionType = crypt.CHACHA20
	T.DNSBlocking = false
	T.PreventIPv6 = false
	T.TxQueueLen = 1000
	T.MTU = 1420
	T.ServerID = ""
	T.AutoReconnect = false
	T.AutoConnect = false
	T.CloseConnectionsOnConnect = false
	T.Networks = make([]*ServerNetwork, 0)
	T.DNSServers = make([]string, 0)
	T.DNS = make([]*ServerDNS, 0)
	T.WindowsGUID = CreateConnectionUUID()
	return
}

func createDefaultTunnelMeta() (M *TunnelMETA) {
	M = new(TunnelMETA)
	M = createTunnel()
	M.RequestVPNPorts = true
	M.IPv4Address = "172.22.22.22"
	M.NetMask = "255.255.255.255"
	M.Tag = DefaultTunnelName
	M.IFName = DefaultTunnelName
	M.EnableDefaultRoute = true
	return
}

func createMinimalConnection() (M *TunnelMETA) {
	M = new(TunnelMETA)
	M = createTunnel()
	M.RequestVPNPorts = false
	M.IPv4Address = "172.22.22.22"
	M.NetMask = "255.255.255.255"
	M.Tag = DefaultTunnelNameMin
	M.IFName = DefaultTunnelNameMin
	if CLIHostname != "" {
		M.Hostname = CLIHostname
	}
	if CLIDNS != "" {
		M.DNSDiscovery = CLIDNS
	}
	M.Private = true
	M.EnableDefaultRoute = false
	M.AutoConnect = true
	return
}

func FindMETAForConnectRequest(CC *ConnectionRequest) *TunnelMETA {
	for i, v := range STATEOLD.C.Connections {
		if strings.EqualFold(v.Tag, CC.Tag) {
			return STATEOLD.C.Connections[i]
		}
	}
	return nil
}

func findTunnelByGUID(GUID string) (CON *Tunnel) {
	for i := range TunList {
		if TunList[i] == nil {
			continue
		}
		if TunList[i].Meta.WindowsGUID == GUID {
			DEBUG("FoundConnection:", GUID)
			CON = TunList[i]
			return
		}
	}
	return
}

func findDefaultTunnelMeta() (M *TunnelMETA) {
	for i := range STATEOLD.C.Connections {
		if STATEOLD.C.Connections[i] == nil {
			continue
		}
		c := STATEOLD.C.Connections[i]

		if strings.ToLower(c.IFName) == DefaultTunnelName {
			return STATEOLD.C.Connections[i]
		}
	}

	return nil
}
