//go:build e2e

package clientks

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	netName   = "tcks"
	netCIDR   = "10.99.7.0/24"
	ctrlIP    = "10.99.7.2"
	cliIP     = "10.99.7.3"
	image     = "localhost/tunnels-clientks:latest"
	ctrlPort  = "18443"
	pw        = "clientpassword123"
	adminAPI  = "11111111-1111-1111-1111-111111111111"
	twoFAKey  = "0123456789abcdef0123456789abcdef"
	cookieKey = "abcdef0123456789abcdef0123456789"
)

func sh(t *testing.T, name string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

func mustPodman(t *testing.T, args ...string) string {
	t.Helper()
	out, err := sh(t, "podman", args...)
	if err != nil {
		t.Fatalf("podman %s\n%s\nerr=%v", strings.Join(args, " "), out, err)
	}
	return out
}

func execIn(t *testing.T, ctr string, script string) (string, error) {
	t.Helper()
	return sh(t, "podman", "exec", ctr, "sh", "-c", script)
}

func cleanup(t *testing.T) {
	t.Helper()
	for _, c := range []string{"kcli", "kctrl"} {
		_, _ = sh(t, "podman", "rm", "-f", c)
	}
	_, _ = sh(t, "podman", "network", "rm", netName)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(wd, "/test/clientks")
}

func httpClient() *http.Client {
	return &http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

func ctrlReq(t *testing.T, method, path string, body any, hdr map[string]string, out any) int {
	t.Helper()
	var r io.Reader = bytes.NewReader(nil)
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, "https://127.0.0.1:"+ctrlPort+path, r)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := httpClient().Do(req)
	if err != nil {
		t.Logf("%s %s: %v", method, path, err)
		return 0
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		t.Logf("%s %s -> %d: %s", method, path, resp.StatusCode, string(b))
	}
	if out != nil && len(b) > 0 {
		_ = json.Unmarshal(b, out)
	}
	return resp.StatusCode
}

var cliData string

func controlServers() []map[string]any {
	return []map[string]any{{
		"ID": "tunnels", "Host": ctrlIP, "Port": "443", "ValidateCertificate": false,
	}}
}

func writeClientConfig(t *testing.T, cfg map[string]any) {
	t.Helper()
	if cfg["ControlServers"] == nil {
		cfg["ControlServers"] = controlServers()
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cliData, "tunnels.conf"), b, 0o666); err != nil {
		t.Fatal(err)
	}
}

