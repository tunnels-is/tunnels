package main

import (
	"os"
	"runtime"
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
	ActiveMappins map[[6]byte]mw
	L             sync.Mutex
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

func (t *tun) make() {
	// for i := range 100000 {
	// _, _ = t.sm.LoadOrStore(i, i)
	// time.Sleep(1 * time.Millisecond)
	// m := t.availablePorts.Load()
	// p := m.p[i].Load()
	// if p == nil {
	// 	p = new(port)
	// }
	// p.l = i
	// p.r = i + 1
	// m.p[i].Store(p)
	// t.availablePorts.Store(m)
	// }
}

var s time.Time
var r atomic.Uint64

func (t *tun) get() {
	// time.Sleep(1 * time.Nanosecond)
	// for i := range 100 {
	// 	s := time.Now()
	// 	// x, ok := t.sm.Load(i)
	// 	ss := time.Since(s).Nanoseconds()
	// 	fmt.Println(ss)
	// 	if ok {
	// 		r.Add(1)
	// 		ii := x.(int)
	// 		if ii == 3000 {

	// 		}
	// 	}
	// }
	// m := t.availablePorts.Load()
	// for i := range m.p {
	// 	p := m.p[i].Load()
	// 	if p == nil {
	// 		continue
	// 	}
	// 	if p.l == 10000 {
	// 		fmt.Println(i, p.l, p.r)
	// 	}
	// }
}
func main() {

	tunnel := new(tun)
	// tunnel.sm.Store([4]byte{1, 1, 1, 1}, 2)
	runtime.GOMAXPROCS(runtime.NumCPU())
	// m := new(mappings)
	// m.p = make([]atomic.Pointer[port], 10000)
	// tunnel.availablePorts.Store(m)
	tunnel.make()
	tunnel.get()
	os.Exit(1)
	for {
		time.Sleep(1 * time.Second)
	}
}
