//go:build windows

package client

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// hiddenCommand builds a command that never shows a console window.
//
// The desktop app is linked as a GUI binary, so it has no console of its own.
// Spawning a console program like route.exe or netsh.exe makes Windows create
// one for the child, which flashes on screen and steals focus. Anything on a
// timer turns that into a visible flicker every few seconds.
//
// CREATE_NO_WINDOW stops the console being created at all; HideWindow is kept
// alongside it because it is what the rest of the codebase already relied on
// and it costs nothing.
//
// Every exec in the Windows client goes through here, so a new call site cannot
// reintroduce the flicker by forgetting to set SysProcAttr.
func hiddenCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
	return cmd
}
