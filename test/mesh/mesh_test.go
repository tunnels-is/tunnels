//go:build e2e

package mesh

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/curve25519"
)

const (
	netName  = "tmesh"
	netCIDR  = "10.77.0.0/24"
	ctrlIP   = "10.77.0.2"
	wgAIP    = "10.77.0.3"
	wgBIP    = "10.77.0.4"
	cliAIP   = "10.77.0.5"
	cliBIP   = "10.77.0.6"
	image    = "localhost/tunnels-mesh:latest"
	ctrlPort = "8443"

	wanCIDR   = "10.0.0.0/16"
	subnetA   = "10.0.0.0/24"
	subnetB   = "10.0.1.0/24"
	wgPort    = 51820
	meshPort  = 51821
	dummyPort = 9090
	adminAPI  = "11111111-1111-1111-1111-111111111111"
	twoFAKey  = "0123456789abcdef0123456789abcdef"
	cookieKey = "abcdef0123456789abcdef0123456789"
	pw        = "clientpassword123"
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

func podman(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return sh(t, "podman", args...)
}

func mustPodman(t *testing.T, args ...string) string {
	t.Helper()
	out, err := podman(t, args...)
	if err != nil {
		t.Fatalf("podman %s\n%s\nerr=%v", strings.Join(args, " "), out, err)
	}
	return out
}

func execIn(t *testing.T, ctr, script string) (string, error) {
	t.Helper()
	return sh(t, "podman", "exec", ctr, "sh", "-c", script)
}

func cleanup(t *testing.T) {
	t.Helper()
	for _, c := range []string{"mctrl", "mwgA", "mwgB", "mcliA", "mcliB"} {
		_, _ = podman(t, "rm", "-f", c)
	}
	_, _ = podman(t, "network", "rm", netName)
}

func genKeypair(t *testing.T) (privB64, pubB64 string) {
	t.Helper()
	var priv [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		t.Fatal(err)
	}
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64
	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(priv[:]), base64.StdEncoding.EncodeToString(pub)
}

type ctrl struct {
	t    *testing.T
	c    *http.Client
	base string
}

func newCtrl(t *testing.T) *ctrl {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	return &ctrl{
		t:    t,
		base: "https://127.0.0.1:" + ctrlPort,
		c: &http.Client{
			Timeout: 20 * time.Second,
			Jar:     jar,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

func (x *ctrl) req(method, path string, body any, hdr map[string]string, out any) int {
	x.t.Helper()
	var r *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	} else {
		r = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, x.base+path, r)
	if err != nil {
		x.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := x.c.Do(req)
	if err != nil {
		x.t.Logf("%s %s: %v", method, path, err)
		return 0
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	buf.ReadFrom(resp.Body)
	if out != nil && buf.Len() > 0 {
		if err := json.Unmarshal(buf.Bytes(), out); err != nil {
			x.t.Logf("%s %s unmarshal: %v body=%s", method, path, err, buf.String())
		}
	}
	if resp.StatusCode >= 400 {
		x.t.Logf("%s %s -> %d: %s", method, path, resp.StatusCode, buf.String())
	}
	return resp.StatusCode
}

func TestMesh(t *testing.T) {
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available")
	}
	cleanup(t)
	t.Cleanup(func() {
		if os.Getenv("MESH_KEEP") != "" {
			t.Log("MESH_KEEP set — leaving containers running for inspection")
			return
		}
		if t.Failed() {
			for _, c := range []string{"mwgA", "mwgB", "mcliA", "mcliB", "mctrl"} {
				out, _ := podman(t, "logs", "--tail", "80", c)
				t.Logf("=== logs %s ===\n%s", c, out)
			}
		}
		cleanup(t)
	})

	buildArtifacts(t)

	mustPodman(t, "network", "create", "--subnet", netCIDR, netName)

	startController(t)
	c := newCtrl(t)
	waitController(t, c)

	adminPW := readAdminPassword(t)
	if code := c.req("POST", "/ui/user/login", map[string]any{"Email": "admin", "Password": adminPW}, nil, nil); code != 200 {
		t.Fatalf("admin login failed: %d (pw=%q)", code, adminPW)
	}
	t.Log("admin logged in")

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
	t.Logf("wan=%s meshgroup=%s", wan.ID, mg.ID)

	srvA := createServer(t, c, "serverA", wgAIP, subnetA, wan.ID, mg.ID)
	srvB := createServer(t, c, "serverB", wgBIP, subnetB, wan.ID, mg.ID)
	t.Logf("serverA id=%s apikey=%s", srvA.ID, srvA.APIKey)
	t.Logf("serverB id=%s apikey=%s", srvB.ID, srvB.APIKey)

	uA := createUser(t, c, "clienta@test.local")
	uB := createUser(t, c, "clientb@test.local")
	t.Logf("userA id=%s token=%s", uA.ID, uA.Token)
	t.Logf("userB id=%s token=%s", uB.ID, uB.Token)

	startWGServer(t, "mwgA", wgAIP, srvA.APIKey)
	startWGServer(t, "mwgB", wgBIP, srvB.APIKey)

	pubA := waitServersProvisioned(t, c, srvA.ID)
	pubB := waitServersProvisioned(t, c, srvB.ID)
	if pubA == "" || pubB == "" || pubA == pubB {
		t.Fatalf("wg-servers must report distinct pubkeys, got A=%q B=%q", pubA, pubB)
	}
	waitMeshRoutes(t)
	t.Log("mesh routes installed on both wg-servers")

	privA, cliPubA := genKeypair(t)
	privB, cliPubB := genKeypair(t)
	devA := createDevice(t, c, uA, srvA.ID, cliPubA, "devA")
	devB := createDevice(t, c, uB, srvB.ID, cliPubB, "devB")
	t.Logf("clientA id=%s wgIP=%s serverID=%s serverpub=%s endpoint=%s:%d",
		devA.ID, devA.WGIP, devA.ServerID, devA.ServerPub, devA.ServerIP, devA.ServerPort)
	t.Logf("clientB id=%s wgIP=%s serverID=%s serverpub=%s endpoint=%s:%d",
		devB.ID, devB.WGIP, devB.ServerID, devB.ServerPub, devB.ServerIP, devB.ServerPort)

	assertEnrolledOnSeparateServers(t, c, uA, uB, srvA, srvB, devA, devB, pubA, pubB)

	startClient(t, "mcliA", cliAIP, privA, devA, "")
	startClient(t, "mcliB", cliBIP, privB, devB, "clientB")
	waitClientUp(t, "mcliA", devA.WGIP)
	waitClientUp(t, "mcliB", devB.WGIP)
	waitPeerAuthorized(t, "mwgA", devA.WGIP)
	waitPeerAuthorized(t, "mwgB", devB.WGIP)
	assertLiveOnSeparateServers(t, devA, devB)

	txBefore := ifacePackets(t, "mwgA", "wg0mesh", "tx")
	rxBefore := ifacePackets(t, "mwgB", "wg0mesh", "rx")

	t.Run("dummyhttp_on_b_from_a", func(t *testing.T) {
		waitDummyLocal(t, "mcliB", devB.WGIP)
		waitHTTP(t, "mcliA", fmt.Sprintf("http://%s:%d/", devB.WGIP, dummyPort), "DUMMY_OK name=clientB")
	})
	t.Run("dummyhttp_not_on_podman_lan", func(t *testing.T) {
		out, err := podman(t, "exec", "mcliA", "curl", "-sS", "--max-time", "3",
			fmt.Sprintf("http://%s:%d/", cliBIP, dummyPort))
		if err == nil && strings.Contains(out, "DUMMY_OK") {
			t.Fatalf("dummyhttp was reachable on podman LAN %s:%d (%q) — that is not the mesh", cliBIP, dummyPort, strings.TrimSpace(out))
		}
		t.Logf("LAN shortcut to %s:%d failed as required (err=%v out=%q)", cliBIP, dummyPort, err, strings.TrimSpace(out))
	})
	t.Run("a_to_b_ping", func(t *testing.T) {
		waitPing(t, "mcliA", devB.WGIP)
	})
	t.Run("b_to_a_ping", func(t *testing.T) {
		waitPing(t, "mcliB", devA.WGIP)
	})
	t.Run("traffic_crossed_mesh", func(t *testing.T) {
		txAfter := ifacePackets(t, "mwgA", "wg0mesh", "tx")
		rxAfter := ifacePackets(t, "mwgB", "wg0mesh", "rx")
		t.Logf("mwgA wg0mesh tx %d -> %d; mwgB wg0mesh rx %d -> %d", txBefore, txAfter, rxBefore, rxAfter)
		if txAfter <= txBefore {
			dumpDiag(t)
			t.Fatalf("mwgA wg0mesh tx_packets did not increase (%d -> %d); traffic did not leave via the mesh TUN", txBefore, txAfter)
		}
		if rxAfter <= rxBefore {
			dumpDiag(t)
			t.Fatalf("mwgB wg0mesh rx_packets did not increase (%d -> %d); traffic did not arrive on the mesh TUN", rxBefore, rxAfter)
		}
	})
}

func buildArtifacts(t *testing.T) {
	t.Helper()
	root := repoRoot(t)
	dir := filepath.Join(os.TempDir(), "tunnels-mesh-img")
	_ = os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	df, err := os.ReadFile(filepath.Join(root, "test/mesh/Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), df, 0o644); err != nil {
		t.Fatal(err)
	}

	build := func(out, pkg string) {
		cmd := exec.Command("go", "build", "-o", out, pkg)
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+runtime.GOARCH)
		cmd.Dir = root
		if o, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", pkg, err, o)
		}
	}
	build(filepath.Join(dir, "server"), "./server")
	build(filepath.Join(dir, "meshpeer"), "./test/mesh/meshpeer")
	build(filepath.Join(dir, "dummyhttp"), "./test/mesh/dummyhttp")
	if out, err := podman(t, "build", "-t", image, dir); err != nil {
		t.Fatalf("image build: %v\n%s", err, out)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(wd, string(filepath.Separator)+"test"+string(filepath.Separator)+"mesh")
}

func startController(t *testing.T) {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "tunnels-mesh-ctrl")
	_ = os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	cfg := map[string]any{
		"APIIP": "0.0.0.0", "APIPort": "443",
		"AdminAPIKey": adminAPI, "TwoFactorKey": twoFAKey, "CookieSigningKey": cookieKey,
		"CertPem": "/cert.pem", "KeyPem": "/key.pem", "DBurl": "",
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "config.json"), b, 0o666); err != nil {
		t.Fatal(err)
	}

	mustPodman(t, "run", "-d", "--name", "mctrl", "--network", netName, "--ip", ctrlIP,
		"-p", "127.0.0.1:"+ctrlPort+":443", "-v", dir+":/data", "-w", "/data",
		image, "/server", "-createCert", "selfsign", "-ip", ctrlIP,
		"-createAdmin", "-auth", "-configPath", "/data/config.json", "-silent=false")
}

func waitController(t *testing.T, c *ctrl) {
	t.Helper()
	for i := 0; i < 60; i++ {
		if code := c.req("GET", "/health", nil, nil, nil); code == 200 {
			t.Log("controller healthy")
			return
		}
		time.Sleep(1 * time.Second)
	}
	out, _ := podman(t, "logs", "mctrl")
	t.Fatalf("controller never became healthy\n%s", out)
}

func readAdminPassword(t *testing.T) string {
	t.Helper()
	for i := 0; i < 30; i++ {
		logs, _ := podman(t, "logs", "mctrl")
		for _, line := range strings.Split(logs, "\n") {
			if idx := strings.Index(line, "pass="); idx >= 0 {
				p := strings.TrimSpace(line[idx+len("pass="):])
				if sp := strings.IndexAny(p, " \t"); sp >= 0 {
					p = p[:sp]
				}
				p = strings.Trim(p, "\"")
				if p != "" {
					return p
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
	out, _ := podman(t, "logs", "mctrl")
	t.Fatalf("could not read admin password from controller logs\n%s", out)
	return ""
}

type serverInfo struct {
	ID     string `json:"_id"`
	APIKey string `json:"APIKey"`
}

func createServer(t *testing.T, c *ctrl, tag, ip, subnet, wanID, mgID string) serverInfo {
	t.Helper()
	body := map[string]any{"Server": map[string]any{
		"Tag": tag, "Country": "test", "IP": ip, "Port": "443",
		"WireGuardSubnet": subnet, "WireGuardPort": wgPort, "WireGuardMeshPort": meshPort,
		"WireGuardIface": "wg0", "InternetIface": "eth0",
		"WANID": wanID, "MeshGroupID": mgID, "EnableFirewall": false,
	}}
	var si serverInfo
	if code := c.req("POST", "/ui/server/create", body, nil, &si); code != 200 || si.ID == "" || si.APIKey == "" {
		t.Fatalf("create server %s: code=%d id=%q apikey=%q", tag, code, si.ID, si.APIKey)
	}
	return si
}

type userInfo struct {
	ID    string
	Token string
}

func createUser(t *testing.T, c *ctrl, email string) userInfo {
	t.Helper()
	var resp struct {
		ID          string `json:"_id"`
		DeviceToken struct {
			DT string `json:"DT"`
		} `json:"DeviceToken"`
	}
	if code := c.req("POST", "/client/user/create", map[string]any{"Email": email, "Password": pw}, nil, &resp); code != 200 || resp.ID == "" || resp.DeviceToken.DT == "" {
		t.Fatalf("create user %s: code=%d id=%q token=%q", email, code, resp.ID, resp.DeviceToken.DT)
	}
	return userInfo{ID: resp.ID, Token: resp.DeviceToken.DT}
}

func waitServersProvisioned(t *testing.T, c *ctrl, id string) string {
	t.Helper()
	for i := 0; i < 60; i++ {
		var s struct {
			WireGuardPubKey string `json:"WireGuardPubKey"`
			IP              string `json:"IP"`
		}
		c.req("POST", "/ui/server", map[string]any{"ServerID": id}, nil, &s)
		if s.WireGuardPubKey != "" {
			t.Logf("wg-server %s provisioned pubkey=%s ip=%s", id, s.WireGuardPubKey, s.IP)
			return s.WireGuardPubKey
		}
		time.Sleep(2 * time.Second)
	}
	dumpDiag(t)
	t.Fatalf("wg-server %s did not report a pubkey in time", id)
	return ""
}

func waitMeshRoutes(t *testing.T) {
	t.Helper()
	var lastA, lastB string
	for i := 0; i < 40; i++ {
		a, _ := execIn(t, "mwgA", "ip route show "+subnetB)
		b, _ := execIn(t, "mwgB", "ip route show "+subnetA)
		lastA, lastB = strings.TrimSpace(a), strings.TrimSpace(b)
		if strings.Contains(lastA, "wg0mesh") && strings.Contains(lastB, "wg0mesh") {
			t.Logf("mwgA route: %s", lastA)
			t.Logf("mwgB route: %s", lastB)
			return
		}
		time.Sleep(2 * time.Second)
	}
	dumpDiag(t)
	t.Fatalf("mesh routes never appeared. mwgA=%q mwgB=%q", lastA, lastB)
}

type deviceInfo struct {
	ID         string
	WGIP       string
	ServerID   string
	ServerPub  string
	ServerIP   string
	ServerPort int
}

func createDevice(t *testing.T, c *ctrl, u userInfo, serverID, pubKey, tag string) deviceInfo {
	t.Helper()
	hdr := map[string]string{"X-Device-Token": u.Token, "X-UID": u.ID}
	body := map[string]any{"Device": map[string]any{
		"Tag": tag, "ServerID": serverID, "WireGuardKey": pubKey,
	}}
	var resp struct {
		Device struct {
			ID          string `json:"_id"`
			ServerID    string `json:"ServerID"`
			WireGuardIP string `json:"WireGuardIP"`
		} `json:"Device"`
		ServerPubKey string `json:"ServerPubKey"`
		ServerIP     string `json:"ServerIP"`
		ServerPort   string `json:"ServerPort"`
		ServerSubnet string `json:"ServerSubnet"`
	}
	if code := c.req("POST", "/client/device/create", body, hdr, &resp); code != 200 {
		t.Fatalf("create device %s: code=%d", tag, code)
	}
	if resp.Device.ID == "" || resp.Device.WireGuardIP == "" || resp.ServerPubKey == "" || resp.ServerIP == "" {
		t.Fatalf("device %s missing fields: %+v", tag, resp)
	}
	port := wgPort
	fmt.Sscanf(resp.ServerPort, "%d", &port)
	sid := resp.Device.ServerID
	if sid == "" {
		sid = serverID
	}
	return deviceInfo{
		ID:         resp.Device.ID,
		WGIP:       resp.Device.WireGuardIP,
		ServerID:   sid,
		ServerPub:  resp.ServerPubKey,
		ServerIP:   resp.ServerIP,
		ServerPort: port,
	}
}

func startWGServer(t *testing.T, name, ip, apiKey string) {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "tunnels-mesh-"+name)
	_ = os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	wgcfg := map[string]any{
		"APIKey": apiKey, "ControllerURL": "https://" + ctrlIP + ":443", "InsecureSkipVerify": true,
	}
	b, _ := json.MarshalIndent(wgcfg, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "wg-config.json"), b, 0o666); err != nil {
		t.Fatal(err)
	}
	mustPodman(t, "run", "-d", "--name", name, "--network", netName, "--ip", ip,
		"--cap-add", "NET_ADMIN", "--device", "/dev/net/tun",
		"--sysctl", "net.ipv4.ip_forward=1",
		"--sysctl", "net.ipv4.conf.all.rp_filter=0",
		"--sysctl", "net.ipv4.conf.default.rp_filter=0",
		"--sysctl", "net.ipv6.conf.all.disable_ipv6=0",
		"--sysctl", "net.ipv6.conf.default.disable_ipv6=0",
		"-e", "TUNNELS_MESH_RECONCILE_SECONDS=5",
		"-v", dir+":/data", "-w", "/data",
		image, "/server", "-wg", "-wgConfigPath", "/data/wg-config.json", "-silent=false")
}

func startClient(t *testing.T, name, ip, priv string, dev deviceInfo, dummyName string) {
	t.Helper()
	peerCmd := fmt.Sprintf("/meshpeer -priv %s -serverpub %s -endpoint %s:%d -ip %s -allowed %s",
		priv, dev.ServerPub, dev.ServerIP, dev.ServerPort, dev.WGIP, wanCIDR)
	script := peerCmd
	if dummyName != "" {
		script = fmt.Sprintf("/dummyhttp -addr %s:%d -name %s >/tmp/dummyhttp.log 2>&1 & exec %s",
			dev.WGIP, dummyPort, dummyName, peerCmd)
	}
	mustPodman(t, "run", "-d", "--name", name, "--network", netName, "--ip", ip,
		"--cap-add", "NET_ADMIN", "--cap-add", "NET_RAW", "--device", "/dev/net/tun",
		"--sysctl", "net.ipv4.ip_forward=1",
		image, "sh", "-c", script)
}

func waitClientUp(t *testing.T, name, wgIP string) {
	t.Helper()
	for i := 0; i < 30; i++ {
		out, _ := execIn(t, name, "ip addr show wg0")
		if strings.Contains(out, wgIP) {
			t.Logf("%s wg0 is up with %s", name, wgIP)
			return
		}
		time.Sleep(1 * time.Second)
	}
	out, _ := podman(t, "logs", name)
	dumpDiag(t)
	t.Fatalf("%s wg0 never got %s\n%s", name, wgIP, out)
}

func waitHTTP(t *testing.T, from, url, want string) {
	t.Helper()
	var last string
	for i := 0; i < 16; i++ {
		out, err := podman(t, "exec", from, "curl", "-sS", "--max-time", "5", url)
		last = fmt.Sprintf("out=%q err=%v", strings.TrimSpace(out), err)
		if err == nil && strings.Contains(out, want) {
			t.Logf("%s reached %s: %q", from, url, strings.TrimSpace(out))
			return
		}
		time.Sleep(3 * time.Second)
	}
	dumpDiag(t)
	t.Fatalf("%s could not HTTP to %s. last: %s", from, url, last)
}

func waitDummyLocal(t *testing.T, ctr, wgIP string) {
	t.Helper()
	url := fmt.Sprintf("http://%s:%d/", wgIP, dummyPort)
	var last string
	for i := 0; i < 30; i++ {
		out, err := podman(t, "exec", ctr, "curl", "-sS", "--max-time", "2", url)
		last = fmt.Sprintf("out=%q err=%v", strings.TrimSpace(out), err)
		if err == nil && strings.Contains(out, "DUMMY_OK") {
			t.Logf("dummyhttp is up on %s (%s)", ctr, strings.TrimSpace(out))
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	logs, _ := execIn(t, ctr, "cat /tmp/dummyhttp.log 2>/dev/null; ps aux")
	dumpDiag(t)
	t.Fatalf("dummyhttp never came up on %s %s. last: %s\n%s", ctr, url, last, logs)
}

func waitPeerAuthorized(t *testing.T, wgCtr, wgIP string) {
	t.Helper()
	needle := "ip= " + wgIP
	for i := 0; i < 30; i++ {
		logs, _ := podman(t, "logs", wgCtr)
		if strings.Contains(logs, "reconcilePeer: authorized") && strings.Contains(logs, needle) {
			t.Logf("%s authorized peer %s", wgCtr, wgIP)
			return
		}
		time.Sleep(1 * time.Second)
	}
	dumpDiag(t)
	t.Fatalf("%s never authorized WireGuard peer %s", wgCtr, wgIP)
}

func assertEnrolledOnSeparateServers(t *testing.T, c *ctrl, uA, uB userInfo, srvA, srvB serverInfo, devA, devB deviceInfo, pubA, pubB string) {
	t.Helper()
	if srvA.ID == srvB.ID {
		t.Fatalf("server A and B have the same id %s", srvA.ID)
	}
	if devA.ServerID != srvA.ID {
		t.Fatalf("device A ServerID=%s, want server A %s", devA.ServerID, srvA.ID)
	}
	if devB.ServerID != srvB.ID {
		t.Fatalf("device B ServerID=%s, want server B %s", devB.ServerID, srvB.ID)
	}
	if devA.ServerIP != wgAIP {
		t.Fatalf("device A endpoint IP=%s, want wg-server A %s", devA.ServerIP, wgAIP)
	}
	if devB.ServerIP != wgBIP {
		t.Fatalf("device B endpoint IP=%s, want wg-server B %s", devB.ServerIP, wgBIP)
	}
	if devA.ServerPub != pubA {
		t.Fatalf("device A server pubkey=%s, want %s", devA.ServerPub, pubA)
	}
	if devB.ServerPub != pubB {
		t.Fatalf("device B server pubkey=%s, want %s", devB.ServerPub, pubB)
	}
	if devA.ServerPub == devB.ServerPub {
		t.Fatal("both devices were given the same server pubkey — they are not on separate wg-servers")
	}
	if !strings.HasPrefix(devA.WGIP, "10.0.0.") {
		t.Fatalf("client A should be in %s, got %s", subnetA, devA.WGIP)
	}
	if !strings.HasPrefix(devB.WGIP, "10.0.1.") {
		t.Fatalf("client B should be in %s, got %s", subnetB, devB.WGIP)
	}

	gotA := fetchDevice(t, c, uA, devA.ID)
	gotB := fetchDevice(t, c, uB, devB.ID)
	if gotA.ServerID != srvA.ID || gotA.WireGuardIP != devA.WGIP {
		t.Fatalf("controller device A = %+v, want server %s ip %s", gotA, srvA.ID, devA.WGIP)
	}
	if gotB.ServerID != srvB.ID || gotB.WireGuardIP != devB.WGIP {
		t.Fatalf("controller device B = %+v, want server %s ip %s", gotB, srvB.ID, devB.WGIP)
	}
	t.Logf("enrollment: A on %s (%s/%s) B on %s (%s/%s)",
		srvA.ID, wgAIP, subnetA, srvB.ID, wgBIP, subnetB)
}

func fetchDevice(t *testing.T, c *ctrl, u userInfo, deviceID string) struct {
	ServerID    string `json:"ServerID"`
	WireGuardIP string `json:"WireGuardIP"`
} {
	t.Helper()
	hdr := map[string]string{"X-Device-Token": u.Token, "X-UID": u.ID}
	var d struct {
		ServerID    string `json:"ServerID"`
		WireGuardIP string `json:"WireGuardIP"`
	}
	if code := c.req("POST", "/client/device", map[string]any{"DeviceID": deviceID}, hdr, &d); code != 200 || d.ServerID == "" {
		t.Fatalf("fetch device %s: code=%d %+v", deviceID, code, d)
	}
	return d
}

func assertLiveOnSeparateServers(t *testing.T, devA, devB deviceInfo) {
	t.Helper()
	mustRouteVia(t, "mcliA", devB.WGIP, "wg0")
	mustRouteVia(t, "mcliB", devA.WGIP, "wg0")
	mustRouteVia(t, "mwgA", devB.WGIP, "wg0mesh")
	mustRouteVia(t, "mwgB", devA.WGIP, "wg0mesh")
	mustRouteVia(t, "mwgA", devA.WGIP, "wg0")
	mustRouteVia(t, "mwgB", devB.WGIP, "wg0")

	logsA, _ := podman(t, "logs", "mwgA")
	logsB, _ := podman(t, "logs", "mwgB")
	if strings.Contains(logsA, "ip= "+devB.WGIP) {
		t.Fatalf("mwgA authorized client B (%s) — B is not supposed to be a local peer of server A", devB.WGIP)
	}
	if strings.Contains(logsB, "ip= "+devA.WGIP) {
		t.Fatalf("mwgB authorized client A (%s) — A is not supposed to be a local peer of server B", devA.WGIP)
	}
	t.Log("live topology: A peers only mwgA, B peers only mwgB, remote subnets via wg0mesh")
}

func mustRouteVia(t *testing.T, ctr, dest, iface string) {
	t.Helper()
	out, err := execIn(t, ctr, "ip route get "+dest)
	if err != nil {
		dumpDiag(t)
		t.Fatalf("%s ip route get %s: %v\n%s", ctr, dest, err, out)
	}
	if !strings.Contains(out, "dev "+iface) {
		dumpDiag(t)
		t.Fatalf("%s route to %s is not via %s:\n%s", ctr, dest, iface, out)
	}
	t.Logf("%s: %s via %s (%s)", ctr, dest, iface, strings.TrimSpace(strings.ReplaceAll(out, "\n", " ")))
}

func waitPing(t *testing.T, from, destIP string) {
	t.Helper()
	var last string
	for i := 0; i < 8; i++ {
		out, err := execIn(t, from, "ping -c 1 -W 3 "+destIP)
		last = strings.TrimSpace(out)
		if err == nil && strings.Contains(out, "1 received") {
			t.Logf("%s ping %s ok", from, destIP)
			return
		}
		time.Sleep(2 * time.Second)
	}
	dumpDiag(t)
	t.Fatalf("%s could not ping %s. last:\n%s", from, destIP, last)
}

func ifacePackets(t *testing.T, ctr, iface, dir string) int64 {
	t.Helper()
	out, err := execIn(t, ctr, "cat /sys/class/net/"+iface+"/statistics/"+dir+"_packets")
	if err != nil {
		t.Fatalf("%s %s %s_packets: %v\n%s", ctr, iface, dir, err, out)
	}
	n, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	if err != nil {
		t.Fatalf("parse %s %s %s_packets %q: %v", ctr, iface, dir, out, err)
	}
	return n
}

func dumpDiag(t *testing.T) {
	t.Helper()
	for _, c := range []string{"mwgA", "mwgB", "mcliA", "mcliB"} {
		out, _ := execIn(t, c, "echo '=== addr ==='; ip addr; echo '=== routes ==='; ip route; echo '=== iptables filter ==='; iptables -S 2>/dev/null; echo '=== iptables nat ==='; iptables -t nat -S 2>/dev/null; echo '=== dummyhttp ==='; cat /tmp/dummyhttp.log 2>/dev/null")
		t.Logf("=== %s net ===\n%s", c, out)
		logs, _ := podman(t, "logs", "--tail", "60", c)
		t.Logf("=== %s logs ===\n%s", c, logs)
	}
}
