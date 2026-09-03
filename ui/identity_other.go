//go:build !linux

package ui

func setLinuxWindowIdentity() {}

func registerLinuxDesktop([]byte) {}
