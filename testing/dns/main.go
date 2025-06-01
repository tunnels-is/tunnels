package main

import (
	"fmt"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

// Egress Packet
// x.x.x.x:22000 > 8.8.8.8:53
// Egress mapping:
// v.v.v.v:1000 > 8.8.8.8:53
// ++ availablePorts[1000].activeMappings[dip/dp] (mapping)
// ++ ActiveTCPMapping[sip|dip|sp|dp] (mapping)
//
// Ingress Packet:
// 8.8.8.8:53 > v.v.v.v:1000
// Ingress Mapping
// 8.8.8.8:53 > x.x.x.x:22000
// << availablePorts[1000].activeMappings[dip/dp] (mapping)

// Egress Packet
// x.x.x.x:22000 > 8.8.8.8:52
// Egress mapping:
// v.v.v.v:1000 > 8.8.8.8:52
// ++ availablePorts[1000].activeMappings[dip/dp] (mapping)
// ++ ActiveTCPMapping[sip|dip|sp|dp] (mapping)
//
// Ingress Packet:
// 8.8.8.8:52 > v.v.v.v:1000
// Ingress Mapping
// 8.8.8.8:52 > x.x.x.x:22000
// << availablePorts[1000].activeMappings[dip/dp] (mapping)

type tun struct {
	// egress
	// sip/dip/sp/dp
	// key == [12]byte
	ActiveUDPMapping sync.Map
	// ActiveUDPMapping map[[12]byte]mw

	// ingress
	// index == local port number
	availablePorts []atomic.Pointer[port]
}

type port struct {
	// dip/dp
	// key = [6]byte
	ActiveMapping sync.Map
	// ActiveMappins map[[6]byte]mw
	// L             sync.Mutex
}

type mw struct {
	am atomic.Pointer[mapping]
}

type mapping struct {
	Proto            byte
	EFIN             byte
	ESYN             byte
	IFIN             byte
	ERST             byte
	IRST             byte
	LastActivity     time.Time
	LocalPort        [2]byte
	dstPort          [2]byte
	VPNPort          [2]byte
	OriginalSourceIP [4]byte
	DestinationIP    [4]byte
}

var s time.Time
var r atomic.Uint64

type xxx struct {
	atomic.Pointer[mapping]
}

func main() {
	xx := new(xxx)
	xx.Store(&mapping{Proto: 111})

	sm := new(sync.Map)
	xx2, ok := sm.LoadOrStore([12]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}, xx)
	fmt.Println(ok)
	fmt.Println(reflect.TypeOf(xx2))
	m := xx2.(*xxx).Load()
	fmt.Println(m.Proto)

	// fmt.Println(reflect.TypeOf(xx))
	os.Exit(1)

	// sm.Store(1, xxx{})

}
