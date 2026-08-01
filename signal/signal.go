package signal

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"
)

func NewSignal(tag string, ctx context.Context, sleep time.Duration, logFunc func(string), method func()) *Signal {
	newSignal := &Signal{
		Ctx:    ctx,
		Method: method,
		Log:    logFunc,
		Tag:    tag,
		Sleep:  sleep,
	}
	go newSignal.Start()
	return newSignal
}

type Signal struct {
	Ctx    context.Context
	Method func()
	Log    func(string)
	Tag    string
	Sleep  time.Duration
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

	for s.Ctx.Err() == nil {
		s.Method()
		time.Sleep(s.Sleep)
		s.Log("goroutine restart: " + s.Tag)
	}
}
