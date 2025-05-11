package signal

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

// var Signals = make([]*Signal, 0)
var Signals sync.Map

func NewSignal(tag string, ctx context.Context, cancel context.CancelFunc, sleep time.Duration, logFunc func(string), method func()) *Signal {
	newSignal := &Signal{
		Ctx:        ctx,
		Cancel:     cancel,
		Method:     method,
		Log:        logFunc,
		Tag:        tag,
		Sleep:      sleep,
		ShouldStop: atomic.Bool{},
	}
	Signals.Store(tag, newSignal)

	go newSignal.Start()
	return newSignal
}

type Signal struct {
	Ctx        context.Context
	Cancel     context.CancelFunc
	Method     func()
	Log        func(string)
	Tag        string
	Sleep      time.Duration
	ShouldStop atomic.Bool
}

func (s *Signal) Stop() {
	if s.Cancel != nil {
		s.Cancel()
	}
	s.ShouldStop.Store(true)
}

func (s *Signal) Start() {
	defer func() {
		r := recover()
		if r != nil {
			if s.Log != nil {
				s.Log(fmt.Sprintf("err: %s \n %s", r, string(debug.Stack())))
			}
		}
		s.Log("goroutine exit: " + s.Tag)
	}()

	for !s.ShouldStop.Load() && s.Ctx.Err() == nil {
		s.Method()
		time.Sleep(s.Sleep)
		s.Log("goroutine restart: " + s.Tag)
	}
}
