//go:build windows

package core

import (
	"runtime/debug"
	"time"

	"golang.org/x/sys/windows"
)

func (T *TunnelInterface) ReadFromTunnelInterface() {
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
		Tun          *Tunnel
		out          = make([]byte, 70000)
	)

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
			time.Sleep(10 * time.Millisecond)
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
		if !Tun.Connected {
			time.Sleep(1 * time.Millisecond)
			continue
		}

		shouldSend := Tun.ProcessEgressPacket(&packet)
		if !shouldSend {
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
		if V.Connected && V.Interface.shouldRestart {
			V.UserRWLoopAbnormalExit = true
			tunnelMonitor <- V
		}
		select {
		case V.Interface.exitChannel <- 1:
		default:
		}
	}()

	var (
		ingressAllocationBuffer []byte
		writeError              error
		readErr                 error

		n       int
		packet  []byte
		buff    = make([]byte, 70000)
		staging = make([]byte, 70000)
		err     error
	)

	for {

		if V.Interface.shouldExit {
			return
		}

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

		if len(packet) < 20 {
			go V.RegisterPing(CopySlice(packet))
			continue
		}

		V.IngressBytes += n

		if !V.ProcessIngressPacket(packet) {
			debugMissingIngressMapping(packet)
			continue
		}

		ingressAllocationBuffer, writeError = V.Interface.AllocateSendPacket(len(packet))
		if writeError != nil {
			ERROR("ingress packet allocation error: ", writeError)
			return
		}

		if V.IP_MP != nil {
			V.IP_MP.ingressBytes += n
		}

		copy(ingressAllocationBuffer, packet)
		writeError = V.Interface.SendPacket(ingressAllocationBuffer)
		if writeError != nil {
			ERROR("adapter write error: ", writeError)
			return
		}
	}
}
