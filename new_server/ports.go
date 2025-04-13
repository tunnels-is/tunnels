package main

import (
	"errors"

	"github.com/tunnels-is/tunnels/types"
)

func allocatePorts(CRR *types.ConnectRequestResponse, index int) (err error) {
	Config := Config.Load()
	var startPort uint16 = 0
	var endPort uint16 = 0
	for i := range PortToCoreMapping {
		if i < int(Config.StartPort) {
			continue
		}

		if PortToCoreMapping[i] == nil {
			// WARN("PORT TO CLIENT MAPPING IS NIL: ", i)
			continue
		}

		if PortToCoreMapping[i].Client == nil {
			PortToCoreMapping[i].Client = ClientCoreMappings[index]
			ClientCoreMappings[index].PortRange = PortToCoreMapping[i]
			startPort = PortToCoreMapping[i].StartPort
			endPort = PortToCoreMapping[i].EndPort
			break
		}
	}

	if startPort == 0 {
		return errors.New("No port mappings available on the server")
	}

	CRR.StartPort = startPort
	CRR.EndPort = endPort
	return nil
}
