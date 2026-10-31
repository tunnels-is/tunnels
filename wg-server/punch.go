package wgserver

import (
	"net/netip"
	"sync"
	"time"

	"github.com/tunnels-is/tunnels/punch"
)

var (
	punchMu  sync.RWMutex // write: key swap; read: held while a request is opened
	punchHub *punch.HubKey

	// punchReplay drops a reflex request whose ephemeral was already seen
	// inside its timestamp window. Fixed size (punch.ReplayCache evicts the
	// oldest entry when full).
	punchReplay = punch.NewReplayCache(0)

	punchSem = make(chan struct{}, 32)

	deliverPunchMu sync.Mutex
	deliverPunch   = deliverPunchOnBind
)

func setPunchDisco(priv [32]byte) {
	hub, err := punch.NewHubKey(priv)
	if err != nil {
		WARN("punch disco key: ", err)
		return
	}
	punchMu.Lock()
	if punchHub != nil {
		punchHub.Wipe()
	}
	punchHub = hub
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
	// Nothing on the wire says what a punch packet is: an MTU probe carries
	// a keyed marker, a reflex request a check keyed by the hub's public key.
	// Anything else is dropped without an answer.
	if ack := punch.MTUAckFor(pkt); ack != nil {
		sendPunch(from, ack)
		return
	}
	handleReflex(from, pkt)
}

func handleReflex(from netip.AddrPort, pkt []byte) {
	now := time.Now()
	punchMu.RLock()
	hub := punchHub
	if hub == nil || !hub.IsReflexReq(pkt) {
		punchMu.RUnlock()
		return
	}
	// The device UUID and disco pub inside the request are only used to
	// authenticate it; the hub keeps no per-device state and asks no one.
	req, err := hub.OpenReflexReq(pkt, now)
	punchMu.RUnlock()
	if err != nil {
		return
	}
	if !punchReplay.Fresh(req.ReplayID(), req.Expires(), now) {
		req.Wipe()
		return
	}
	resp, err := punch.BuildReflexResp(&req, from)
	req.Wipe()
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
