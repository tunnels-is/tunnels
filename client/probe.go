package client

import (
	"sort"
	"sync"
	"time"

	"github.com/tunnels-is/tunnels/types"
)

// ServerProbe is one server's measured round-trip time.
type ServerProbe struct {
	Tag      string
	IP       string
	Country  string
	ServerID string
	Latency  time.Duration
	OK       bool
}

// LatencyMS is the round trip in milliseconds, or -1 when unreachable.
func (p ServerProbe) LatencyMS() int64 {
	if !p.OK {
		return -1
	}
	return p.Latency.Milliseconds()
}

// probeConcurrency bounds in-flight pings. The existing auto-connect path
// probes serially, which costs up to autoConnectPingTimeout per server; a
// dashboard showing every server needs them measured in parallel.
const probeConcurrency = 8

// ProbeServers measures ICMP round-trip time to every server, fastest first.
// Unreachable servers are included with OK false so the caller can tell
// "no answer" apart from "not measured".
func ProbeServers(servers []types.Server) []ServerProbe {
	defer RecoverAndLog()

	out := make([]ServerProbe, len(servers))
	sem := make(chan struct{}, probeConcurrency)
	var wg sync.WaitGroup

	for i := range servers {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			defer RecoverAndLog()
			sem <- struct{}{}
			defer func() { <-sem }()

			s := servers[idx]
			p := ServerProbe{
				Tag:      s.Tag,
				IP:       s.IP,
				Country:  s.Country,
				ServerID: s.ID.String(),
			}
			if latency, ok := pingICMP(&s); ok {
				p.Latency, p.OK = latency, true
			}
			out[idx] = p
		}(i)
	}
	wg.Wait()

	// Reachable servers first, by latency; unreachable keep a stable order.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].OK != out[j].OK {
			return out[i].OK
		}
		if !out[i].OK {
			return false
		}
		return out[i].Latency < out[j].Latency
	})
	return out
}
