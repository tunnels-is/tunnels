package wgserver

import "encoding/binary"

func (p *fwPeer) touchFlow(k flowKey) {
	p.mu.RLock()
	if r, ok := p.flows[k]; ok {
		r.packets.Add(1)
		p.mu.RUnlock()
		return
	}
	p.mu.RUnlock()

	p.mu.Lock()
	if r, ok := p.flows[k]; ok {
		r.packets.Add(1)
	} else if len(p.flows) < flowSoftCap {
		r := &flowRec{}
		r.packets.Store(1)
		p.flows[k] = r
	}
	p.mu.Unlock()
}

func (p *fwPeer) flowMatch(k flowKey) bool {
	p.mu.RLock()
	r, ok := p.flows[k]
	if ok {
		r.packets.Add(1)
	}
	p.mu.RUnlock()
	return ok
}

func (p *fwPeer) noteFragment(k fragKey) {
	p.mu.RLock()
	if r, ok := p.frags[k]; ok {
		r.packets.Add(1)
		p.mu.RUnlock()
		return
	}
	p.mu.RUnlock()

	p.mu.Lock()
	if r, ok := p.frags[k]; ok {
		r.packets.Add(1)
	} else if len(p.frags) < flowSoftCap {
		r := &flowRec{}
		r.packets.Store(1)
		p.frags[k] = r
	}
	p.mu.Unlock()
}

func (p *fwPeer) fragmentAdmitted(k fragKey) bool {
	p.mu.RLock()
	r, ok := p.frags[k]
	if ok {
		r.packets.Add(1)
	}
	p.mu.RUnlock()
	return ok
}

func cleanFlows() {
	for i := range fwV4Slots {
		p := fwV4Slots[i].Load()
		if p == nil {
			continue
		}
		p.mu.Lock()
		for k, r := range p.flows {
			if pk := r.packets.Load(); pk == r.prev {
				delete(p.flows, k)
			} else {
				r.prev = pk
			}
		}
		for k, r := range p.frags {
			if pk := r.packets.Load(); pk == r.prev {
				delete(p.frags, k)
			} else {
				r.prev = pk
			}
		}
		p.mu.Unlock()
	}
}

func l4Ports(proto byte, l4 []byte) (sport, dport uint16) {
	switch proto {
	case 6, protoUDP:
		if len(l4) >= 4 {
			return binary.BigEndian.Uint16(l4[0:2]), binary.BigEndian.Uint16(l4[2:4])
		}
	}
	return 0, 0
}
