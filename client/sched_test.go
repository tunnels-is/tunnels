package client

import (
	"context"
	"testing"
	"time"
)

// TestEnqueueEvent_DropsWhenFull ...
func TestEnqueueEvent_DropsWhenFull(t *testing.T) {
	ch := make(chan *event, 1)
	ch <- &event{}
	enqueueEvent(ch, func() {})
	if len(ch) != 1 {
		t.Fatalf("queue length %d", len(ch))
	}
}

// TestStartBackgroundTask_DropsWhenFull ...
func TestStartBackgroundTask_DropsWhenFull(t *testing.T) {
	n := cap(concurrencyMonitor)
	for i := 0; i < n; i++ {
		concurrencyMonitor <- &backgroundTask{tag: "fill"}
	}
	t.Cleanup(func() {
		for len(concurrencyMonitor) > 0 {
			<-concurrencyMonitor
		}
	})
	startBackgroundTask("dropped", context.Background(), func() {})
	if got := len(concurrencyMonitor); got != n {
		t.Fatalf("monitor length %d, want %d", got, n)
	}
}

// TestBackgroundTask_DropsWhenMonitorFull ...
func TestBackgroundTask_DropsWhenMonitorFull(t *testing.T) {
	mon := make(chan *backgroundTask, 1)
	mon <- &backgroundTask{tag: "fill"}
	ran := false
	task := &backgroundTask{
		monitor: mon,
		tag:     "again",
		method:  func() { ran = true },
	}
	start := time.Now()
	task.execute()
	if !ran {
		t.Fatal("task did not run")
	}
	if time.Since(start) < time.Second {
		t.Fatal("requeue did not wait")
	}
	if len(mon) != 1 {
		t.Fatalf("monitor length %d", len(mon))
	}
}
