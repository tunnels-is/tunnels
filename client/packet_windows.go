//go:build windows

package client

import (
	"runtime/debug"
	"time"

	"golang.org/x/sys/windows"
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

		select {
		case T.exitChannel <- 1:
		default:
		}
	}()

	var (
		waitForTimeout = time.Now()
		readError      error
		packet         []byte
		packetSize     uint16

		err          error
		writtenBytes int
		Tun          *TUN
	)

	Tun = *T.tunnel.Load()

	for {
		if T.shouldExit {
			return
		}

		_ = T.ReleaseReceivePacket(packet)
		packet, packetSize, readError = T.ReceivePacket()

		if readError == windows.ERROR_NO_MORE_ITEMS {

			if time.Since(waitForTimeout).Seconds() > 120 {
				DEBUG("ADAPTER: no packets in buffer, waiting for packets")
				waitForTimeout = time.Now()
			}
			// V.Tun.ReleaseReceivePacket(packet)
			time.Sleep(100 * time.Millisecond)
			continue

		} else if readError == windows.ERROR_HANDLE_EOF {

			ERROR("ADAPTER (eof): ", readError)
			return

		} else if readError == windows.ERROR_INVALID_DATA {

			ERROR("ADAPTER (invalid data): ", readError)
			return

		} else if readError != nil {

			ERROR("ADAPTER (unknown error): ", readError)
			return

		}

		if packetSize == 0 {
			DEBUG("Read size was 0")
			continue
		}

		Tun = *T.tunnel.Load()
		if Tun == nil {
			time.Sleep(1 * time.Millisecond)
			continue
		}

		if Tun.GetState() == TUN_Disconnected {
			time.Sleep(5 * time.Millisecond)
			continue
		}

		shouldSend := Tun.ProcessEgressPacket(&packet)
		if !shouldSend {
			continue
		}

		writtenBytes, err = Tun.connection.Write(Tun.encWrapper.SEAL.Seal1(packet, Tun.Index))
		if err != nil {
			ERROR("router write error: ", err)
			continue
		}
		Tun.egressBytes.Add(int64(writtenBytes))

	}
}

func (V *TUN) ReadFromServeTunnel() {
	defer func() {
		if r := recover(); r != nil {
			ERROR(r, string(debug.Stack()))
		}
		meta := V.meta.Load()
		inf := V.tunnel.Load()
		DEBUG("Server listener exiting:", meta.Tag)
		if (V.GetState() == TUN_Connected) && inf.shouldRestart {
			tunnelMonitor <- V
		}
		select {
		case inf.exitChannel <- 1:
		default:
		}
	}()

	var (
		writeError error
		readErr    error

		n       int
		packet  []byte
		buff    = make([]byte, 500000)
		staging = make([]byte, 500000)
		inf     = V.tunnel.Load()
		err     error
	)

	for {

		if inf.shouldExit {
			return
		}

		n, readErr = V.connection.Read(buff)
		if readErr != nil {
			ERROR("error reading from server socket: ", readErr, n)
			return
		}

		V.Nonce2Bytes = buff[2:10]
		packet, err = V.encWrapper.SEAL.Open2(
			buff[10:n],
			buff[2:10],
			staging[:0],
			buff[0:2],
		)
		if err != nil {
			ERROR("Packet authentication error: ", err)
			return
		}
		V.ingressBytes.Add(int64(n))

		if len(packet) < 20 {
			go V.RegisterPing(CopySlice(packet))
			continue
		}

		if !V.ProcessIngressPacket(packet) {
			debugMissingIngressMapping(packet)
			continue
		}

		outb, allocErr := inf.AllocateSendPacket(len(packet))
		if allocErr != nil {
			ERROR("ingress packet allocation error: ", allocErr)
			return
		}

		copy(outb, packet)
		writeError = inf.SendPacket(outb)
		if writeError != nil {
			ERROR("adapter write error: ", writeError)
			return
		}
	}
}
