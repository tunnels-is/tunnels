package ui

import (
	"fmt"
	"time"
)

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Local().Format("2006-01-02 15:04")
}

func fmtTimeShort(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Local().Format("01-02 15:04")
}

// fmtCount renders "3 accounts saved" style subtitles.
func fmtCount(n int, suffix string) string {
	return fmt.Sprintf("%d %s", n, suffix)
}