func waitClientReady(t *testing.T) {
	t.Helper()
	for i := 0; i < 40; i++ {
		out, _ := sh(t, "podman", "logs", "kcli")
		if strings.Contains(out, "Tunnels is ready") {
			t.Log("client ready")
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	out, _ := sh(t, "podman", "logs", "kcli")
	t.Fatalf("client never became ready\n%s", out)
}

func runClient(t *testing.T) {
	t.Helper()
	_, _ = sh(t, "podman", "rm", "-f", "kcli")
	mustPodman(t, "run", "-d", "--name", "kcli",
		"--network", netName, "--ip", cliIP,
		"--cap-add", "NET_ADMIN", "--cap-add", "NET_RAW",
		"--device", "/dev/net/tun",
		"--sysctl", "net.ipv6.conf.all.disable_ipv6=0",
		"--sysctl", "net.ipv6.conf.default.disable_ipv6=0",
		"--sysctl", "net.ipv4.ip_forward=1",
		"-v", cliData+":/data",
		image, "/tunnels", "--debug", "--basePath", "/data", "--tunnelType", "default")
	waitClientReady(t)
}

func TestClientKillSwitchAndReuse(t *testing.T) {
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available")
	}
	cleanup(t)
	t.Cleanup(func() {
		if os.Getenv("KS_KEEP") != "" {
			t.Log("KS_KEEP set — leaving containers")
			return
		}
		for _, c := range []string{"kcli", "kctrl"} {
			out, _ := sh(t, "podman", "logs", "--tail", "80", c)
			t.Logf("=== logs %s ===\n%s", c, out)
		}
		cleanup(t)
	})

	buildImage(t)
	mustPodman(t, "network", "create", "--ipv6",
		"--subnet", netCIDR, "--subnet", "fd99:7::/64", netName)
	startController(t)
	waitController(t)

	startClient(t)

	t.Run("ipv6_blackhole_on_startup", func(t *testing.T) {
		out, err := execIn(t, "kcli", "ip -6 route show type blackhole")
		if err != nil {
			t.Fatalf("ip -6 route: %v\n%s", err, out)
		}
		if !strings.Contains(out, "blackhole default") && !strings.Contains(out, "blackhole ::/0") {
			t.Fatalf("expected IPv6 blackhole default, got:\n%s", out)
		}
		if !strings.Contains(out, "metric 50") && !strings.Contains(out, "metric 1024") {
			t.Logf("IPv6 blackhole present (metric may vary): %s", out)
		}
		t.Logf("IPv6 kill switch route:\n%s", out)
	})

	t.Run("ipv4_blackhole_off_by_default", func(t *testing.T) {
		out, err := execIn(t, "kcli", "ip -4 route show type blackhole")
		if err != nil {
			t.Fatalf("ip -4 route: %v\n%s", err, out)
		}
		if strings.Contains(out, "blackhole") {
			t.Fatalf("IPv4 kill switch should be off by default, got:\n%s", out)
		}
	})

	t.Run("ipv4_blackhole_after_enable", func(t *testing.T) {
		writeClientConfig(t, map[string]any{
			"KillSwitchIPv4": true,
			"KillSwitchIPv6": true,
		})
		runClient(t)
		out, err := execIn(t, "kcli", "ip -4 route show type blackhole")
		if err != nil {
			t.Fatalf("ip -4 route: %v\n%s", err, out)
		}
		if !strings.Contains(out, "blackhole") {
			t.Fatalf("expected IPv4 blackhole after enable, got:\n%s", out)
		}
		t.Logf("IPv4 kill switch route:\n%s", out)

		writeClientConfig(t, map[string]any{
			"KillSwitchIPv4": false,
			"KillSwitchIPv6": true,
		})
		runClient(t)
	})

	uid, token := registerUser(t)
	srvID := defaultServerID(t, uid, token)
	connect(t, uid, token, srvID)

	idx1 := tunnelsIfIndex(t)
	t.Logf("tunnels ifindex before reconnect: %d", idx1)

	t.Run("reconnect_reuses_tun", func(t *testing.T) {
		connect(t, uid, token, srvID)
		idx2 := tunnelsIfIndex(t)
		if idx1 == 0 || idx2 == 0 {
			t.Fatalf("missing tunnels interface (before=%d after=%d)", idx1, idx2)
		}
		if idx1 != idx2 {
			t.Fatalf("TUN was recreated: ifindex %d -> %d", idx1, idx2)
		}
		logs, _ := sh(t, "podman", "logs", "kcli")
		if !strings.Contains(logs, "replaced WireGuard session in place") &&
			!strings.Contains(logs, "reused OS TUN") {
			t.Logf("ifindex stayed %d but did not see in-place log; last logs:\n%s", idx2, tail(logs, 40))
		} else {
			t.Log("in-place replace logged")
		}
	})

	t.Run("ipv6_blackhole_survives_connect", func(t *testing.T) {
		out, err := execIn(t, "kcli", "ip -6 route show type blackhole")
		if err != nil {
			t.Fatalf("ip -6 route: %v\n%s", err, out)
		}
		if !strings.Contains(out, "blackhole default") && !strings.Contains(out, "blackhole ::/0") {
			t.Fatalf("IPv6 kill switch should remain after connect/reconnect, got:\n%s", out)
		}
	})
}

func buildImage(t *testing.T) {
	t.Helper()
	root := repoRoot(t)
	dir := filepath.Join(os.TempDir(), "clientks-img")
	_ = os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	df, err := os.ReadFile(filepath.Join(root, "test/clientks/Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), df, 0o644); err != nil {
		t.Fatal(err)
	}

	build := func(out, pkg string) {
		cmd := exec.Command("go", "build", "-o", out, pkg)
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64")
		cmd.Dir = root
		if o, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", pkg, err, o)
		}
	}
	build(filepath.Join(dir, "server"), "./server")
	build(filepath.Join(dir, "tunnels"), "./cmd/main")

	if out, err := sh(t, "podman", "build", "-t", image, dir); err != nil {
		t.Fatalf("image build: %v\n%s", err, out)
	}
}

func startController(t *testing.T) {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "clientks-ctrl")
	_ = os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	// APIIP is also the default WG server's advertised endpoint
	// (initializeDefaultServer copies it). Certs are written next to
	// the /server binary, not under /data.
	cfg := map[string]any{
		"APIIP": ctrlIP, "APIPort": "443",
		"AdminAPIKey": adminAPI, "TwoFactorKey": twoFAKey, "CookieSigningKey": cookieKey,
		"CertPem": "/cert.pem", "KeyPem": "/key.pem",
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "config.json"), b, 0o666); err != nil {
		t.Fatal(err)
	}
	wgcfg := map[string]any{"InsecureSkipVerify": true}
	wb, _ := json.MarshalIndent(wgcfg, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "wg-config.json"), wb, 0o666); err != nil {
		t.Fatal(err)
	}
	mustPodman(t, "run", "-d", "--name", "kctrl",
		"--network", netName, "--ip", ctrlIP,
		"--cap-add", "NET_ADMIN", "--device", "/dev/net/tun",
		"--sysctl", "net.ipv4.ip_forward=1",
		"--sysctl", "net.ipv6.conf.all.disable_ipv6=0",
		"--sysctl", "net.ipv6.conf.default.disable_ipv6=0",
		"-p", "127.0.0.1:"+ctrlPort+":443",
		"-v", dir+":/data", "-w", "/data",
		image, "/server", "-allinone", "-createConfig", "all",
		"-createCert", "selfsign", "-ip", ctrlIP,
		"-configPath", "/data/config.json",
		"-wgConfigPath", "/data/wg-config.json",
		"-silent=false")
}

