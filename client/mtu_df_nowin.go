//go:build !windows

package client

import wgconn "golang.zx2c4.com/wireguard/conn"

func setExtraBindDontFrag(wgconn.Bind) func() { return func() {} }
