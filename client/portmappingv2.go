package client

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// STREAM_DEBUG    = false
	streamDebugChan = make(chan Mapping, 1000)
	noMappingChan   = make(chan []byte, 1000)
)

func StartTraceProcessor() {
	c := CONFIG.Load()
	defer func() {
		if !c.ConnectionTracer {
			time.Sleep(10 * time.Second)
		}
	}()

	if !c.ConnectionTracer {
		return
	}

	defer RecoverAndLogToFile()
	DEBUG("Tracing module started")
	var P []byte
	var M Mapping
	var err error

	s := STATE.Load()
	if TraceFile == nil {
		TraceFile, err = CreateFile(s.TraceFileName)
		if err != nil {
			return
		}
	}

	var ISP bool
	var ISM bool

	for {
		ISM = false
		ISP = false

		select {
		case P = <-noMappingChan:
			ISP = true
		case M = <-streamDebugChan:
			ISM = true
		}

		if ISP && P != nil {
			PP := debugProcessPacket(P)
			_, _ = fmt.Fprintf(
				TraceFile,
				"X: s= %-15s:%-05d || d= %-15s:%-5d || p= %-4s || F %08b || v= %s \n",
				net.IP(PP.SrcIP).String(),
				binary.BigEndian.Uint16(PP.TCPH[0:2]),
				net.IP(PP.DstIP).String(),
				binary.BigEndian.Uint16(PP.TCPH[2:4]),
				strconv.Itoa(int(PP.Proto)),
				PP.Flags,
				strconv.Itoa(int(PP.Version)),
			)
		}

		if ISM {
			_, _ = fmt.Fprintf(
				TraceFile,
				"M: s= %-15s:%-05d || d= %-15s:%-5d  || m= %-4d || p= %-2s || e=SFR %s.%s.%s || i=FR %s.%s \n",
				net.IP(M.OriginalSourceIP[:]).String(),
				binary.BigEndian.Uint16(M.LocalPort[:]),
				net.IP(M.DestinationIP[:]).String(),
				binary.BigEndian.Uint16(M.dstPort[:]),
				binary.BigEndian.Uint16(M.VPNPort[:]),
				strconv.Itoa(int(M.Proto)),
				strconv.Itoa(int(M.ESYN)),
				strconv.Itoa(int(M.EFIN)),
				strconv.Itoa(int(M.ERST)),
				strconv.Itoa(int(M.IFIN)),
				strconv.Itoa(int(M.IRST)),
				// M.ingressBytes,
				// M.egressBytes,
			)
		}

	}
}

type VPNPort struct {
	M map[[4]byte]*Mapping
	L sync.Mutex
}

