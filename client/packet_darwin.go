//go:build darwin

package client

import (
	"runtime/debug"
)

func (tun *TUN) ReadFromTunnelInterface() {
	defer func() {
		if r := recover(); r != nil {
			ERROR(r, string(debug.Stack()))
		}
		DEBUG("tun/tap listener exiting:")
		if tun.GetState() >= TUN_Connected {
			interfaceMonitor <- tun
		}
	}()

	var (
		err          error
		packetLength int
		packet       []byte
		writtenBytes int
		sendRemote   bool
		tempBytes    = make([]byte, 66000)
		out          []byte
		tunif        = tun.tunnel.Load()
	)

	DEBUG("New tunnel interface reader:", tunif.Name)
	for {
		if tun.GetState() < TUN_Connected {
			return
		}
		packetLength, err = tunif.RWC.Read(tempBytes[0:])
		if err != nil {
			ERROR("error in tun/tap reader loop:", err)
			return
		}

		if packetLength == 0 {
			DEEP("tun/tap read size was 0")
			continue
		}

		packet = tempBytes[4:packetLength]

		sendRemote = tun.ProcessEgressPacket(&packet)
		if !sendRemote {
			continue
		}

		out = tun.encWrapper.SEAL.Seal1(packet, tun.Index)

		writtenBytes, err = tun.connection.Write(out)
		if err != nil {
			ERROR("router write error: ", err)
			continue
		}

		tun.egressBytes.Add(int64(writtenBytes))
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
		prePend  = []byte{0, 0, 0, 2}
		meta     = tun.meta.Load()
	)

	DEBUG("Server Tunnel listener initialized")
	for {
		if tun.GetState() < TUN_Connected {
			return
		}
		// osTunnel = tun.tunnel.Load()
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
			tun.RegisterPing(meta.Tag, CopySlice(packet))
			continue
		}

		if !tun.ProcessIngressPacket(packet) {
			continue
		}

		prePend = append(prePend[:4], packet...)
		_, writeErr = osTunnel.RWC.Write(prePend[:len(packet)+4])
		//_, writeErr = osTunnel.RWC.Write(packet)
		if writeErr != nil {
			ERROR("tun/tap write Error: ", writeErr)
			return
		}
	}
}
