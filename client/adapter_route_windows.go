//go:build windows

package client

import "os/exec"

func setIPv4RouteMetric(network string, ifname string, metric string) (err error) {
	if err = validateRouteArgs(network, "", metric); err != nil {
		return err
	}
	if metric == "0" {
		metric = "1"
	}

	cmd := hiddenCommand(
		"netsh",
		"interface",
		"ipv4",
		"set",
		"route",
		network,
		ifname,
		"metric="+metric,
		"store=active",
	)
	DEBUG(
		"netsh",
		"interface",
		"ipv4",
		"set",
		"route",
		network,
		ifname,
		"metric="+metric,
		"store=active",
	)

	ob, cerr := cmd.Output()
	if cerr != nil {
		ERROR(fmt.Sprintf("%s - out: %s ", ob, cerr))
		return cerr
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
	if metric == "0" {
		metric = "1"
	}

	_ = delIPv4Route(network, gateway, metric)

	cmd := hiddenCommand(
		"netsh",
		"interface",
		"ipv4",
		"add",
		"route",
		network,
		ifName,
		gateway,
		metric,
		"store=active",
	)

	DEBUG(
		"netsh",
		"interface",
		"ipv4",
		"add",
		"route",
		network,
		ifName,
		gateway,
		metric,
		"store=active",
	)

	ob, cerr := cmd.Output()

	if cerr != nil {
		return fmt.Errorf("%s - out: %s", cerr, ob)
	}

	return
}

func delIPv4Route(network string, _ string, _ string) (err error) {
	cmd := hiddenCommand("route", "DELETE", network)

	DEBUG("route", "DELETE", network)

	ob, cerr := cmd.Output()
	if cerr != nil {
		ERROR(fmt.Sprintf("%s - out: %s ", ob, cerr))
		return cerr
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
	if metric == "0" {
		metric = "1"
	}

	_ = delIPv6Route(network, gateway, metric)

	dest := network
	if network == "default" {
		dest = "::/0"
	}
	cmd := netshAddIPv6RouteCmd(dest, ifName, metric)
	DEBUG(
		"netsh",
		"interface",
		"ipv6",
		"add",
		"route",
		dest,
		`interface="`+ifName+`"`,
		"metric="+metric,
		"store=active",
	)

	ob, cerr := cmd.Output()

	if cerr != nil {
		if strings.Contains(string(ob), "already exists") || strings.Contains(cerr.Error(), "already exists") {
			DEBUG("IPv6 route already exists: ", network)
			return nil
		}
		return fmt.Errorf("IPv6 route add failed: %s - out: %s", cerr, ob)
	}

	return
}

func netshAddIPv6RouteCmd(dest, ifName, metric string) *exec.Cmd {
	return hiddenCommand(
		"netsh",
		"interface",
		"ipv6",
		"add",
		"route",
		dest,
		`interface="`+ifName+`"`,
		"metric="+metric,
		"store=active",
	)
}

func delIPv6Route(network string, _ string, _ string) (err error) {
	var cmd *exec.Cmd
	if network == "default" {
		cmd = hiddenCommand(
			"netsh",
			"interface",
			"ipv6",
			"delete",
			"route",
			"::/0",
		)
		DEBUG(
			"netsh",
			"interface",
			"ipv6",
			"delete",
			"route",
			"::/0",
		)
	} else {
		cmd = hiddenCommand(
			"netsh",
			"interface",
			"ipv6",
			"delete",
			"route",
			network,
		)
		DEBUG(
			"netsh",
			"interface",
			"ipv6",
			"delete",
			"route",
			network,
		)
	}

	ob, cerr := cmd.Output()
	if cerr != nil {
		ERROR(fmt.Sprintf("IPv6 route delete failed: %s - out: %s", cerr, ob))
		return cerr
	}

	return
}
