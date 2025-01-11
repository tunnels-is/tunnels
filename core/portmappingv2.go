package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"
)

var (
	// STREAM_DEBUG    = false
	streamDebugChan = make(chan Mapping, 1000)
	noMappingChan   = make(chan []byte, 1000)
)

func StartTraceProcessor(MONITOR chan int) {
	defer func() {
		if !GLOBAL_STATE.C.ConnectionTracer {
			time.Sleep(10 * time.Second)
		}
		MONITOR <- 7
	}()

	if !GLOBAL_STATE.C.ConnectionTracer {
		return
	}

	defer RecoverAndLogToFile()
	DEBUG("Tracing module started")
	var P []byte
	var M Mapping

	if TraceFile == nil {
		InitPacketTraceFile()
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
				"M: s= %-15s:%-05d || d= %-15s:%-5d  || m= %-4d || p= %-2s || e=SFR %s.%s.%s || i=FR %s.%s || i/e= %d/%d  \n",
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
				M.ingressBytes,
				M.egressBytes,
			)
		}

	}
}

type VPNPort struct {
	M map[[4]byte]*Mapping
	L sync.Mutex
}

type Mapping struct {
	Proto        byte
	EFIN         byte
	ESYN         byte
	IFIN         byte
	ERST         byte
	IRST         byte
	LastActivity time.Time
	// IncementMS       int64
	LocalPort        [2]byte
	dstPort          [2]byte
	VPNPort          [2]byte
	OriginalSourceIP [4]byte
	DestinationIP    [4]byte
	ingressBytes     int
	egressBytes      int
}

func (V *Tunnel) InitPortMap() {
	V.TCP_M = make([]VPNPort, V.EndPort-V.StartPort)
	V.UDP_M = make([]VPNPort, V.EndPort-V.StartPort)

	for i := range V.TCP_M {
		V.TCP_M[i].M = make(map[[4]byte]*Mapping)
	}
	for i := range V.UDP_M {
		V.UDP_M[i].M = make(map[[4]byte]*Mapping)
	}
}

func (V *Tunnel) CreateNEWPortMapping(Emap map[[10]byte]*Mapping, PortMap []VPNPort, ips, port []byte) *Mapping {
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
		PortMap[i].L.Lock()
		m, ok := PortMap[i].M[Emap[EID].DestinationIP]
		if !ok || m == nil {
			PortMap[i].M[Emap[EID].DestinationIP] = Emap[EID]
			PortMap[i].L.Unlock()
			Emap[EID].Proto = V.EP_Protocol
			Emap[EID].LastActivity = time.Now()
			Emap[EID].LocalPort = [2]byte{port[0], port[1]}
			Emap[EID].dstPort[0] = port[2]
			Emap[EID].dstPort[1] = port[3]
			binary.BigEndian.PutUint16(
				Emap[EID].VPNPort[:],
				i+V.StartPort,
			)
			break
		}
		PortMap[i].L.Unlock()
	}

	if Emap[EID].LocalPort == [2]byte{0, 0} {
		Emap[EID] = nil
		return nil
	}

	return Emap[EID]
}

func (V *Tunnel) getIngressPortMapping(VPNPortMap []VPNPort, dstIP []byte, port [2]byte) *Mapping {
	p := binary.BigEndian.Uint16(port[:]) - V.StartPort
	VPNPortMap[p].L.Lock()
	mapping, ok := VPNPortMap[p].M[[4]byte(dstIP)]
	VPNPortMap[p].L.Unlock()
	if !ok || mapping == nil {
		return nil
	}
	mapping.LastActivity = time.Now()
	return mapping
}

func debugMissingEgressMapping(packet []byte) {
	if !GLOBAL_STATE.C.ConnectionTracer {
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
	if !GLOBAL_STATE.C.ConnectionTracer {
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
	if !GLOBAL_STATE.C.ConnectionTracer {
		return
	}
	select {
	case streamDebugChan <- *M:
	default:
	}
}

func (V *Tunnel) cleanPortMap() {
	for i := range V.TCP_M {
		V.TCP_M[i].L.Lock()
		for k, v := range V.TCP_M[i].M {
			switch {
			case v.EFIN > 0 && v.IFIN > 0:
				if time.Since(v.LastActivity) > time.Second*10 {
					debugMappStream(v)
					delete(V.TCP_M[i].M, k)
				}
			case v.ERST > 0 || v.IRST > 0:
				if time.Since(v.LastActivity) > time.Second*10 {
					debugMappStream(v)
					delete(V.TCP_M[i].M, k)
				}
			default:
				if time.Since(v.LastActivity) > time.Second*360 {
					debugMappStream(v)
					delete(V.TCP_M[i].M, k)
				}
			}
		}
		V.TCP_M[i].L.Unlock()
	}

	for i := range V.UDP_M {
		V.UDP_M[i].L.Lock()
		for k, v := range V.UDP_M[i].M {
			dnsL := 0
			if V.CRR != nil {
				dnsL = len(V.CRR.DNSServers)
			}

			isDNS := false
			if dnsL > 0 {
				if bytes.Equal(v.DestinationIP[:], net.ParseIP(V.CRR.DNSServers[0]).To4()) {
					isDNS = true
				}
				if dnsL > 1 && !isDNS {
					if bytes.Equal(v.DestinationIP[:], net.ParseIP(V.CRR.DNSServers[1]).To4()) {
						isDNS = true
					}
				}
				if isDNS {
					if time.Since(v.LastActivity) > time.Second*15 {
						debugMappStream(v)
						delete(V.UDP_M[i].M, k)
					}
				}
			}

			if !isDNS {
				if time.Since(v.LastActivity) > time.Second*150 {
					debugMappStream(v)
					delete(V.UDP_M[i].M, k)
				}
			}
		}
		V.UDP_M[i].L.Unlock()
	}
}

func CleanPortsForAllConnections(MONITOR chan int) {
	defer func() {
		time.Sleep(10 * time.Second)
		MONITOR <- 6
	}()
	defer RecoverAndLogToFile()
	for i := range TunList {
		if TunList[i] != nil {
			TunList[i].cleanPortMap()
		}
	}
}
