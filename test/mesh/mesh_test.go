//go:build e2e

// Mesh end-to-end test. Spawns (rootless podman): a controller, two wg-servers
// in one mesh group, and two client peers — client B runs nginx:8080. Success =
// client A reaches client B's nginx across the server-to-server mesh.
//
// Run: go test -tags e2e -run TestMesh ./test/mesh -v -timeout 900s
//
// Requires: podman, /dev/net/tun, and the tunnels-mesh image (the test builds
// the binaries + image itself).
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
	ctrlPort = "8443" // published on host -> 443 in container

	wanCIDR   = "10.0.0.0/16"
	subnetA   = "10.0.0.0/24"
	subnetB   = "10.0.1.0/24"
	wgPort    = 51820
	meshPort  = 51821
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

func cleanup(t *testing.T) {
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

// ---- controller HTTP client (host -> published port, Go TLS supports the PQ curve) ----

type ctrl struct {
	t    *testing.T
	c    *http.Client
	base string
}

func newCtrl(t *testing.T) *ctrl {
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
		// Transport error (e.g. controller not listening yet) — return 0 so
		// pollers can retry instead of aborting the whole test.
		return 0
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	buf.ReadFrom(resp.Body)
	if out != nil && buf.Len() > 0 {
		_ = json.Unmarshal(buf.Bytes(), out)
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
		for _, c := range []string{"mwgA", "mwgB", "mcliA", "mcliB", "mctrl"} {
			out, _ := podman(t, "logs", "--tail", "40", c)
			t.Logf("=== logs %s ===\n%s", c, out)
		}
		cleanup(t)
	})

	// --- build binaries + image ---
	buildArtifacts(t)

	// --- network ---
	mustPodman(t, "network", "create", "--subnet", netCIDR, netName)

	// --- controller ---
	startController(t)
	c := newCtrl(t)
	waitController(t, c)

	adminPW := readAdminPassword(t)
	if code := c.req("POST", "/ui/user/login", map[string]any{"Email": "admin", "Password": adminPW}, nil, nil); code != 200 {
		t.Fatalf("admin login failed: %d (pw=%q)", code, adminPW)
	}
	t.Log("admin logged in")

	// --- WAN + mesh group ---
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

	// --- servers ---
	srvA := createServer(t, c, "serverA", wgAIP, subnetA, wan.ID, mg.ID)
	srvB := createServer(t, c, "serverB", wgBIP, subnetB, wan.ID, mg.ID)
	t.Logf("serverA id=%s apikey=%s", srvA.ID, srvA.APIKey)
	t.Logf("serverB id=%s apikey=%s", srvB.ID, srvB.APIKey)

	// --- users ---
	uA := createUser(t, c, "clienta@test.local")
	uB := createUser(t, c, "clientb@test.local")
	t.Logf("userA id=%s token=%s", uA.ID, uA.Token)
	t.Logf("userB id=%s token=%s", uB.ID, uB.Token)

	// --- wg-servers ---
	startWGServer(t, "mwgA", wgAIP, srvA.APIKey)
	startWGServer(t, "mwgB", wgBIP, srvB.APIKey)

	// Wait until both servers reported their pubkey, then let the fast mesh
	// reconcile (8s interval via TUNNELS_MESH_RECONCILE_SECONDS) converge
	// bidirectionally — each server picks up the sibling on its next tick.
	waitServersProvisioned(t, c, srvA.ID, srvB.ID)
	t.Log("waiting for mesh convergence")
	time.Sleep(25 * time.Second)

	// --- client devices (register pubkeys, get assigned IPs) ---
	privA, pubA := genKeypair(t)
	privB, pubB := genKeypair(t)
	devA := createDevice(t, c, uA, srvA.ID, pubA, "devA")
	devB := createDevice(t, c, uB, srvB.ID, pubB, "devB")
	t.Logf("clientA wgIP=%s serverpub=%s endpoint=%s:%d", devA.WGIP, devA.ServerPub, devA.ServerIP, devA.ServerPort)
	t.Logf("clientB wgIP=%s serverpub=%s endpoint=%s:%d", devB.WGIP, devB.ServerPub, devB.ServerIP, devB.ServerPort)

	// --- client containers ---
	startClient(t, "mcliA", cliAIP, privA, devA, false)
	startClient(t, "mcliB", cliBIP, privB, devB, true) // B runs nginx
	time.Sleep(8 * time.Second)

	// --- the actual test: client A reaches client B's nginx over the mesh ---
	var lastErr string
	ok := false
	for i := 0; i < 12; i++ {
		out, err := podman(t, "exec", "mcliA", "curl", "-s", "--max-time", "5", "http://"+devB.WGIP+":8080/")
		if err == nil && strings.Contains(out, "MESH_OK") {
			t.Logf("SUCCESS: client A reached client B nginx: %q", strings.TrimSpace(out))
			ok = true
			break
		}
		lastErr = fmt.Sprintf("attempt %d: out=%q err=%v", i, strings.TrimSpace(out), err)
		time.Sleep(5 * time.Second)
	}
	if !ok {
		// Diagnostics
		dumpDiag(t)
		t.Fatalf("client A could not reach client B nginx across the mesh. last: %s", lastErr)
	}
}

