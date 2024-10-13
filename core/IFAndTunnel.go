package core

import (
	"errors"
	"strconv"
	"strings"

	"github.com/google/uuid"
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
	ls := strconv.Itoa(len(C.Connections))
	ifAndTag := "newconnection" + ls

	M := new(TunnelMETA)
	M.AutomaticRouter = true
	M.IPv4Address = "777.777.777.777"
	M.NetMask = "255.255.255.255"
	M.DNSBlocking = false
	M.PreventIPv6 = false
	M.Tag = ifAndTag
	M.IFName = ifAndTag
	M.TxQueueLen = 3000
	M.MTU = 1426
	M.AutoReconnect = false
	M.CloseConnectionsOnConnect = false
	M.WindowsGUID = CreateConnectionUUID()

	M.DNSServers = []string{}
	M.DNS = []*ServerDNS{}
	M.Networks = []*ServerNetwork{}

	C.Connections = append(C.Connections, M)
	Cfg = C

	err = SaveConfig(Cfg)
	if err != nil {
		return nil, err
	}

	return
}

func createDefaultTunnelMeta() (M *TunnelMETA) {
	M = new(TunnelMETA)
	M.AutomaticRouter = true
	M.EncryptionType = crypt.CHACHA20
	M.IPv4Address = "172.22.22.22"
	M.NetMask = "255.255.255.255"
	M.DNSBlocking = false
	M.PreventIPv6 = false
	M.Tag = DefaultTunnelName
	M.IFName = DefaultTunnelName
	M.TxQueueLen = 3000
	M.MTU = 1426
	M.ServerID = ""
	M.AutoReconnect = false
	M.AutoConnect = false
	M.CloseConnectionsOnConnect = false
	M.EnableDefaultRoute = true
	M.Networks = make([]*ServerNetwork, 0)
	M.DNSServers = make([]string, 0)
	M.DNS = make([]*ServerDNS, 0)
	M.WindowsGUID = "{" + strings.ToUpper(uuid.NewString()) + "}"

	return
}

func FindMETAForConnectRequest(CC *UIConnectRequest) *TunnelMETA {
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
