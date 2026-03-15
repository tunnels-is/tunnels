package wgserver

import (
	"os"

	"golang.zx2c4.com/wireguard/tun"
)

type inspectingTUN struct {
	tun.Device
}

func (t *inspectingTUN) Write(bufs [][]byte, offset int) (int, error) {
	return t.Device.Write(bufs, offset)
}

func (t *inspectingTUN) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	return t.Device.Read(bufs, sizes, offset)
}

func (t *inspectingTUN) File() *os.File {
	return t.Device.File()
}
