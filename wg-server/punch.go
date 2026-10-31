package wgserver

import (
	"net/netip"
	"sync"
	"time"

	"github.com/tunnels-is/tunnels/punch"
)

var (
	punchMu        sync.Mutex
	punchDiscoPriv [32]byte
	punchDiscoOK   bool

	punchSem = make(chan struct{}, 32)

	deliverPunchMu sync.Mutex
	deliverPunch   = deliverPunchOnBind
)

func setPunchDisco(priv [32]byte) {
	punchMu.Lock()
	punchDiscoPriv = priv
	punchDiscoOK = true
	punchMu.Unlock()
}

func enqueuePunch(from string, pkt []byte) {
	select {
	case punchSem <- struct{}{}:
		go func() {
			defer func() { <-punchSem }()
			dispatchPunch(from, pkt)
		}()
	default:
	}
}

func dispatchPunch(fromStr string, pkt []byte) {
	from, err := netip.ParseAddrPort(fromStr)
	if err != nil || !from.IsValid() {
		return
	}
	switch punch.TypeOf(pkt) {
	case punch.TypeReflexReq:
		handleReflex(from, pkt)
	case punch.TypeMTUProbe:
		if ack := punch.MTUAckFor(pkt); ack != nil {
			sendPunch(from, ack)
		}
	}
}

func handleReflex(from netip.AddrPort, pkt []byte) {
	punchMu.Lock()
	priv := punchDiscoPriv
	ok := punchDiscoOK
	punchMu.Unlock()
	if !ok {
		return
	}
	id, pub, nonce, err := punch.OpenReflexReq(priv, pkt, time.Now())
	if err != nil {
		return
	}
	resp, err := punch.BuildReflexResp(priv, pub, id, from, nonce)
	if err != nil {
		return
	}
	sendPunch(from, resp)
}

func sendPunch(to netip.AddrPort, pkt []byte) {
	deliverPunchMu.Lock()
	fn := deliverPunch
	deliverPunchMu.Unlock()
	if fn == nil {
		return
	}
	if err := fn(to, pkt); err != nil {
		WARN("punch send to ", to.String(), ": ", err)
	}
}

func deliverPunchOnBind(to netip.AddrPort, pkt []byte) error {
	if wgLazyBind == nil || !to.IsValid() || len(pkt) == 0 {
		return nil
	}
	ep, err := wgLazyBind.ParseEndpoint(to.String())
	if err != nil {
		return err
	}
	return wgLazyBind.Send([][]byte{pkt}, ep)
}
