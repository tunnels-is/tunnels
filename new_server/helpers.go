package main

import (
	"runtime/debug"
)

func BasicRecover() {
	if r := recover(); r != nil {
		ERR(r, string(debug.Stack()))
	}
}

func CopySlice(in []byte) (out []byte) {
	out = make([]byte, len(in))
	_ = copy(out, in)
	return
}
