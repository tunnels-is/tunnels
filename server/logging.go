package main

import "fmt"

// LOG ...
func LOG(x ...any) {
	logger.Info("INFO", "msg", buildOut(x))
}

// INFO ...
func INFO(x ...any) {
	logger.Info("INFO", "msg", buildOut(x))
}

// WARN ...
func WARN(x ...any) {
	logger.Warn("WARN", "msg", buildOut(x))
}

// ERR ...
func ERR(x ...any) {
	logger.Error("ERROR", "msg", buildOut(x))
}

// ADMIN ...
func ADMIN(x ...any) {
	logger.Warn("ADMIN", "msg", buildOut(x))
}

// buildOut ...
// we will eventually replace this and the calling functions
// with something better
func buildOut(x ...any) (out string) {
	for _, v := range x {
		out += fmt.Sprint(v)
	}
	return
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
