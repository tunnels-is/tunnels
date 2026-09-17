//go:build darwin

package client

import (
	"os/exec"
)

func addDefaultIPv4Route(gateway string) (err error) {
	if err = validateRouteArgs("", gateway, ""); err != nil {
		return err
	}
	DEBUG("route", "add", "default", gateway)

	out, err := exec.Command("route", "add", "default", gateway).CombinedOutput()
	if err != nil {
		ERROR("Unable to add route: ", string(out), " err: ", err)
		return err
	}
	return
}

func delDefaultIPv4Route() (err error) {
	DEBUG("route", "delete", "default")

	out, err := exec.Command("route", "delete", "default").CombinedOutput()
	if err != nil {
		ERROR("Unable to delete route: ", string(out), " err: ", err)
		return err
	}
	return
}

func addIPv4Route(
	network string,
	ifName string,
	gateway string,
	metric string,
) (err error) {
	if err = validateRouteArgs(network, gateway, metric); err != nil {
		return err
	}
	_ = delIPv4Route(network, "", metric)

	args := []string{"-n", "add"}
	if ifName != "" {
		args = append(args, "-ifscope", ifName)
	}
	args = append(args, "-net", network, gateway)

	DEBUG("route ", args)
	out, err := exec.Command("route", args...).CombinedOutput()
	if err != nil {
		ERROR("Unable to add route: ", string(out), " err: ", err)
		return err
	}

	return
}

func delIPv4Route(network string, gateway string, metric string) (err error) {
	DEBUG("route", "-n", "delete", "-net", network)

	out, err := exec.Command("route", "-n", "delete", "-net", network).CombinedOutput()
	if err != nil {
		ERROR("Unable to delete route: ", string(out), " err: ", err)
		return err
	}

	return
}

func addIPv6Route(
	network string,
	ifName string,
	gateway string,
	metric string,
) (err error) {
	if err = validateRouteArgs(network, "", metric); err != nil {
		return err
	}
	_ = delIPv6Route(network, gateway, metric)

	var cmd *exec.Cmd
	if network == "default" {
		DEBUG("route", "-n", "add", "-inet6", "default", "-interface", ifName)
		cmd = exec.Command("route", "-n", "add", "-inet6", "default", "-interface", ifName)
	} else {
		DEBUG("route", "-n", "add", "-inet6", "-net", network, "-interface", ifName)
		cmd = exec.Command("route", "-n", "add", "-inet6", "-net", network, "-interface", ifName)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "File exists") || strings.Contains(err.Error(), "exists") {
			DEBUG("IPv6 route already exists: ", network)
			return nil
		}
		ERROR("Unable to add IPv6 route: ", err, " out: ", string(out))
		return err
	}

	return
}

func delIPv6Route(network string, _ string, _ string) (err error) {
	var cmd *exec.Cmd
	if network == "default" {
		DEBUG("route", "-n", "delete", "-inet6", "default")
		cmd = exec.Command("route", "-n", "delete", "-inet6", "default")
	} else {
		DEBUG("route", "-n", "delete", "-inet6", "-net", network)
		cmd = exec.Command("route", "-n", "delete", "-inet6", "-net", network)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "not in table") || strings.Contains(string(out), "No such process") {
			DEBUG("IPv6 route doesn't exist (already deleted): ", network)
			return nil
		}
		ERROR("Unable to delete IPv6 route: ", err, " out: ", string(out))
		return err
	}

	return
}

func AdjustRoutersForTunneling() (err error) {
	return nil
}
