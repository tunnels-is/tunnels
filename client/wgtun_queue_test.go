package client

import (
	"math"
	"strings"
	"testing"
)

func TestEgressQueueLen(t *testing.T) {
	cases := []struct {
		in   int32
		want int
	}{
		{0, egressQueueDefault},
		{-1, egressQueueDefault},
		{math.MinInt32, egressQueueDefault},
		{1, 1},
		{2000, 2000},
		{egressQueueMax, egressQueueMax},
		{egressQueueMax + 1, egressQueueMax},
		{math.MaxInt32, egressQueueMax},
	}
	for _, tc := range cases {
		if got := egressQueueLen(tc.in); got != tc.want {
			t.Fatalf("egressQueueLen(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestEgressQueueCap_UsesTunnelTXQueue(t *testing.T) {
	if got := egressQueueCap(nil); got != egressQueueDefault {
		t.Fatalf("nil tunnel %d, want %d", got, egressQueueDefault)
	}
	tun := &TUN{}
	if got := egressQueueCap(tun); got != egressQueueDefault {
		t.Fatalf("nil meta %d, want %d", got, egressQueueDefault)
	}
	tun.meta.Store(&TunnelMeta{TxQueueLen: 4096})
	if got := egressQueueCap(tun); got != 4096 {
		t.Fatalf("meta %d, want 4096", got)
	}
	tun.meta.Store(&TunnelMeta{TxQueueLen: 0})
	if got := egressQueueCap(tun); got != egressQueueDefault {
		t.Fatalf("zero meta %d, want %d", got, egressQueueDefault)
	}
}

func TestEnqueueEgress_DropsOldest(t *testing.T) {
	ch := make(chan []byte, 2)
	for _, s := range []string{"a", "b", "c", "d", "e"} {
		enqueueEgress(ch, []byte(s))
	}
	var got []string
	for {
		select {
		case pkt := <-ch:
			got = append(got, string(pkt))
		default:
			if strings.Join(got, "") != "de" {
				t.Fatalf("queue %q, want de", strings.Join(got, ""))
			}
			return
		}
	}
}

func TestEnqueueEgress_KeepsOrderWhenRoom(t *testing.T) {
	ch := make(chan []byte, 3)
	for _, s := range []string{"a", "b", "c"} {
		enqueueEgress(ch, []byte(s))
	}
	var got []string
	for {
		select {
		case pkt := <-ch:
			got = append(got, string(pkt))
		default:
			if strings.Join(got, "") != "abc" {
				t.Fatalf("queue %q, want abc", strings.Join(got, ""))
			}
			return
		}
	}
}
