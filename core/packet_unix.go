//go:build freebsd || linux || openbsd

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
		tempBytes    = make([]byte, 500000)
		Tun          *Tunnel
		out          = make([]byte, 500000)
	)

	DEBUG("New tunnel interface reader:", T.Name)
	for {

		packetLength, err = T.RWC.Read(tempBytes[0:])
		if err != nil {
			ERROR("error in tun/tap reader loop:", err)
			return
		}

		if packetLength == 0 {
			DEEP("tun/tap read size was 0")
			continue
		}

		Tun = *T.tunnel.Load()
		if Tun == nil {
			time.Sleep(1 * time.Millisecond)
			continue
		}

		packet = tempBytes[:packetLength]

		sendRemote = Tun.ProcessEgressPacket(&packet)
		if !sendRemote {
			continue
		}

		out = Tun.EH.SEAL.Seal1(packet, Tun.Index)

		// TIP; SET DONT FRAGMENT
		writtenBytes, err = Tun.Con.Write(out)
		if err != nil {
			ERROR("router write error: ", err)
			continue
		}

		if Tun.EP_MP != nil {
			Tun.EP_MP.egressBytes += writtenBytes
		}
		Tun.EgressBytes += writtenBytes
	}
}

func (V *Tunnel) ReadFromServeTunnel() {
	defer func() {
		if r := recover(); r != nil {
			ERROR(r, string(debug.Stack()))
		}
		DEBUG("Server listener exiting:", V.Meta.Tag)
		if V.Connected {
			V.UserRWLoopAbnormalExit = true
			tunnelMonitor <- V
		}
	}()

	var (
		writeErr error
		readErr  error
		n        int
		packet   []byte
		buff     = make([]byte, 500000)
		staging  = make([]byte, 500000)
		err      error
	)

	DEBUG("Server Tunnel listener initialized")
	for {
		n, readErr = V.Con.Read(buff)
		if readErr != nil {
			ERROR("error reading from server socket: ", readErr, n)
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

		_, writeErr = V.Interface.RWC.Write(packet)
		if writeErr != nil {
			ERROR("tun/tap write Error: ", writeErr)
			return
		}
	}
}
