package ui

import (
	"testing"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/client"
	"github.com/tunnels-is/tunnels/types"
)

// TestAutoProbeDecision covers the rules the web UI uses for the first probe:
// signed in, exactly once, and never while a tunnel is already up.
func TestAutoProbeDecision(t *testing.T) {
	cs := &client.ControlServer{ID: "cs-1", Host: "api.tunnels.is", Port: "443"}
	sid := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	srv := []types.Server{{ID: sid, Tag: "is-reykjavik-01", Country: "IS", IP: "82.221.10.5", Port: "443"}}
	user := func() *client.User {
		return &client.User{ID: "u1", Email: "s@min.io", ControlServer: cs}
	}

	cases := []struct {
		name string
		app  *App
		want autoProbeDecision
	}{
		{
			name: "cold start with servers loaded",
			app:  &App{user: user(), servers: srv},
			want: autoProbeRun,
		},
		{
			name: "cold start before servers arrive",
			app:  &App{user: user()},
			want: autoProbeNeedServers,
		},
		{
			name: "not signed in",
			app:  &App{servers: srv},
			want: autoProbeSkip,
		},
		{
			name: "no control server",
			app:  &App{user: &client.User{ID: "u1", Email: "s@min.io"}, servers: srv},
			want: autoProbeSkip,
		},
		{
			name: "already probed once",
			app:  &App{user: user(), servers: srv, probedOnce: true},
			want: autoProbeSkip,
		},
		{
			name: "probe already running",
			app:  &App{user: user(), servers: srv, probing: true},
			want: autoProbeSkip,
		},
		{
			name: "already connected, so the server in use is the answer",
			app: &App{user: user(), servers: srv, active: []*client.TUN{
				{ID: "t1", CR: &client.ConnectionRequest{UserID: "u1", Tag: "default", ServerID: sid.String()}},
			}},
			want: autoProbeSkip,
		},
		{
			name: "another user's tunnel does not count as connected",
			app: &App{user: user(), servers: srv, active: []*client.TUN{
				{ID: "t1", CR: &client.ConnectionRequest{UserID: "someone-else", Tag: "default", ServerID: sid.String()}},
			}},
			want: autoProbeRun,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.app.autoProbeState(); got != c.want {
				t.Errorf("autoProbeState() = %v, want %v", got, c.want)
			}
		})
	}
}

// TestAutoProbeRunsOnce guards the chain fetchServers -> maybeAutoProbe against
// re-probing every time the server list is refreshed.
func TestAutoProbeRunsOnce(t *testing.T) {
	cs := &client.ControlServer{ID: "cs-1", Host: "api.tunnels.is", Port: "443"}
	a := &App{
		user:    &client.User{ID: "u1", Email: "s@min.io", ControlServer: cs},
		servers: []types.Server{{Tag: "a", IP: "10.0.0.1"}},
	}
	if a.autoProbeState() != autoProbeRun {
		t.Fatal("expected the first call to probe")
	}
	// runProbe sets this; simulate it without sending packets.
	a.probedOnce = true
	if got := a.autoProbeState(); got != autoProbeSkip {
		t.Errorf("second call = %v, want skip", got)
	}
}
