package signal

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStartContinuesAfterPanic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var mu sync.Mutex
	calls := 0
	var messages []string
	secondCall := make(chan struct{})

	s := &Signal{
		Ctx:   ctx,
		Tag:   "TEST",
		Sleep: 5 * time.Millisecond,
		Log: func(msg string) {
			mu.Lock()
			messages = append(messages, msg)
			mu.Unlock()
		},
		Method: func() {
			mu.Lock()
			calls++
			n := calls
			mu.Unlock()
			if n == 1 {
				panic("boom")
			}
			if n == 2 {
				close(secondCall)
			}
		},
	}

	done := make(chan struct{})
	go func() {
		s.Start()
		close(done)
	}()

	select {
	case <-secondCall:
	case <-time.After(2 * time.Second):
		t.Fatal("method did not run again after panic")
	}
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after cancel")
	}

	mu.Lock()
	defer mu.Unlock()
	if calls < 2 {
		t.Fatalf("calls = %d, want at least 2", calls)
	}
	logged := false
	for _, msg := range messages {
		if strings.Contains(msg, "boom") {
			logged = true
		}
	}
	if !logged {
		t.Fatalf("panic was not logged: %v", messages)
	}
}