type Mapping struct {
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

func (V *TUN) CreateNEWPortMapping(Emap map[[10]byte]*Mapping, PortMap []atomic.Pointer[VPNPort], ips, port []byte) *Mapping {
	EID := [10]byte{
		ips[0],
		ips[1],
		ips[2],
		ips[3], // SRC IP
		ips[4],
		ips[5],
		ips[6],
		ips[7], // DST IP
		port[0],
		port[1], // SRC PORT
	}

	m, ok := Emap[EID]
	if ok && m != nil {
		if V.EP_SYN > 0 {
			m.IRST = 0
			m.IFIN = 0
			m.ERST = 0
			m.EFIN = 0
		}

		m.LastActivity = time.Now()
		return m
	}

	Emap[EID] = &Mapping{}
	Emap[EID].OriginalSourceIP = [4]byte{
		ips[0],
		ips[1],
		ips[2],
		ips[3], // SRC IP
	}

	Emap[EID].DestinationIP = [4]byte{
		ips[4],
		ips[5],
		ips[6],
		ips[7], // DST IP
	}

	l := uint16(len(PortMap))
	for i := uint16(0); i < l; i++ {
		p := PortMap[i].Load()
		p.L.Lock()
		m, ok := p.M[Emap[EID].DestinationIP]
		if !ok || m == nil {
			p.M[Emap[EID].DestinationIP] = Emap[EID]
			Emap[EID].Proto = V.EP_Protocol
			Emap[EID].LastActivity = time.Now()
			Emap[EID].LocalPort = [2]byte{port[0], port[1]}
			Emap[EID].dstPort[0] = port[2]
			Emap[EID].dstPort[1] = port[3]
			binary.BigEndian.PutUint16(
				Emap[EID].VPNPort[:],
				i+V.startPort,
			)
			p.L.Unlock()
			PortMap[i].Store(p)
			break
		}
		p.L.Unlock()
	}

	if Emap[EID].LocalPort == [2]byte{0, 0} {
		Emap[EID] = nil
		return nil
	}

	return Emap[EID]
}

func (V *TUN) getIngressPortMapping(VPNPortMap []atomic.Pointer[VPNPort], dstIP []byte, port [2]byte) *Mapping {
	p := binary.BigEndian.Uint16(port[:]) - V.startPort
	vpnp := VPNPortMap[p].Load()
	vpnp.L.Lock()
	mapping, ok := vpnp.M[[4]byte(dstIP)]
	vpnp.L.Unlock()
	if !ok || mapping == nil {
		return nil
	}
	mapping.LastActivity = time.Now()
	VPNPortMap[p].Store(vpnp)
	return mapping
}

func debugMissingEgressMapping(packet []byte) {
	c := CONFIG.Load()
	if !c.ConnectionTracer {
		if len(packet) > 60 {
			DEEP("Missing egress mapping: ", packet[0:60])
		} else {
			DEEP("Missing egress mapping: ", packet[0:len(packet)-1])
		}
		return
	}

	select {
	case noMappingChan <- packet:
	default:
		DEEP("noMappingChan full")
	}
}

func debugMissingIngressMapping(packet []byte) {
	c := CONFIG.Load()
	if !c.ConnectionTracer {
		if len(packet) > 60 {
			DEEP("Missing ingress mapping: ", packet[0:60])
		} else {
			DEEP("Missing ingress mapping: ", packet[0:len(packet)-1])
		}
		return
	}

	select {
	case noMappingChan <- CopySlice(packet):
	default:
	}
}

func debugMappStream(M *Mapping) {
	c := CONFIG.Load()
	if !c.ConnectionTracer {
		return
	}
	select {
	case streamDebugChan <- *M:
	default:
	}
}

func (t *TUN) cleanPortMap() {
	for i := range t.TCPPortMap {
		tm := t.TCPPortMap[i].Load()
		tm.L.Lock()
		for k, v := range tm.M {
			switch {
			case v.EFIN > 0 && v.IFIN > 0:
				if time.Since(v.LastActivity) > time.Second*10 {
					debugMappStream(v)
					delete(tm.M, k)
				}
			case v.ERST > 0 || v.IRST > 0:
				if time.Since(v.LastActivity) > time.Second*10 {
					debugMappStream(v)
					delete(tm.M, k)
				}
			default:
				if time.Since(v.LastActivity) > time.Second*360 {
					debugMappStream(v)
					delete(tm.M, k)
				}
			}
		}
		tm.L.Unlock()
		t.TCPPortMap[i].Store(tm)
	}

	for i := range t.UDPPortMap {
		um := t.UDPPortMap[i].Load()
		um.L.Lock()
		for k, v := range um.M {
			dnsL := 0
			if t.ServerReponse != nil {
				dnsL = len(t.ServerReponse.DNSServers)
			}

			isDNS := false
			if dnsL > 0 {
				if bytes.Equal(v.DestinationIP[:], net.ParseIP(t.ServerReponse.DNSServers[0]).To4()) {
					isDNS = true
				}
				if dnsL > 1 && !isDNS {
					if bytes.Equal(v.DestinationIP[:], net.ParseIP(t.ServerReponse.DNSServers[1]).To4()) {
						isDNS = true
					}
				}
				if isDNS {
					if time.Since(v.LastActivity) > time.Second*15 {
						debugMappStream(v)
						delete(um.M, k)
					}
				}
			}

			if !isDNS {
				if time.Since(v.LastActivity) > time.Second*150 {
					debugMappStream(v)
					delete(um.M, k)
				}
			}
		}
		um.L.Unlock()
		t.UDPPortMap[i].Store(um)
	}
}

func CleanPortsForAllConnections() {
	defer func() {
		time.Sleep(10 * time.Second)
	}()
	defer RecoverAndLogToFile()
	tunnelMapRange(func(tun *TUN) bool {
		tun.cleanPortMap()
		return true
	})
}
