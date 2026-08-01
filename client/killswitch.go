package client

import "sync"

var (
	killSwitchMu   sync.Mutex
	killSwitchTags = map[string]struct{}{}
)

func engageKillSwitch(tag string) error {
	killSwitchMu.Lock()
	defer killSwitchMu.Unlock()
	if err := enableKillSwitch(); err != nil {
		return err
	}
	killSwitchTags[tag] = struct{}{}
	return nil
}

func releaseKillSwitch(tag string) {
	killSwitchMu.Lock()
	defer killSwitchMu.Unlock()
	delete(killSwitchTags, tag)
	if len(killSwitchTags) == 0 {
		disableKillSwitch()
	}
}

func releaseAllKillSwitches() {
	killSwitchMu.Lock()
	defer killSwitchMu.Unlock()
	clear(killSwitchTags)
	disableKillSwitch()
}
