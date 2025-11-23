package main

import (
	"net"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

var (
	RatelimitMap  = make(map[string]*Ratelimiter)
	RatelimitLock = sync.Mutex{}
)

type Ratelimiter struct {
	Hit   time.Time
	Count int
}

func Ratelimit(conn net.Conn) (allowed bool) {
	defer func() {
		if r := recover(); r != nil {
			ERR(r, string(debug.Stack()))
		}
		RatelimitLock.Unlock()
	}()
	RatelimitLock.Lock()

	IP := strings.Split(conn.RemoteAddr().String(), ":")[0]

	limiter, ok := RatelimitMap[IP]
	if !ok {
		RatelimitMap[IP] = new(Ratelimiter)
		RatelimitMap[IP].Hit = time.Now()
		RatelimitMap[IP].Count = 1
		limiter = RatelimitMap[IP]
	}

	if time.Since(limiter.Hit).Seconds() > 5 {
		limiter.Hit = time.Now()
		limiter.Count = 1
	} else {
		limiter.Count++
		if limiter.Count > 100 {
			WARN( "RATELIMIT HIT FOR IP: ", IP)
			return false
		}
	}

	return true
}
