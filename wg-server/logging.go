package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
)

var (
	logger      *slog.Logger
	disableLogs bool
)

func LOG(x ...any)   { logger.Info("INFO", "msg", buildOut(x)) }
func INFO(x ...any)  { logger.Info("INFO", "msg", buildOut(x)) }
func DEBUG(x ...any) { logger.Debug("DEBUG", "msg", buildOut(x)) }
func WARN(x ...any)  { logger.Warn("WARN", "msg", buildOut(x)) }
func ERR(x ...any)   { logger.Error("ERROR", "msg", buildOut(x)) }

func buildOut(x ...any) (out string) {
	for _, v := range x {
		out += fmt.Sprint(v)
	}
	return out
}

func getLogLevelInt(level string) int {
	switch level {
	case "debug":
		return -4
	case "info":
		return 0
	case "warn":
		return 4
	case "error":
		return 8
	default:
		return -4
	}
}

func initLogging(silent, jsonLogs, sourceInfo bool, logLevel string) {
	var logHandler slog.Handler
	slogConfig := &slog.HandlerOptions{
		Level:     slog.Level(getLogLevelInt(logLevel)),
		AddSource: sourceInfo,
	}
	if !silent {
		if !jsonLogs {
			logHandler = slog.NewTextHandler(os.Stdout, slogConfig)
		} else {
			logHandler = slog.NewJSONHandler(os.Stdout, slogConfig)
		}
	} else {
		disableLogs = true
		if !jsonLogs {
			logHandler = slog.NewTextHandler(io.Discard, slogConfig)
		} else {
			logHandler = slog.NewJSONHandler(io.Discard, slogConfig)
		}
	}
	logger = slog.New(logHandler)
	slog.SetDefault(logger)
}
