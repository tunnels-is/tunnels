package wgserver

import (
	"encoding/json"
	"net/netip"
)

func (t *inspectingTUN) applyControl(src netip.Addr, payload []byte) {
	entries, allowAll, ok := parseACLPayload(payload)
	if !ok {
		return
	}

	p, local := fwClassify(src)
	if !local || p == nil {
		return
	}
	p.setAllowed(entries, allowAll)
}

func parseACLPayload(payload []byte) (entries []aclEntry, allowAll bool, ok bool) {
	if len(payload) == 0 || len(payload) > aclMaxPayload {
		return nil, false, false
	}
	var msg struct {
		AllowAll bool     `json:"AllowAll"`
		Allowed  []string `json:"Allowed"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil, false, false
	}
	if len(msg.Allowed) > aclMaxAllowed {
		return nil, false, false
	}

	entries = make([]aclEntry, 0, len(msg.Allowed))
	for _, s := range msg.Allowed {
		if e, ok := parseACLEntry(s); ok {
			entries = append(entries, e)
		}
	}
	return entries, msg.AllowAll, true
}
