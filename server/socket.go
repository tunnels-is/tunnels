package main

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/tunnels-is/tunnels/types"
	"golang.org/x/crypto/sha3"
)

func hashIdentifier(identifier string) string {
	hash := sha3.Sum256([]byte(identifier))
	return hex.EncodeToString(hash[:])
}

func countConnections(id string) (count int, userCount int) {
	for i := range clientCoreMappings[:slots] {
		cm := clientCoreMappings[i].Load()
		if cm == nil || cm == sessionSentinel {
			continue
		}
		count++
		if cm.ID == id {
			userCount++
		}
	}
	return count, userCount
}

func CreateClientCoreMapping(CRR *types.ServerConnectResponse, CR *types.ControllerConnectRequest) (index int, err error) {
	defer func() {
		r := recover()
		if r != nil {
			ERR(r, string(debug.Stack()))
		}
	}()

	index = -1
	for i := range slots {
		if clientCoreMappings[i].CompareAndSwap(nil, sessionSentinel) {
			index = i
			break
		}
	}
	if index < 0 {
		return 0, errors.New("No session slots available on the server")
	}
	defer func() {
		r := recover()
		if r != nil {
			ERR(r, string(debug.Stack()))
		}
		if err != nil {
			clientCoreMappings[index].Store(nil)
		}
	}()

	cm := new(UserCoreMapping)
	cm.ID = hashIdentifier(CR.UserID.Hex())
	if CR.DeviceToken != "" {
		cm.DeviceToken = hashIdentifier(CR.DeviceToken)
	} else {
		cm.DeviceToken = hashIdentifier(CR.DeviceKey)
	}
	cm.Created = time.Now()
	cm.Uindex = make([]byte, 2)
	binary.BigEndian.PutUint16(cm.Uindex, uint16(index))

	clientCoreMappings[index].Store(cm)

	CRR.Index = index

	if LANEnabled {
		err = assignDHCP(CR, CRR, index)
		if err != nil {
			WARN("Unable to assign DHCP address")
			NukeClient(index)
			return 0, err
		}
		LOG(fmt.Sprintf("Assigned Index (%d)", index))
	}

	return index, err
}
