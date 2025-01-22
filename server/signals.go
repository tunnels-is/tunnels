package main

import "context"

var SignalMonitor = make(chan *SIGNAL, 10000)

type SIGNAL struct {
	ID  int
	MSG string
	Ctx context.Context
	OK  chan byte
}

func NewSignal(ctx context.Context, ID int) (S *SIGNAL) {
	S = new(SIGNAL)
	S.ID = ID
	S.Ctx = ctx
	S.OK = make(chan byte, 100)
	return
}
