//go:build darwin

package core

import (
	"fmt"
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
		tempBytes    = make([]byte, 500000)
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

		packet = tempBytes[4:packetLength]

		fmt.Println("Packet length:", packetLength)
		fmt.Println("Packet data:", packet)
		sendRemote = Tun.ProcessEgressPacket(&packet)
		if !sendRemote {
			fmt.Println("Packet not sent to remote")
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
		buff     = make([]byte, 500000)
		staging  = make([]byte, 500000)
		err      error
		osTunnel = tun.tunnel.Load()
		prePend  = []byte{0, 0, 0, 2}
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

		fmt.Println("read bytes:", packet)
		tun.ingressBytes.Add(int64(n))

		if len(packet) < 20 {
			fmt.Println("packet too small. ping ??")
			tun.RegisterPing(CopySlice(packet))
			continue
		}

		if !tun.ProcessIngressPacket(packet) {
			fmt.Println("missing mapping for packet")
			fmt.Println("packet: ", packet)
			debugMissingIngressMapping(packet)
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
