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
		logDirect("priority channel full, dropping event")
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
		logDirect("concurrency monitor is full, dropping " + tag)
	}
}

func (s *backgroundTask) execute() {
	defer RecoverAndLog()
	s.method()
	time.Sleep(1 * time.Second)

	select {
	case s.monitor <- s:
	default:
		logDirect("monitor channel is full, dropping " + s.tag)
	}
}
