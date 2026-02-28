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
		} else if tun.wgDevice != nil {
			tun.wgDevice.Close()
		}
	}()

	var (
		err          error
		packetLength int
		packet       []byte
		sendRemote   bool
		tempBytes    = make([]byte, 66000)
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
		DEBUG("Server listener exiting:", meta.Tag, tun.ID)
		if tun.GetState() >= TUN_Connected {
			tunnelMonitor <- tun
		} else if tun.wgDevice != nil {
			tun.wgDevice.Close()
		}
	}()

	var (
		writeErr error
		packet   []byte
		ok       bool
		osTunnel = tun.tunnel.Load()
		prePend  = []byte{0, 0, 0, 2}
	)

	DEBUG("Server Tunnel listener initialized")
	for {
		if tun.GetState() < TUN_Connected {
			return
		}

		packet, ok = tun.wgTun.readIngress()
		if !ok {
			return
		}
		tun.ingressBytes.Add(int64(len(packet)))

		if !tun.ProcessIngressPacket(packet) {
			continue
		}

		prePend = append(prePend[:4], packet...)
		_, writeErr = osTunnel.RWC.Write(prePend[:len(packet)+4])
		if writeErr != nil {
			ERROR("tun/tap write Error: ", writeErr)
			return
		}
	}
}
