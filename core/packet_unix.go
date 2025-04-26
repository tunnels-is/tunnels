//go:build freebsd || linux || openbsd

package core

import (
	"runtime/debug"
	"time"
)

func (T *TInterface) ReadFromTunnelInterface() {
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
		tempBytes    = make([]byte, 66000)
		Tun          *TUN
		out          []byte
	)

	Tun = *T.tunnel.Load()

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

		out = Tun.encWrapper.SEAL.Seal1(packet, Tun.Index)

		writtenBytes, err = Tun.connection.Write(out)
		if err != nil {
			ERROR("router write error: ", err)
			continue
		}

		Tun.egressBytes.Add(int64(writtenBytes))
	}
}

func (tun *TUN) ReadFromServeTunnel() {
	defer func() {
		if r := recover(); r != nil {
			ERROR(r, string(debug.Stack()))
		}
		meta := tun.meta.Load()
		DEBUG("Server listener exiting:", meta.Tag, tun.ID)
		if tun.GetState() == TUN_Connected {
			tunnelMonitor <- tun
		}
	}()

	var (
		writeErr error
		readErr  error
		n        int
		packet   []byte
		buff     = make([]byte, 66000)
		staging  = make([]byte, 66000)
		err      error
		osTunnel = tun.tunnel.Load()
	)

	DEBUG("Server Tunnel listener initialized")
	for {
		osTunnel = tun.tunnel.Load()
		n, readErr = tun.connection.Read(buff)
		if readErr != nil {
			ERROR("error reading from server socket: ", readErr, n)
			return
		}

		tun.Nonce2Bytes = buff[2:10]
		packet, err = tun.encWrapper.SEAL.Open2(
			buff[10:n],
			buff[2:10],
			staging[:0],
			buff[0:2],
		)
		if err != nil {
			ERROR("Packet authentication error: ", err)
			return
		}

		tun.ingressBytes.Add(int64(n))

		if len(packet) < 20 {
			tun.RegisterPing(CopySlice(packet))
			continue
		}

		if !tun.ProcessIngressPacket(packet) {
			debugMissingIngressMapping(packet)
			continue
		}

		_, writeErr = osTunnel.RWC.Write(packet)
		if writeErr != nil {
			ERROR("tun/tap write Error: ", writeErr)
			return
		}
	}
}
