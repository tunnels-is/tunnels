package client

import (
	"testing"
	"time"
)

func TestBandwidthHistoryAppendDoesNotGrowCap(t *testing.T) {
	bh := &BandwidthHistory{}
	now := time.Now()
	for i := 0; i < MaxBandwidthRecords+50; i++ {
		bh.Append(BandwidthRecord{Timestamp: now.Add(time.Duration(i) * time.Second), IngressBytes: int64(i)})
	}
	if len(bh.records) != MaxBandwidthRecords {
		t.Fatalf("len=%d, want %d", len(bh.records), MaxBandwidthRecords)
	}
	if cap(bh.records) != MaxBandwidthRecords {
		t.Fatalf("cap=%d, want %d (ring must not grow)", cap(bh.records), MaxBandwidthRecords)
	}
	held := cap(bh.records)
	for i := 0; i < 25; i++ {
		bh.Append(BandwidthRecord{Timestamp: now.Add(time.Duration(MaxBandwidthRecords+50+i) * time.Second), IngressBytes: int64(MaxBandwidthRecords + 50 + i)})
	}
	if cap(bh.records) != held {
		t.Fatalf("cap grew from %d to %d after further appends", held, cap(bh.records))
	}
	if bh.records[0].IngressBytes != 75 {
		t.Fatalf("oldest sample = %d, want 75", bh.records[0].IngressBytes)
	}
}

func TestBandwidthHistorySnapshotSinceCopiesWindow(t *testing.T) {
	bh := &BandwidthHistory{}
	now := time.Now()
	for i := 0; i < 100; i++ {
		bh.Append(BandwidthRecord{Timestamp: now.Add(time.Duration(i-100) * time.Second)})
	}
	cutoff := now.Add(-30 * time.Second)
	got := bh.SnapshotSince(cutoff)
	if len(got) < 28 || len(got) > 31 {
		t.Fatalf("window len=%d, want ~30", len(got))
	}
	all := bh.Snapshot()
	if len(all) != 100 {
		t.Fatalf("full snapshot len=%d, want 100", len(all))
	}
	got[0].IngressBytes = 999
	if bh.records[len(bh.records)-len(got)].IngressBytes == 999 {
		t.Fatal("SnapshotSince must not alias the live buffer")
	}
}
