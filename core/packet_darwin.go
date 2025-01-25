//go:build darwin

package core

import (
	"runtime/debug"
	"time"
)

func stripSuffix(domain string) string {
	// if strings.Contains(domain, ".lan") {
	// 	domain = strings.TrimSuffix(domain, ".lan.")
	// 	domain += "."
	// }
	return domain
}

func (T *TunnelInterface) ReadFromTunnelInterface() {
	defer func() {
		if r := recover(); r != nil {
			ERROR(r, string(debug.Stack()))
		}
		DEBUG("tun/tap listener exiting:", T.Name)
		if T.shouldRestart {
			interfaceMonitor <- T
		}
	}()

	var (
		err          error
		packetLength int
		packet       []byte
		writtenBytes int
		sendRemote   bool
		tempBytes    = make([]byte, 70000)
		Tun          *Tunnel
		out          = make([]byte, 70000)
	)

	for {

		packetLength, err = T.RWC.Read(tempBytes[0:])
		if err != nil {
			ERROR("error in interface reader loop:", err)
			return
		}

		if packetLength == 0 {
			DEBUG("tun/tap read size was 0")
			continue
		}

		Tun = *T.tunnel.Load()
		if Tun == nil {
			time.Sleep(1 * time.Millisecond)
			continue
		}

		packet = tempBytes[4:packetLength]

		sendRemote = Tun.ProcessEgressPacket(&packet)
		if !sendRemote {
			continue
		}

		out = Tun.EH.SEAL.Seal1(packet, Tun.Index)

		writtenBytes, err = Tun.Con.Write(out)
		if err != nil {
			ERROR("socket write errir: ", err)
			return
		}
		if Tun.EP_MP != nil {
			Tun.EP_MP.egressBytes += writtenBytes
		}
		Tun.EgressBytes += writtenBytes
	}
}

func (V *Tunnel) ReadFromServeTunnel() {
	defer func() {
		RecoverAndLogToFile()
		DEBUG("tun tap listener exiting:", V.Meta.Tag)
	}()

	var (
		writeErr      error
		readErr       error
		receivedBytes int
		packet        []byte
		prePend       = []byte{0, 0, 0, 2}

		buff    = make([]byte, 70000)
		staging = make([]byte, 70000)
		err     error
		n       int
	)

	for {
		n, readErr = V.Con.Read(buff)
		if readErr != nil {
			ERROR("error reading from node socket", readErr, receivedBytes)
			return
		}

		V.Nonce2Bytes = buff[2:10]
		packet, err = V.EH.SEAL.Open2(
			buff[10:n],
			buff[2:10],
			staging[:0],
			buff[0:2],
		)
		if err != nil {
			ERROR("Packet authentication error: ", err)
			return
		}

		V.IngressBytes += n

		if len(packet) < 20 {
			V.RegisterPing(CopySlice(packet))
			continue
		}

		if !V.ProcessIngressPacket(packet) {
			debugMissingIngressMapping(packet)
			continue
		}
		if V.IP_MP != nil {
			V.IP_MP.ingressBytes += n
		}

		prePend = append(prePend[:4], packet...)
		_, writeErr = V.Interface.RWC.Write(prePend[:len(packet)+4])
		if writeErr != nil {
			ERROR("tun/tap write Error: ", writeErr)
			return
		}

	}
}