func buildArtifacts(t *testing.T) {
	t.Helper()
	// Binaries + image are (re)built so the test reflects current code.
	build := func(out, pkg string) {
		cmd := exec.Command("go", "build", "-o", out, pkg)
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64")
		cmd.Dir = repoRoot(t)
		if o, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", pkg, err, o)
		}
	}
	build("/tmp/mesh/img/server", "./server")
	build("/tmp/mesh/img/meshpeer", "./test/mesh/meshpeer")
	if out, err := podman(t, "build", "-t", "tunnels-mesh:latest", "/tmp/mesh/img"); err != nil {
		t.Fatalf("image build: %v\n%s", err, out)
	}
}

func repoRoot(t *testing.T) string {
	// test file lives in <root>/test/mesh
	wd, _ := os.Getwd()
	return strings.TrimSuffix(wd, "/test/mesh")
}

func startController(t *testing.T) {
	t.Helper()
	dir := "/tmp/mesh/ctrl"
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
	if err := os.WriteFile(dir+"/config.json", b, 0o666); err != nil {
		t.Fatal(err)
	}
	// The server binary blocks at the end of main() serving, so all setup must
	// happen in ONE invocation: createCert + createAdmin + auth run in order,
	// then it serves. The admin password is logged to stdout (read via logs).
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
				p = strings.Trim(p, "\"")
				if p != "" {
					return p
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
	t.Fatal("could not read admin password from controller logs")
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

func waitServersProvisioned(t *testing.T, c *ctrl, ids ...string) {
	t.Helper()
	for i := 0; i < 60; i++ {
		all := true
		for _, id := range ids {
			var s struct {
				WireGuardPubKey string `json:"WireGuardPubKey"`
			}
			c.req("POST", "/ui/server", map[string]any{"ServerID": id}, nil, &s)
			if s.WireGuardPubKey == "" {
				all = false
			}
		}
		if all {
			t.Log("both wg-servers reported pubkeys")
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatal("wg-servers did not report pubkeys in time")
}

type deviceInfo struct {
	WGIP       string
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
			WireGuardIP string `json:"WireGuardIP"`
		} `json:"Device"`
		ServerPubKey string `json:"ServerPubKey"`
		ServerIP     string `json:"ServerIP"`
		ServerPort   string `json:"ServerPort"`
	}
	if code := c.req("POST", "/client/device/create", body, hdr, &resp); code != 200 {
		t.Fatalf("create device %s: code=%d", tag, code)
	}
	if resp.Device.WireGuardIP == "" || resp.ServerPubKey == "" {
		t.Fatalf("device %s missing fields: %+v", tag, resp)
	}
	port := wgPort
	fmt.Sscanf(resp.ServerPort, "%d", &port)
	return deviceInfo{WGIP: resp.Device.WireGuardIP, ServerPub: resp.ServerPubKey, ServerIP: resp.ServerIP, ServerPort: port}
}

func startWGServer(t *testing.T, name, ip, apiKey string) {
	t.Helper()
	dir := "/tmp/mesh/" + name
	_ = os.RemoveAll(dir)
	_ = os.MkdirAll(dir, 0o777)
	wgcfg := map[string]any{
		"APIKey": apiKey, "ControllerURL": "https://" + ctrlIP + ":443", "InsecureSkipVerify": true,
	}
	b, _ := json.MarshalIndent(wgcfg, "", "  ")
	_ = os.WriteFile(dir+"/wg-config.json", b, 0o666)
	mustPodman(t, "run", "-d", "--name", name, "--network", netName, "--ip", ip,
		"--cap-add", "NET_ADMIN", "--device", "/dev/net/tun", "--sysctl", "net.ipv4.ip_forward=1",
		"-e", "TUNNELS_MESH_RECONCILE_SECONDS=8",
		"-v", dir+":/data", "-w", "/data",
		image, "/server", "-wg", "-wgConfigPath", "/data/wg-config.json", "-silent=false")
}

func startClient(t *testing.T, name, ip, priv string, dev deviceInfo, nginx bool) {
	t.Helper()
	peerCmd := fmt.Sprintf("/meshpeer -priv %s -serverpub %s -endpoint %s:%d -ip %s -allowed %s",
		priv, dev.ServerPub, dev.ServerIP, dev.ServerPort, dev.WGIP, wanCIDR)
	entry := peerCmd
	if nginx {
		entry = "nginx; " + peerCmd
	}
	mustPodman(t, "run", "-d", "--name", name, "--network", netName, "--ip", ip,
		"--cap-add", "NET_ADMIN", "--device", "/dev/net/tun", "--sysctl", "net.ipv4.ip_forward=1",
		image, "sh", "-c", entry)
}

func dumpDiag(t *testing.T) {
	t.Helper()
	for _, c := range []string{"mwgA", "mwgB", "mcliA", "mcliB"} {
		out, _ := podman(t, "exec", c, "sh", "-c", "ip addr; echo '--- routes ---'; ip route")
		t.Logf("=== %s net ===\n%s", c, out)
	}
}
