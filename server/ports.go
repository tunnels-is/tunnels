package main

import (
	"github.com/tunnels-is/tunnels/types"
)

func allocatePorts(CRR *types.ServerConnectResponse, index int) {
	portStart := startPort + uint16(index)*uint16(portPerUser)
	portEnd := portStart + uint16(portPerUser)

	cm := clientCoreMappings[index].Load()
	cm.PortStart = portStart
	cm.PortEnd = portEnd

	CRR.StartPort = portStart
	CRR.EndPort = portEnd
}
