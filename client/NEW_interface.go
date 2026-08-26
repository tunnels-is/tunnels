package client

import "time"

func AutoConnect() {
	defer RecoverAndLog()
	autoConnectWith(PublicConnect)
}

func autoConnectWith(connect func(*ConnectionRequest) (int, error)) {
	if connect == nil {
		return
	}

	if s := STATE.Load(); s == nil || s.ActiveAccountHash == "" {
		if err := activateSoleAccount(); err != nil {
			ERROR("auto-connect: unable to activate account: ", err)
			return
		}
	}

	user := userForActiveAccount()
	if user == nil {
		return
	}
	if user.DeviceToken == nil || user.DeviceToken.DT == "" {
		ERROR("auto-connect: active account has no device token")
		return
	}
	cs := controlServerForUser(user)
	if cs == nil {
		ERROR("auto-connect: no control server for account")
		return
	}

	tunnelMetaMapRange(func(meta *TunnelMETA) bool {
		if meta == nil || !meta.AutoConnect || meta.ServerID == "" {
			return true
		}
		if tunnelBusy(meta.Tag) {
			return true
		}

		INFO("auto-connect: bringing up ", meta.Tag, " server ", meta.ServerID)
		code, err := connect(&ConnectionRequest{
			UserID:      user.ID,
			DeviceToken: user.DeviceToken.DT,
			Tag:         meta.Tag,
			ServerID:    meta.ServerID,
			Server:      cs,
		})
		if err != nil {
			ERROR("auto-connect: ", meta.Tag, " failed, code: ", code, " err: ", err)
		}
		return true
	})
}

func tunnelBusy(tag string) bool {
	busy := false
	tunnelMapRange(func(tun *TUN) bool {
		if tun == nil {
			return true
		}
		m := tun.meta.Load()
		if m == nil || m.Tag != tag {
			return true
		}
		if tun.GetState() >= TUN_Connecting {
			busy = true
			return false
		}
		return false
	})
	return busy
}

func userForActiveAccount() *User {
	s := STATE.Load()
	if s == nil || s.ActiveAccountHash == "" {
		return nil
	}
	users, err := getUsers()
	if err != nil {
		ERROR("auto-connect: unable to load accounts: ", err)
		return nil
	}
	for _, u := range users {
		if u != nil && u.SaveFileHash == s.ActiveAccountHash {
			return u
		}
	}
	return nil
}

func controlServerForUser(u *User) *ControlServer {
	if u != nil && u.ControlServer != nil {
		return u.ControlServer
	}
	conf := CONFIG.Load()
	if conf != nil && len(conf.ControlServers) > 0 {
		return conf.ControlServers[0]
	}
	return nil
}

func autoConnectRetryWait() {
	if CancelContext == nil {
		time.Sleep(30 * time.Second)
		return
	}
	select {
	case <-CancelContext.Done():
	case <-time.After(30 * time.Second):
	}
}
