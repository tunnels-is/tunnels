package client

import "sync"

// The route kill switch is a single, process-wide blackhole default route, but
// several tunnels can each require it (any tunnel with KillSwitch +
// EnableDefaultRoute). killSwitchTags refcounts which tunnels currently need it
// so the blackhole is only lifted once NO tunnel still does — otherwise
// disconnecting or reconnecting one kill-switch tunnel would strip the
// protection from another that is still down and leak its traffic.
//
// enableKillSwitch/disableKillSwitch (platform-specific) install/remove the
// actual route and are idempotent. killSwitchMu MUST span each
// enable/disable-plus-bookkeeping sequence: the route state and the tag set are
// two pieces of shared state, and a check-then-act on them from concurrent
// engage/release calls (each handleTunnelDeath and HTTP_Disconnect runs on its
// own goroutine) would otherwise race — e.g. release(B) could observe an empty
// set and tear down the route in the window after engage(A)'s "already active"
// no-op enable but before it records tag A, leaving A unprotected.
var (
	killSwitchMu   sync.Mutex
	killSwitchTags = map[string]struct{}{}
)

// engageKillSwitch installs the blackhole route and, only if that succeeds,
// records that tag needs it — so a failed install doesn't leave a phantom
// refcount entry that would suppress a later disable. enableKillSwitch is
// idempotent (returns nil when already active for another tag).
func engageKillSwitch(tag string) error {
	killSwitchMu.Lock()
	defer killSwitchMu.Unlock()
	if err := enableKillSwitch(); err != nil {
		return err
	}
	killSwitchTags[tag] = struct{}{}
	return nil
}

// releaseKillSwitch drops tag's need for the kill switch and lifts the blackhole
// only when no other tunnel still requires it.
func releaseKillSwitch(tag string) {
	killSwitchMu.Lock()
	defer killSwitchMu.Unlock()
	delete(killSwitchTags, tag)
	if len(killSwitchTags) == 0 {
		disableKillSwitch()
	}
}

// releaseAllKillSwitches clears all tracking and lifts the blackhole. Used on a
// clean process exit and when a user disconnect can't be scoped to one tunnel.
func releaseAllKillSwitches() {
	killSwitchMu.Lock()
	defer killSwitchMu.Unlock()
	clear(killSwitchTags)
	disableKillSwitch()
}
