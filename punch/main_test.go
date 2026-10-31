package punch

import (
	"os"
	"testing"
)

// TestMain pins timer jitter to zero so the legacy tests that check exact
// keepalive boundaries (e695dc8 gating) stay deterministic. Jitter itself
// is covered by tests that switch it back on per engine.
func TestMain(m *testing.M) {
	timerJitter = 0
	os.Exit(m.Run())
}
