package client

import (
	"encoding/binary"
	"net"
	"slices"
	"sync"
	"time"
)

var (
	// STREAM_DEBUG    = false
	streamDebugChan = make(chan Mapping, 1000)
	noMappingChan   = make(chan []byte, 1000)
)

func (V *TUN) CreateNEWPortMapping(p *[]byte) (m *Mapping) {
	packet := *p
	EID := [12]byte{
		packet[12],
		packet[13],
		packet[14],
		packet[15], // src IP
		packet[16],
		packet[17],
		packet[18],
		packet[19], // dst IP
		V.EP_TPHeader[0],
		V.EP_TPHeader[1], // SRC PORT
		V.EP_TPHeader[2],
		V.EP_TPHeader[3], // dst PORT
	}

	var smap *sync.Map
	var aports []sync.Map
	if V.EP_Protocol == 6 {
		smap = V.ActiveTCPMapping
		aports = V.AvailableTCPPorts
	} else {
		smap = V.ActiveUDPMapping
		aports = V.AvailableUDPPorts
	}

	mm, ok := smap.Load(EID)
	if ok && mm != nil {
		m = mm.(*Mapping)
		// TODO...
		m.UnixTime.Store(time.Now().UnixMicro())
		if V.EP_TPHeader[13]&0x2 > 0 {
			m.rstFound.Store(false)
			m.finCount.Store(0)
		}
		return m
	}

	for i := range aports {
		_, ok = aports[i].Load([6]byte{EID[4], EID[5], EID[6], EID[7], EID[10], EID[11]})
		if !ok {
			m = &Mapping{
				Proto:   V.EP_Protocol,
				SrcPort: [2]byte{V.EP_TPHeader[0], V.EP_TPHeader[1]},
				DstPort: [2]byte{V.EP_TPHeader[2], V.EP_TPHeader[3]},
				OriginalSourceIP: [4]byte{
					EID[0],
					EID[1],
					EID[2],
					EID[3],
				},
				DestinationIP: [4]byte{
					EID[4],
					EID[5],
					EID[6],
					EID[7],
				},
			}
			m.UnixTime.Store(time.Now().UnixMicro())
			binary.BigEndian.PutUint16(
				m.MappedPort[:],
				uint16(i)+V.startPort,
			)

			smap.Store(EID, m)
			aports[i].Store(
				[6]byte{EID[4], EID[5], EID[6], EID[7], EID[10], EID[11]},
				m,
			)
			return m
		}
	}

	return
}

// func (V *TUN) getIngressPortMapping(VPNPortMap []atomic.Pointer[VPNPort], dstIP []byte, port [2]byte) *Mapping {
func (V *TUN) getIngressPortMapping() (m *Mapping) {

	var imap []sync.Map
	if V.IP_Protocol == 6 {
		imap = V.AvailableTCPPorts
	} else {
		imap = V.AvailableUDPPorts
	}
	mm, ok := imap[binary.BigEndian.Uint16(V.IP_DstPort[:])-V.startPort].Load(
		[6]byte{V.IP_SrcIP[0], V.IP_SrcIP[1], V.IP_SrcIP[2], V.IP_SrcIP[3], V.IP_SrcPort[0], V.IP_SrcPort[1]},
	)
	if ok && mm != nil {
		m = mm.(*Mapping)
		m.UnixTime.Store(time.Now().UnixMicro())
		return
	}
	return nil
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

}

func (t *TUN) cleanPortMap() {
	for i := range t.AvailableTCPPorts {
		t.AvailableTCPPorts[i].Range(func(key, value any) bool {
			m, ok := value.(*Mapping)
			if ok {
				ut := time.UnixMicro(m.UnixTime.Load())
				if m.rstFound.Load() || m.finCount.Load() > 1 {
					if time.Since(ut) > time.Second*10 {
						t.AvailableTCPPorts[i].Delete(key)
					}
				} else if time.Since(ut) > time.Second*360 {
					t.AvailableTCPPorts[i].Delete(key)
				}
			}
			return true
		})
	}

	config := CONFIG.Load()
	dnsServer := [][4]byte{}
	dnsIP1 := net.ParseIP(config.DNS1Default).To4()
	if len(dnsIP1) == 4 {
		dnsServer = append(dnsServer, [4]byte{dnsIP1[0], dnsIP1[1], dnsIP1[2], dnsIP1[3]})
	}
	dnsIP2 := net.ParseIP(config.DNS2Default).To4()
	if len(dnsIP2) == 4 {
		dnsServer = append(dnsServer, [4]byte{dnsIP2[0], dnsIP2[1], dnsIP2[2], dnsIP2[3]})
	}

	if t.ServerResponse != nil {
		for _, v := range t.ServerResponse.DNSServers {
			dns := net.ParseIP(v).To4()
			if len(dns) == 4 {
				dnsServer = append(dnsServer, [4]byte{dns[0], dns[1], dns[2], dns[3]})
			}
		}
	}

	meta := t.meta.Load()
	for _, v := range meta.DNSServers {
		dns := net.ParseIP(v).To4()
		if len(dns) == 4 {
			dnsServer = append(dnsServer, [4]byte{dns[0], dns[1], dns[2], dns[3]})
		}
	}

	for i := range t.AvailableUDPPorts {
		t.AvailableUDPPorts[i].Range(func(key, value any) bool {
			m, ok := value.(*Mapping)
			if ok {
				ut := time.UnixMicro(m.UnixTime.Load())
				if slices.Contains(dnsServer, m.DestinationIP) {
					if time.Since(ut) > time.Second*15 {
						t.AvailableUDPPorts[i].Delete(key)
					}
				} else {
					if time.Since(ut) > time.Second*150 {
						t.AvailableUDPPorts[i].Delete(key)
					}
				}
			}
			return true
		})
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
