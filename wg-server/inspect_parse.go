package wgserver

import (
	"encoding/binary"
	"net/netip"
)

type fragInfo struct {
	id     uint32
	offset uint16
	more   bool
}

func (f fragInfo) isFragment() bool { return f.more || f.offset != 0 }

func (f fragInfo) isTrailing() bool { return f.offset != 0 }

func parseIPHeader(pkt []byte) (src, dst netip.Addr, proto byte, l4 []byte, frag fragInfo, ok bool) {
	if len(pkt) < 1 {
		return
	}
	switch pkt[0] >> 4 {
	case 4:
		if len(pkt) < 20 {
			return
		}
		ihl := int(pkt[0]&0x0F) * 4
		if ihl < 20 || len(pkt) < ihl {
			return
		}
		total := int(binary.BigEndian.Uint16(pkt[2:4]))
		if total < ihl || total > len(pkt) {
			return
		}
		var s, d [4]byte
		copy(s[:], pkt[12:16])
		copy(d[:], pkt[16:20])
		src = netip.AddrFrom4(s)
		dst = netip.AddrFrom4(d)
		proto = pkt[9]
		l4 = pkt[ihl:total]
		ff := binary.BigEndian.Uint16(pkt[6:8])
		frag = fragInfo{
			id:     uint32(binary.BigEndian.Uint16(pkt[4:6])),
			offset: ff & 0x1FFF,
			more:   ff&0x2000 != 0,
		}
		ok = true
		return
	case 6:
		if len(pkt) < 40 {
			return
		}
		payloadLen := int(binary.BigEndian.Uint16(pkt[4:6]))
		if 40+payloadLen > len(pkt) {
			return
		}
		var s, d [16]byte
		copy(s[:], pkt[8:24])
		copy(d[:], pkt[24:40])
		src = netip.AddrFrom16(s)
		dst = netip.AddrFrom16(d)

		next := pkt[6]
		off := 40
		end := 40 + payloadLen
	extLoop:
		for {
			switch next {
			case 0, 43, 60:
				if off+8 > end {
					return
				}
				hlen := 8 + int(pkt[off+1])*8
				if off+hlen > end {
					return
				}
				next = pkt[off]
				off += hlen
			case 44:
				if off+8 > end {
					return
				}
				fo := binary.BigEndian.Uint16(pkt[off+2 : off+4])
				frag = fragInfo{
					id:     binary.BigEndian.Uint32(pkt[off+4 : off+8]),
					offset: fo >> 3,
					more:   fo&0x1 != 0,
				}
				next = pkt[off]
				off += 8
			default:
				break extLoop
			}
		}
		proto = next
		l4 = pkt[off:end]
		ok = true
		return
	}
	return
}
