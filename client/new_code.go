package client

import (
	"errors"
	"time"

	"golang.org/x/net/context"
)

func tunnelMapRange(do func(tun *TUN) bool) {
	TunnelMap.Range(func(key, value any) bool {
		tun, ok := value.(*TUN)
		if !ok {
			return true
		}
		return do(tun)
	})
}

func tunnelMetaMapRange(do func(tun *TunnelMETA) bool) {
	TunnelMetaMap.Range(func(key, value any) bool {
		tun, ok := value.(*TunnelMETA)
		if !ok {
			return true
		}
		return do(tun)
	})
}

func doEvent(channel chan *event, method func()) {
	defer RecoverAndLogToFile()
	select {
	case channel <- &event{
		method: method,
	}:
	default:
		panic("priority channel full")
	}
}

// func doEventWithWait(channel chan *event, method func(), wait func(any), timeout time.Duration) {
// 	defer RecoverAndLogToFile()
// 	ev := new(event)
// 	ev.method = method
// 	select {
// 	case channel <- ev:
// 	default:
// 		panic("priority channel full")
// 	}
// 	ev.Wait(wait, timeout)
// }

type event struct {
	// method is executed inside priority channels
	method func()
	// done is executed on method completion
	done chan any
}

func (e *event) Wait(method func(any), timeout time.Duration) {
	defer RecoverAndLogToFile()
	tick := time.NewTimer(timeout)
	select {
	case done := <-e.done:
		method(done)
		return
	case <-tick.C:
		method(errors.New("timeout waiting"))
	}

	return
}

func newConcurrentSignal(tag string, ctx context.Context, method func()) {
	defer RecoverAndLogToFile()
	select {
	case concurrencyMonitor <- &goSignal{
		monitor: concurrencyMonitor,
		tag:     tag,
		ctx:     ctx,
		method:  method,
	}:
	default:
		panic("concurrency monitor is full")
	}
}

type goSignal struct {
	monitor chan *goSignal
	ctx     context.Context
	// cancel context.CancelFunc
	method func()
	tag    string
}

func (s *goSignal) execute() {
	defer RecoverAndLogToFile()
	s.method()
	time.Sleep(1 * time.Second)

	select {
	case s.monitor <- s:
	default:
		panic("monitor channel is full")
	}
}
