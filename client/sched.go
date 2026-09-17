package client

import (
	"context"
	"time"
)

func enqueueEvent(channel chan *event, method func()) {
	defer RecoverAndLog()
	select {
	case channel <- &event{
		method: method,
	}:
	default:
		panic("priority channel full")
	}
}

func startBackgroundTask(tag string, ctx context.Context, method func()) {
	defer RecoverAndLog()
	select {
	case concurrencyMonitor <- &backgroundTask{
		monitor: concurrencyMonitor,
		tag:     tag,
		ctx:     ctx,
		method:  method,
	}:
	default:
		panic("concurrency monitor is full")
	}
}

func (s *backgroundTask) execute() {
	defer RecoverAndLog()
	s.method()
	time.Sleep(1 * time.Second)

	select {
	case s.monitor <- s:
	default:
		panic("monitor channel is full")
	}
}
