package iptables

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func SetIPTablesRSTDropFilter(interfaceIP string) (err error, exists bool) {
	cmd := exec.Command("sudo", "iptables", "-L", "-n")
	combined, cerr := cmd.CombinedOutput()
	if cerr != nil {
		cmd := exec.Command("iptables", "-L", "-n")
		combined, cerr = cmd.CombinedOutput()
		if cerr != nil {
			err = fmt.Errorf("Unable to run iptables, err:\n %s", combined)
			return
		}
	}

	lines := bytes.Split(combined, []byte{10})
	for _, v := range lines {
		if !bytes.Contains(v, []byte("DROP")) {
			continue
		}
		if !bytes.Contains(v, []byte(interfaceIP)) {
			continue
		}
		if !bytes.Contains(v, []byte("0.0.0.0/0")) {
			continue
		}
		if !bytes.Contains(v, []byte("flags:0x14/0x04")) {
			continue
		}
		exists = true
		return
	}

	params := []string{"-I", "OUTPUT", "-p", "tcp", "--src", interfaceIP, "--tcp-flags", "ACK,RST", "RST", "-j", "DROP"}
	cmd = exec.Command("sudo", append([]string{"iptables"}, params...)...)
	out, cerr := cmd.CombinedOutput()
	if cerr != nil {
		if strings.Contains(string(out), "sudo: command not found") {
			cmd = exec.Command("iptables", params...)
			out, cerr := cmd.CombinedOutput()
			if cerr != nil {
				err = fmt.Errorf("Unable to apply iptables rule, err:\n %s", out)
				return
			}
		} else {
			err = fmt.Errorf("Unable to apply iptables rule, err:\n %s", out)
			return
		}
	}
	return
}
