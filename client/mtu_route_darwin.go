//go:build darwin

package client

import (
	"net"
	"os/exec"
	"strconv"
)

func addPeerMTURoute(ifName, ip string, mtu int) error {
	if ifName == "" || ip == "" || mtu <= 0 {
		return nil
	}
	args := []string{"-n", "add", "-host", ip, "-interface", ifName, "-mtu", strconv.Itoa(mtu)}
	if out, err := exec.Command("route", args...).CombinedOutput(); err != nil {
		change := []string{"-n", "change", "-host", ip, "-interface", ifName, "-mtu", strconv.Itoa(mtu)}
		if out2, err2 := exec.Command("route", change...).CombinedOutput(); err2 != nil {
			DEBUG("mtu: route add ", string(out), " change ", string(out2))
			return err2
		}
	}
	return nil
}

func delPeerMTURoute(ifName, ip string) error {
	if ip == "" {
		return nil
	}
	out, err := exec.Command("route", "-n", "delete", "-host", ip).CombinedOutput()
	if err != nil {
		DEBUG("mtu: route delete ", ip, " ", string(out))
		return err
	}
	return nil
}

func egressMTU(net.IP) int { return 0 }
