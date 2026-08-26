package ui

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/tunnels-is/tunnels/types"
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

// serverWGAddr is the WireGuard endpoint: public IP and WireGuard port,
// not the controller API port.
func serverWGAddr(s *types.Server) string {
	if s == nil || s.IP == "" {
		return "—"
	}
	if s.WireGuardPort <= 0 {
		return s.IP
	}
	return net.JoinHostPort(s.IP, strconv.Itoa(s.WireGuardPort))
}
