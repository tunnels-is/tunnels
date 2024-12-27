package core

import (
	"errors"
	"net"
	"strconv"
	"strings"

	"github.com/zveinn/crypt"
)

func EnsureOrCreateInterface(TUN *Tunnel) (err error, created bool) {
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

func RemoveTunnelInterface(T *TunnelInterface) {
	for i := range IFList {
		if IFList[i] != nil {
			if IFList[i].Name == T.Name {
				IFList[i] = nil
			}
		}
	}
}

func AddTunnelInterface(T *TunnelInterface) (assigned bool) {
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

func RemoveTunnel(GUID string) {
	for i := range ConList {
		if ConList[i] == nil {
			continue
		}
		if ConList[i].Meta.WindowsGUID == GUID {
			DEBUG("RemovingConnection:", GUID)
			ConList[i] = nil
		}
	}
}

func AddConnection(T *Tunnel) (assigned bool) {
	ConLock.Lock()
	defer ConLock.Unlock()

	for i := range ConList {
		if ConList[i] != nil {
			if ConList[i].Meta.WindowsGUID == T.Meta.WindowsGUID {
				DEBUG("RemovingConnection:", T.Meta.WindowsGUID)
				ConList[i] = nil
			}
		}
	}

	for i := range ConList {
		if ConList[i] == nil {
			DEBUG("New Connection @ index (", i, ") GUID (", T.Meta.WindowsGUID, ")")
			ConList[i] = T
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
		RemoveTunnel(GUID)
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
	T.TxQueueLen = 3000
	T.MTU = 1426
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
	M.RequestVPNPorts = true
	M.IPv4Address = "172.22.22.22"
	M.NetMask = "255.255.255.255"
	M.Tag = DefaultTunnelName
	M.IFName = DefaultTunnelName
	M.EnableDefaultRoute = true
	M.AutoConnect = true
	return
}

func FindMETAForConnectRequest(CC *ConnectionRequest) *TunnelMETA {
	for i, v := range GLOBAL_STATE.C.Connections {
		if strings.EqualFold(v.Tag, CC.Tag) {
			return GLOBAL_STATE.C.Connections[i]
		}
	}
	return nil
}

func findTunnelByGUID(GUID string) (CON *Tunnel) {
	for i := range ConList {
		if ConList[i] == nil {
			continue
		}
		if ConList[i].Meta.WindowsGUID == GUID {
			DEBUG("FoundConnection:", GUID)
			CON = ConList[i]
			return
		}
	}
	return
}

func findDefaultTunnelMeta() (M *TunnelMETA) {
	for i := range GLOBAL_STATE.C.Connections {
		if GLOBAL_STATE.C.Connections[i] == nil {
			continue
		}
		c := GLOBAL_STATE.C.Connections[i]

		if strings.ToLower(c.IFName) == DefaultTunnelName {
			return GLOBAL_STATE.C.Connections[i]
		}
	}

	return nil
}
