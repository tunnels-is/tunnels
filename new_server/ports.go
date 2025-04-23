package main

import (
	"errors"

	"github.com/tunnels-is/tunnels/types"
)

func allocatePorts(CRR *types.ServerConnectResponse, index int) (err error) {
	Config := Config.Load()
	var startPort uint16 = 0
	var endPort uint16 = 0
	for i := range portToCoreMapping {
		if i < int(Config.StartPort) {
			continue
		}

		if portToCoreMapping[i] == nil {
			// WARN("PORT TO CLIENT MAPPING IS NIL: ", i)
			continue
		}

		if portToCoreMapping[i].Client == nil {
			portToCoreMapping[i].Client = clientCoreMappings[index]
			clientCoreMappings[index].PortRange = portToCoreMapping[i]
			startPort = portToCoreMapping[i].StartPort
			endPort = portToCoreMapping[i].EndPort
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
