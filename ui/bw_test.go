package ui

import (
	"testing"
	"time"

	"github.com/tunnels-is/tunnels/client"
)

// bwTunnel builds a tunnel with `secs` seconds of history.
func bwTunnel(tag, serverID string, secs int, seed int64) *client.TUN {
	tn := &client.TUN{ID: "t-" + tag, CR: &client.ConnectionRequest{
		UserID: "u1", Tag: tag, ServerID: serverID}}
	bh := &client.BandwidthHistory{}
	now := time.Now()
	v := seed
	for i := secs; i > 0; i-- {
		v = (v*1103515245 + 12345) % 800000
		if v < 0 {
			v = -v
		}
		bh.Append(client.BandwidthRecord{
			Timestamp:    now.Add(-time.Duration(i) * time.Second),
			IngressBytes: v,
			EgressBytes:  v / 3,
		})
	}
	tn.BandwidthHistory.Store(bh)
	return tn
}

func TestBucketSizeFor(t *testing.T) {
	for _, c := range []struct {
		secs, want int
	}{
		{60, 1}, {300, 1}, {301, 10}, {900, 10}, {3600, 10},
		{3601, 60}, {21600, 60}, {21601, 240}, {86400, 240},
	} {
		if got := bucketSizeFor(c.secs); got != c.want {
			t.Errorf("bucketSizeFor(%d) = %d, want %d", c.secs, got, c.want)
		}
	}
}

// Stats must come from the raw samples: bucketing first would report a
// peak-of-averages and hide real spikes.
func TestSummariseUsesRawSamples(t *testing.T) {
	recs := []client.BandwidthRecord{
		{IngressBytes: 10, EgressBytes: 1},
		{IngressBytes: 1000, EgressBytes: 2}, // the spike
		{IngressBytes: 10, EgressBytes: 3},
		{IngressBytes: 20, EgressBytes: 4},
	}
	down, up := summarise(recs)
	if down.peak != 1000 {
		t.Errorf("down.peak = %d, want the raw spike 1000", down.peak)
	}
	if down.total != 1040 {
		t.Errorf("down.total = %d, want 1040", down.total)
	}
	if down.current != 20 {
		t.Errorf("down.current = %d, want the last sample 20", down.current)
	}
	if up.avg != 2 {
		t.Errorf("up.avg = %d, want 2", up.avg)
	}

	// Bucketing all four together would flatten the peak to the mean.
	b := bucket(recs, 4)
	if len(b) != 1 {
		t.Fatalf("bucket produced %d entries, want 1", len(b))
	}
	if b[0].IngressBytes == 1000 {
		t.Error("bucketing should average, not preserve the peak")
	}
	if bd, _ := summarise(b); bd.peak >= down.peak {
		t.Error("bucketed peak should be lower than the raw peak")
	}
}

// The window filter must drop samples older than the range.
func TestSampleWindowFilter(t *testing.T) {
	sid := "11111111-1111-1111-1111-111111111111"
	tn := bwTunnel("default", sid, 600, 3) // 10 minutes of history

	p := newBandwidthPanel(tn, 60)
	if got := len(p.samples()); got < 55 || got > 65 {
		t.Errorf("1m window returned %d samples, expected about 60", got)
	}
	p = newBandwidthPanel(tn, 300)
	if got := len(p.samples()); got < 290 || got > 310 {
		t.Errorf("5m window returned %d samples, expected about 300", got)
	}
	p = newBandwidthPanel(tn, 86400)
	if got := len(p.samples()); got != 600 {
		t.Errorf("24h window returned %d samples, expected all 600", got)
	}
}

// The bar count must stay drawable even for the widest window.
func TestDisplayBucketsCapped(t *testing.T) {
	for _, secs := range []int{60, 300, 900, 3600, 21600, 86400} {
		recs := make([]client.BandwidthRecord, secs)
		for i := range recs {
			recs[i] = client.BandwidthRecord{IngressBytes: int64(i), EgressBytes: int64(i / 2)}
		}
		got := len(displayBuckets(recs, secs))
		if got > maxChartBars {
			t.Errorf("range %ds produced %d bars, over the %d cap", secs, got, maxChartBars)
		}
		if got == 0 {
			t.Errorf("range %ds produced no bars", secs)
		}
	}
	// A short window must not be thinned at all.
	recs := make([]client.BandwidthRecord, 60)
	if got := len(displayBuckets(recs, 60)); got != 60 {
		t.Errorf("1m window thinned to %d bars, expected all 60", got)
	}
}
