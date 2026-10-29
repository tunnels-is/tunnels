//go:build e2e

package mesh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	natPubA = "10.77.0.7"
	natPubB = "10.77.0.8"
	lanNetA = "tnata"
	lanNetB = "tnatb"
	lanA    = "10.88.1.0/24"
	lanB    = "10.88.2.0/24"
	natLanA = "10.88.1.10"
	natLanB = "10.88.2.10"
	cliLanA = "10.88.1.5"
	cliLanB = "10.88.2.5"
	gwA     = "10.0.0.1"
	gwB     = "10.0.1.1"
)

func TestMeshPunch(t *testing.T) {
	if _, err := os.Stat("/dev/net/tun"); err != nil {
		t.Skip("/dev/net/tun is not available")
	}
	if _, err := exec.LookPath(containerCLI()); err != nil {
		t.Skip("podman or docker not available")
	}
	cleanup(t)
	t.Cleanup(func() {
		if os.Getenv("MESH_KEEP") != "" {
			t.Log("MESH_KEEP set — leaving containers running for inspection")
			return
		}
		if t.Failed() {
			for _, c := range []string{"mnatA", "mnatB", "mwgA", "mwgB", "mcliA", "mcliB", "mctrl"} {
				out, _ := podman(t, "logs", "--tail", "160", c)
				t.Logf("=== logs %s ===\n%s", c, out)
			}
		}
		cleanup(t)
	})

	buildArtifacts(t)
	mustPodman(t, "network", "create", "--subnet", netCIDR, netName)
	mustPodman(t, "network", "create", "--subnet", lanA, lanNetA)
	mustPodman(t, "network", "create", "--subnet", lanB, lanNetB)

	startController(t)
	c := newCtrl(t)
	waitController(t, c)
	adminPW := readAdminPassword(t)
	if code := c.req("POST", "/ui/user/login", map[string]any{"Email": "admin", "Password": adminPW}, nil, nil); code != 200 {
		t.Fatalf("admin login failed: %d", code)
	}

	var wan struct {
		ID string `json:"ID"`
	}
	if code := c.req("POST", "/ui/wan/create", map[string]any{"WAN": map[string]any{"Tag": "wan1", "CIDR": wanCIDR}}, nil, &wan); code != 200 || wan.ID == "" {
		t.Fatalf("create wan: code=%d id=%q", code, wan.ID)
	}
	var mg struct {
		ID string `json:"_id"`
	}
	if code := c.req("POST", "/ui/meshgroup/create", map[string]any{"MeshGroup": map[string]any{"Tag": "mesh1"}}, nil, &mg); code != 200 || mg.ID == "" {
		t.Fatalf("create meshgroup: code=%d id=%q", code, mg.ID)
	}

	srvA := createServer(t, c, "serverA", wgAIP, subnetA, wan.ID, mg.ID, true)
	srvB := createServer(t, c, "serverB", wgBIP, subnetB, wan.ID, mg.ID, true)
	uA := createUser(t, c, "clienta@test.local")
	uB := createUser(t, c, "clientb@test.local")
	startWGServer(t, "mwgA", wgAIP, srvA.APIKey)
	startWGServer(t, "mwgB", wgBIP, srvB.APIKey)
	pubA := waitServersProvisioned(t, c, srvA.ID)
	pubB := waitServersProvisioned(t, c, srvB.ID)
	waitServerDisco(t, c, srvA.ID)
	waitServerDisco(t, c, srvB.ID)
	waitMeshRoutes(t)

	privA, cliPubA := genKeypair(t)
	privB, cliPubB := genKeypair(t)
	devA := createDevice(t, c, uA, srvA.ID, cliPubA, "devA")
	devB := createDevice(t, c, uB, srvB.ID, cliPubB, "devB")
	if devA.ServerPub == "" {
		devA.ServerPub = pubA
	}
	if devB.ServerPub == "" {
		devB.ServerPub = pubB
	}
	t.Logf("A %s on %s, B %s on %s", devA.WGIP, devA.ServerIP, devB.WGIP, devB.ServerIP)

	allowRoot := filepath.Join(os.TempDir(), "tunnels-punch-allow")
	allowA := filepath.Join(allowRoot, "a")
	allowB := filepath.Join(allowRoot, "b")
	_ = os.RemoveAll(allowRoot)
	if err := os.MkdirAll(allowA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(allowB, 0o755); err != nil {
		t.Fatal(err)
	}
	writeAllow(t, allowA, "")
	writeAllow(t, allowB, "")

	startNAT(t, "mnatA", natPubA, lanNetA, natLanA)
	startNAT(t, "mnatB", natPubB, lanNetB, natLanB)
	waitLog(t, "mnatA", "conenat ready", 30)
	waitLog(t, "mnatB", "conenat ready", 30)

	startPunchClient(t, "mcliA", cliLanA, natLanA, lanNetA, privA, devA, uA, gwA, allowA, "")
	startPunchClient(t, "mcliB", cliLanB, natLanB, lanNetB, privB, devB, uB, gwB, allowB, "clientB")
	waitClientUp(t, "mcliA", devA.WGIP)
	waitClientUp(t, "mcliB", devB.WGIP)
	waitPeerAuthorized(t, "mwgA", devA.WGIP)
	waitPeerAuthorized(t, "mwgB", devB.WGIP)
	waitLog(t, "mcliA", "punch ready", 30)
	waitLog(t, "mcliB", "punch ready", 30)

	url := fmt.Sprintf("http://%s:%d/", devB.WGIP, dummyPort)
	waitDummyLocal(t, "mcliB", devB.WGIP)

	if !t.Run("deny_until_allowed", func(t *testing.T) {
		expectHTTPFail(t, "mcliA", url)
		expectNoLog(t, "mcliA", "direct up "+devB.WGIP, 8*time.Second)
	}) {
		return
	}

	if !t.Run("port_rule_stays_on_mesh", func(t *testing.T) {
		writeAllow(t, allowA, devB.WGIP+":"+fmt.Sprint(dummyPort))
		writeAllow(t, allowB, devA.WGIP+":"+fmt.Sprint(dummyPort))
		waitLog(t, "mcliA", "announced allowall=false entries=["+devB.WGIP+":", 20)
		waitLog(t, "mcliB", "announced allowall=false entries=["+devA.WGIP+":", 20)
		waitHTTP(t, "mcliA", url, "DUMMY_OK name=clientB")
		before := ifacePackets(t, "mwgA", "wg0mesh", "tx")
		waitHTTP(t, "mcliA", url, "DUMMY_OK name=clientB")
		after := ifacePackets(t, "mwgA", "wg0mesh", "tx")
		t.Logf("mesh tx %d -> %d", before, after)
		if after <= before {
			t.Fatal("port-scoped allow reached the peer without crossing the mesh")
		}
		expectNoLogs(t, 15*time.Second,
			logAbsent{"mcliA", "direct up " + devB.WGIP},
			logAbsent{"mcliB", "direct up " + devA.WGIP},
		)
	}) {
		return
	}

	if !t.Run("bare_ip_punches_through_nat", func(t *testing.T) {
		writeAllow(t, allowA, devB.WGIP)
		writeAllow(t, allowB, devA.WGIP)
		waitLog(t, "mcliA", "direct up "+devB.WGIP+" via "+natPubB+":", 45)
		waitLog(t, "mcliB", "direct up "+devA.WGIP+" via "+natPubA+":", 45)
		extra := trafficExtra(t, "mcliA", url)
		t.Logf("server wg packet extra during direct HTTP: %d", extra)
		if extra > 6 {
			t.Fatalf("direct HTTP still crossed a wg-server (%d extra packets)", extra)
		}
	}) {
		return
	}

	t.Run("revoke_tears_direct_down", func(t *testing.T) {
		writeAllow(t, allowA, "")
		writeAllow(t, allowB, "")
		waitLog(t, "mcliA", "direct down "+devB.WGIP, 20)
		waitLog(t, "mcliB", "direct down "+devA.WGIP, 20)
		expectHTTPFail(t, "mcliA", url)
	})
}

func waitServerDisco(t *testing.T, c *ctrl, id string) {
	t.Helper()
	for i := 0; i < 30; i++ {
		var s struct {
			DiscoKey string `json:"DiscoKey"`
		}
		c.req("POST", "/ui/server", map[string]any{"ServerID": id}, nil, &s)
		if s.DiscoKey != "" {
			t.Logf("server %s disco pinned", id)
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("server %s did not pin a disco key", id)
}

func writeAllow(t *testing.T, dir, content string) {
	t.Helper()
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	path := filepath.Join(dir, "rules")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
}

func startNAT(t *testing.T, name, pub, lanNet, lanIP string) {
	t.Helper()
	mustPodman(t, "run", "-d", "--name", name, "--network", netName, "--ip", pub,
		"--cap-add", "NET_ADMIN", "--cap-add", "NET_RAW",
		"--sysctl", "net.ipv4.ip_forward=1",
		"--sysctl", "net.ipv4.conf.all.route_localnet=1",
		"--sysctl", "net.ipv4.conf.all.rp_filter=0",
		"--sysctl", "net.ipv4.conf.default.rp_filter=0",
		image, "/conenat", "-pub", pub, "-lan", lanIP)
	mustPodman(t, "network", "connect", lanNet, name, "--ip", lanIP)
}

func startPunchClient(t *testing.T, name, ip, via, lanNet, priv string, dev deviceInfo, user userInfo, gateway, allowDir, dummyName string) {
	t.Helper()
	peer := strings.Join([]string{
		"/punchpeer",
		"-priv", q(priv),
		"-serverpub", q(dev.ServerPub),
		"-endpoint", q(fmt.Sprintf("%s:%d", dev.ServerIP, dev.ServerPort)),
		"-ip", q(dev.WGIP),
		"-allowed", q(wanCIDR),
		"-device", q(dev.ID),
		"-server", q(dev.ServerID),
		"-controller", q("https://" + ctrlIP + ":443"),
		"-token", q(user.Token),
		"-uid", q(user.ID),
		"-allow", "/allow/rules",
		"-gateway", q(gateway),
	}, " ")
	script := "ip route replace default via " + via + "; "
	if dummyName != "" {
		script += fmt.Sprintf("/dummyhttp -addr %s:%d -name %s >/tmp/dummyhttp.log 2>&1 & ", dev.WGIP, dummyPort, dummyName)
	}
	script += "exec " + peer
	mustPodman(t, "run", "-d", "--name", name, "--network", lanNet, "--ip", ip,
		"--cap-add", "NET_ADMIN", "--cap-add", "NET_RAW", "--device", "/dev/net/tun",
		"--sysctl", "net.ipv4.conf.all.rp_filter=0",
		"--sysctl", "net.ipv4.conf.default.rp_filter=0",
		"-v", allowDir+":/allow",
		image, "sh", "-c", script)
}

func q(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func waitLog(t *testing.T, ctr, needle string, tries int) {
	t.Helper()
	var last string
	for i := 0; i < tries; i++ {
		last, _ = podman(t, "logs", ctr)
		if strings.Contains(last, needle) {
			t.Logf("%s logged %q", ctr, needle)
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("%s never logged %q\n%s", ctr, needle, tail(last, 80))
}

type logAbsent struct {
	ctr, needle string
}

func expectNoLogs(t *testing.T, wait time.Duration, want ...logAbsent) {
	t.Helper()
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		for _, w := range want {
			logs, _ := podman(t, "logs", w.ctr)
			if strings.Contains(logs, w.needle) {
				t.Fatalf("%s logged %q while a direct path is not allowed\n%s", w.ctr, w.needle, tail(logs, 80))
			}
		}
		time.Sleep(time.Second)
	}
	for _, w := range want {
		t.Logf("%s had no %q", w.ctr, w.needle)
	}
}

func expectNoLog(t *testing.T, ctr, needle string, wait time.Duration) {
	t.Helper()
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		logs, _ := podman(t, "logs", ctr)
		if strings.Contains(logs, needle) {
			t.Fatalf("%s logged %q while a direct path is not allowed\n%s", ctr, needle, tail(logs, 80))
		}
		time.Sleep(time.Second)
	}
	t.Logf("%s had no %q", ctr, needle)
}

func expectHTTPFail(t *testing.T, from, url string) {
	t.Helper()
	for i := 0; i < 3; i++ {
		out, err := podman(t, "exec", from, "curl", "-sS", "--max-time", "3", url)
		if err == nil && strings.Contains(out, "DUMMY_OK") {
			t.Fatalf("%s reached %s (%q) while the firewall should deny it", from, url, strings.TrimSpace(out))
		}
		time.Sleep(time.Second)
	}
	t.Logf("%s cannot reach %s", from, url)
}

func trafficExtra(t *testing.T, from, url string) int64 {
	t.Helper()
	before := serverWGPackets(t)
	time.Sleep(time.Second)
	quiet := serverWGPackets(t) - before
	before = serverWGPackets(t)
	for i := 0; i < 4; i++ {
		out, err := podman(t, "exec", from, "curl", "-sS", "--max-time", "5", url)
		if err != nil || !strings.Contains(out, "DUMMY_OK") {
			t.Fatalf("direct curl failed: %v %q", err, strings.TrimSpace(out))
		}
	}
	burst := serverWGPackets(t) - before
	if burst < quiet {
		return 0
	}
	return burst - quiet
}

func serverWGPackets(t *testing.T) int64 {
	t.Helper()
	var n int64
	for _, ctr := range []string{"mwgA", "mwgB"} {
		for _, iface := range []string{"wg0", "wg0mesh"} {
			n += ifacePackets(t, ctr, iface, "rx")
			n += ifacePackets(t, ctr, iface, "tx")
		}
	}
	return n
}

func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
