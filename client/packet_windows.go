//go:build windows

package client

import (
	"runtime/debug"
	"time"

	"golang.org/x/sys/windows"
)

func (tun *TUN) ReadFromTunnelInterface() {
	defer func() {
		if r := recover(); r != nil {
			ERROR(r, string(debug.Stack()))
		}
		DEBUG("tun/tap listener exiting:")
		if tun.GetState() >= TUN_Connected {
			interfaceMonitor <- tun
		} else {
			_ = tun.connection.Close()
		}

		tif := tun.tunnel.Load()
		if tif != nil {
			select {
			case tif.exitChannel <- 1:
			default:
			}
		}
	}()

	var (
		waitForTimeout = time.Now()
		readError      error
		packet         []byte
		packetSize     uint16

		err          error
		writtenBytes int
		tunif        = tun.tunnel.Load()
	)

	if tunif == nil {
		ERROR("ReadFromTunnelInterface: tunnel interface is nil")
		return
	}

	for {
		if tun.GetState() < TUN_Connected {
			return
		}

		_ = tunif.ReleaseReceivePacket(packet)
		packet, packetSize, readError = tunif.ReceivePacket()

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

		shouldSend := tun.ProcessEgressPacket(&packet)
		if !shouldSend {
			continue
		}

		writtenBytes, err = tun.connection.Write(tun.encWrapper.SEAL.Seal1(packet, tun.Index))
		if err != nil {
			ERROR("router write error: ", err)
			return
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
		DEBUG("Server listener exiting:", meta.Tag)
		if tun.GetState() >= TUN_Connected {
			tunnelMonitor <- tun
		} else {
			_ = tun.connection.Close()
		}

		inf := tun.tunnel.Load()
		if inf != nil {
			select {
			case inf.exitChannel <- 1:
			default:
			}
		}
	}()

	var (
		writeError error
		readErr    error

		n       int
		packet  []byte
		buff    = make([]byte, 66000)
		staging = make([]byte, 66000)
		inf     = tun.tunnel.Load()
		err     error
		meta    = tun.meta.Load()
	)

	if inf == nil {
		ERROR("ReadFromServeTunnel: tunnel interface is nil")
		return
	}

	for {
		if tun.GetState() < TUN_Connected {
			return
		}

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
			go tun.RegisterPing(meta.Tag, CopySlice(packet))
			continue
		}

		if !tun.ProcessIngressPacket(packet) {
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
