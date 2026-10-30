package main

import (
	"errors"
	"net/netip"
	"sync"

	"github.com/tunnels-is/tunnels/punch"
	"golang.zx2c4.com/wireguard/tun"
)

type introTUN struct {
	tun.Device
	inject chan []byte
	intros chan tunnelIntro
	poke   chan struct{}
	onWant func(netip.Addr)

	once sync.Once
	pkts chan []byte
	errs chan error
}

func wrapIntroTUN(inner tun.Device, onWant func(netip.Addr)) (*introTUN, error) {
	return &introTUN{
		Device: inner,
		inject: make(chan []byte, 16),
		intros: make(chan tunnelIntro, 16),
		poke:   make(chan struct{}, 1),
		onWant: onWant,
	}, nil
}

func (t *introTUN) start() {
	t.once.Do(func() {
		t.pkts = make(chan []byte, 64)
		t.errs = make(chan error, 1)
		go func() {
			batch := t.Device.BatchSize()
			if batch < 1 {
				batch = 1
			}
			bufs := make([][]byte, batch)
			sizes := make([]int, batch)
			for i := range bufs {
				bufs[i] = make([]byte, 65535)
			}
			for {
				n, err := t.Device.Read(bufs, sizes, 0)
				for i := 0; i < n; i++ {
					if sizes[i] < 1 {
						continue
					}
					t.pkts <- append([]byte(nil), bufs[i][:sizes[i]]...)
				}
				if err != nil {
					if errors.Is(err, tun.ErrTooManySegments) {
						continue
					}
					select {
					case t.errs <- err:
					default:
					}
					return
				}
			}
		}()
	})
}

// Read ...
func (t *introTUN) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	t.start()
	for {
		if n := drainBytes(t.inject, bufs, sizes, offset); n > 0 {
			return n, nil
		}
		select {
		case pkt := <-t.pkts:
			if t.onWant != nil {
				if ip, ok := ipv4TCPUDPDest(pkt); ok {
					t.onWant(ip)
				}
			}
			if offset+len(pkt) > len(bufs[0]) {
				continue
			}
			copy(bufs[0][offset:], pkt)
			sizes[0] = len(pkt)
			return 1, nil
		case err := <-t.errs:
			return 0, err
		case <-t.poke:
		}
	}
}

// Write ...
func (t *introTUN) Write(bufs [][]byte, offset int) (int, error) {
	keep := make([][]byte, 0, len(bufs))
	for _, buf := range bufs {
		if len(buf) <= offset {
			continue
		}
		pkt := buf[offset:]
		if src, payload, ok := introPayload(pkt); ok {
			select {
			case t.intros <- tunnelIntro{from: src, data: payload}:
			default:
			}
			continue
		}
		keep = append(keep, buf)
	}
	if len(keep) == 0 {
		return len(bufs), nil
	}
	if _, err := t.Device.Write(keep, offset); err != nil {
		return 0, err
	}
	return len(bufs), nil
}

func (t *introTUN) queue(pkt []byte) {
	select {
	case t.inject <- pkt:
		select {
		case t.poke <- struct{}{}:
		default:
		}
	default:
	}
}

type tunnelIntro struct {
	from netip.Addr
	data []byte
}

func introPayload(pkt []byte) (netip.Addr, []byte, bool) {
	src, _, payload, ok := punch.UnwrapIPv4UDP(pkt, punch.IntroPort)
	if !ok || punch.TypeOf(payload) != punch.TypeIntro {
		return netip.Addr{}, nil, false
	}
	return src, append([]byte(nil), payload...), true
}

func ipv4TCPUDPDest(pkt []byte) (netip.Addr, bool) {
	if len(pkt) < 20 || pkt[0]>>4 != 4 {
		return netip.Addr{}, false
	}
	ihl := int(pkt[0]&0x0f) * 4
	if ihl < 20 || len(pkt) < ihl {
		return netip.Addr{}, false
	}
	if pkt[9] != 6 && pkt[9] != 17 {
		return netip.Addr{}, false
	}
	return netip.AddrFrom4([4]byte{pkt[16], pkt[17], pkt[18], pkt[19]}), true
}

func drainBytes(ch chan []byte, bufs [][]byte, sizes []int, offset int) int {
	n := 0
	for n < len(bufs) {
		var pkt []byte
		select {
		case pkt = <-ch:
		default:
			return n
		}
		if offset+len(pkt) > len(bufs[n]) {
			continue
		}
		copy(bufs[n][offset:], pkt)
		sizes[n] = len(pkt)
		n++
	}
	return n
}
