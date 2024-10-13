package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"runtime/debug"
	"time"
)

func GetServerConfig(path string) (Server *Server, err error) {
	var nb []byte
	nb, err = os.ReadFile(path)
	if err != nil {
		return
	}
	err = json.Unmarshal(nb, &Server)
	return
}

var RAND_SOURCE = rand.NewSource(time.Now().UnixNano())

func BasicRecover() {
	if r := recover(); r != nil {
		fmt.Println("PANIC", r, true, nil)
	}
}

func CopySlice(in []byte) (out []byte) {
	out = make([]byte, len(in))
	_ = copy(out, in)
	return
}

// TODO .. make sure it's catching the context of the initiator
func RecoverAndReturnID(SIGNAL *SIGNAL, sleepTimeInSeconds int) {
	if r := recover(); r != nil {
		fmt.Println(r, string(debug.Stack()))
	}

	if SIGNAL.Ctx.Err() != nil {
		fmt.Println("Signal context err:", SIGNAL.ID, SIGNAL.Ctx.Err())
		return
	}

	select {
	case <-SIGNAL.Ctx.Done():
		fmt.Println("Signal context done:", SIGNAL.ID, SIGNAL.Ctx.Err())
		return
	default:
	}

	if sleepTimeInSeconds > 0 {
		time.Sleep(time.Duration(sleepTimeInSeconds) * time.Second)
	}

	SignalMonitor <- SIGNAL
}

var letterRunes = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ234567")
