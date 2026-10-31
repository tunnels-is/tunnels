package wgserver

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

var logger *slog.Logger

func init() {
	logger = slog.Default()
}

// INFO ...
func INFO(x ...any) { logger.Info("INFO", "msg", buildOut(x)) }

// WARN ...
func WARN(x ...any) { logger.Warn("WARN", "msg", buildOut(x)) }

// ERR ...
func ERR(x ...any) { logger.Error("ERROR", "msg", buildOut(x)) }

type warnGate struct {
	mu    sync.Mutex
	last  map[string]time.Time
	every time.Duration
}

func newWarnGate(every time.Duration) *warnGate {
	return &warnGate{every: every, last: make(map[string]time.Time)}
}

func (g *warnGate) do(key string, args ...any) {
	now := time.Now()
	g.mu.Lock()
	if t, ok := g.last[key]; ok && now.Sub(t) < g.every {
		g.mu.Unlock()
		return
	}
	g.last[key] = now
	g.mu.Unlock()
	WARN(args...)
}

var slowWarn = newWarnGate(time.Second)

func buildOut(x ...any) (out string) {
	for _, v := range x {
		out += fmt.Sprint(v)
	}
	return out
}
