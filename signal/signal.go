package signal

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync/atomic"
	"time"
)

var Signals = make([]*Signal, 0)

func NewSignal(tag string, ctx context.Context, cancel context.CancelFunc, logFunc func(string), method func()) {
	Signals = append(Signals, &Signal{
		Ctx:        ctx,
		Cancel:     cancel,
		Method:     method,
		Log:        logFunc,
		Tag:        tag,
		ShouldStop: atomic.Bool{},
	})
}

type Signal struct {
	Ctx        context.Context
	Cancel     context.CancelFunc
	Method     func()
	Log        func(string)
	Tag        string
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
		if s.Log != nil {
			s.Log(fmt.Sprintf("err: %s \n %s", r, string(debug.Stack())))
		}
	}()

	for !s.ShouldStop.Load() && s.Ctx.Err() != nil {
		s.Method()
		time.Sleep(1 * time.Second)
		s.Log("goroutine restart:" + s.Tag)
	}
}