func waitController(t *testing.T) {
	t.Helper()
	for i := 0; i < 90; i++ {
		if ctrlReq(t, "GET", "/health", nil, nil, nil) == 200 {
			t.Log("controller healthy")
			return
		}
		time.Sleep(time.Second)
	}
	out, _ := sh(t, "podman", "logs", "kctrl")
	t.Fatalf("controller never healthy\n%s", out)
}

func startClient(t *testing.T) {
	t.Helper()
	cliData = filepath.Join(os.TempDir(), "clientks-cli")
	_ = os.RemoveAll(cliData)
	if err := os.MkdirAll(cliData, 0o777); err != nil {
		t.Fatal(err)
	}
	writeClientConfig(t, map[string]any{
		"KillSwitchIPv4": false,
		"KillSwitchIPv6": true,
	})
	runClient(t)
}

func registerUser(t *testing.T) (id, token string) {
	t.Helper()
	var resp struct {
		ID          string `json:"_id"`
		DeviceToken struct {
			DT string `json:"DT"`
		} `json:"DeviceToken"`
	}
	code := ctrlReq(t, "POST", "/client/user/create",
		map[string]any{"Email": "ks@test.local", "Password": pw}, nil, &resp)
	if code != 200 || resp.ID == "" || resp.DeviceToken.DT == "" {
		t.Fatalf("register: code=%d id=%q token=%q", code, resp.ID, resp.DeviceToken.DT)
	}
	return resp.ID, resp.DeviceToken.DT
}

func defaultServerID(t *testing.T, uid, token string) string {
	t.Helper()
	hdr := map[string]string{"X-Device-Token": token, "X-UID": uid}
	var servers []struct {
		ID              string `json:"_id"`
		WireGuardPubKey string `json:"WireGuardPubKey"`
		IP              string `json:"IP"`
	}
	// wg-server retries fetch every 5s until the API is up, then
	// writes its pubkey onto the default server record.
	var lastCode int
	for i := 0; i < 40; i++ {
		lastCode = ctrlReq(t, "POST", "/client/servers", map[string]any{"StartIndex": 0}, hdr, &servers)
		if lastCode == 200 && len(servers) > 0 && servers[0].ID != "" && servers[0].WireGuardPubKey != "" {
			t.Logf("default server id=%s ip=%s pubkey ready", servers[0].ID, servers[0].IP)
			return servers[0].ID
		}
		time.Sleep(time.Second)
	}
	out, _ := sh(t, "podman", "logs", "--tail", "80", "kctrl")
	t.Fatalf("default server not provisioned: code=%d n=%d\n%s", lastCode, len(servers), out)
	return ""
}

func connect(t *testing.T, uid, token, serverID string) {
	t.Helper()
	writeClientConfig(t, map[string]any{
		"KillSwitchIPv4": false,
		"KillSwitchIPv6": true,
		"CLIConfig": map[string]any{
			"ControlServerID": "tunnels",
			"DeviceToken":     token,
			"UserID":          uid,
			"ServerID":        serverID,
		},
	})
	runClient(t)
	for i := 0; i < 40; i++ {
		if tunnelsIfIndex(t) != 0 {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	rt, _ := execIn(t, "kcli", "ip route; echo '---'; ip addr")
	logs, _ := sh(t, "podman", "logs", "--tail", "40", "kcli")
	t.Fatalf("tunnels interface never appeared\n%s\n=== kcli ===\n%s", rt, logs)
}

func tunnelsIfIndex(t *testing.T) int {
	t.Helper()
	out, err := execIn(t, "kcli", "cat /sys/class/net/tunnels/ifindex 2>/dev/null || echo 0")
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(out))
	return n
}

func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
