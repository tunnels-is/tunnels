package wgserver

import (
	"fmt"
	"log/slog"
)

var logger *slog.Logger

func init() {
	logger = slog.Default()
}

func INFO(x ...any) { logger.Info("INFO", "msg", buildOut(x)) }
func WARN(x ...any) { logger.Warn("WARN", "msg", buildOut(x)) }
func ERR(x ...any)  { logger.Error("ERROR", "msg", buildOut(x)) }

func buildOut(x ...any) (out string) {
	for _, v := range x {
		out += fmt.Sprint(v)
	}
	return out
}
