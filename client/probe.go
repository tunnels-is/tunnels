package client

import (
	"sort"
	"sync"
	"time"

	"github.com/tunnels-is/tunnels/types"
)

type ServerProbe struct {
	Tag      string
	IP       string
	Country  string
	ServerID string
	Latency  time.Duration
	OK       bool
}

// LatencyMS ...
func (p ServerProbe) LatencyMS() int64 {
	if !p.OK {
		return -1
	}
	return p.Latency.Milliseconds()
}

const probeConcurrency = 8

// ProbeServers ...
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
