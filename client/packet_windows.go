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
		} else if tun.wgDevice != nil {
			tun.wgDevice.Close()
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

		tunif = tun.tunnel.Load()
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

		DEBUG("egress→wg: ", pktInfo(packet))
		tun.wgTun.writeEgress(packet)
		tun.egressBytes.Add(int64(len(packet)))
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
		} else if tun.wgDevice != nil {
			tun.wgDevice.Close()
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
		packet     []byte
		ok         bool
		inf        = tun.tunnel.Load()
	)

	if inf == nil {
		ERROR("ReadFromServeTunnel: tunnel interface is nil")
		return
	}

	for {
		if tun.GetState() < TUN_Connected {
			return
		}

		packet, ok = tun.wgTun.readIngress()
		if !ok {
			return
		}
		tun.ingressBytes.Add(int64(len(packet)))
		DEBUG("wg→ingress: ", pktInfo(packet))

		if !tun.ProcessIngressPacket(packet) {
			continue
		}

		DEBUG("ingress→tun: ", pktInfo(packet))
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
