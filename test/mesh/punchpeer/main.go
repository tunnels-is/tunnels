package main

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	neturl "net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/punch"
	"golang.org/x/crypto/curve25519"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

func main() {
	iface := flag.String("iface", "wg0", "wg interface name")
	privB64 := flag.String("priv", "", "client private key (base64)")
	serverPubB64 := flag.String("serverpub", "", "wg-server public key (base64)")
	endpoint := flag.String("endpoint", "", "wg-server endpoint host:port")
	wgIP := flag.String("ip", "", "this client's WireGuard IPv4")
	allowed := flag.String("allowed", "", "routed CIDR (the WAN)")
	mtu := flag.Int("mtu", 1380, "tunnel MTU")
	deviceID := flag.String("device", "", "controller device id")
	serverID := flag.String("server", "", "controller server id")
	controller := flag.String("controller", "", "controller base URL")
	token := flag.String("token", "", "device token")
	uid := flag.String("uid", "", "user id")
	allowPath := flag.String("allow", "", "allow-file path")
	gateway := flag.String("gateway", "", "firewall announce address")
	flag.Parse()

	privRaw, err := base64.StdEncoding.DecodeString(*privB64)
	if err != nil || len(privRaw) != 32 {
		fatalf("bad private key: %v", err)
	}
	var wgPriv [32]byte
	copy(wgPriv[:], privRaw)
	pubRaw, err := curve25519.X25519(wgPriv[:], curve25519.Basepoint)
	if err != nil {
		fatalf("public key: %v", err)
	}
	var wgPub [32]byte
	copy(wgPub[:], pubRaw)
	clientPub := base64.StdEncoding.EncodeToString(pubRaw)

	self, err := uuid.Parse(*deviceID)
	if err != nil {
		fatalf("device id: %v", err)
	}
	ip, err := netip.ParseAddr(*wgIP)
	if err != nil || !ip.Is4() {
		fatalf("wg ip: %v", err)
	}
	gw, err := netip.ParseAddr(*gateway)
	if err != nil {
		fatalf("gateway: %v", err)
	}

	rawTUN, err := tun.CreateTUN(*iface, *mtu)
	if err != nil {
		fatalf("CreateTUN: %v", err)
	}
	logger := device.NewLogger(device.LogLevelError, "["+*iface+"] ")
	pb := newPunchBind(conn.NewDefaultBind())
	disco := punch.DiscoPrivateFor(wgPriv)
	eng := punch.NewEngine(self, disco, wgPub, ip)
	it, err := wrapIntroTUN(rawTUN, func(dst netip.Addr) {
		eng.NoteTraffic(dst)
	})
	if err != nil {
		fatalf("intro tun: %v", err)
	}
	dev := device.NewDevice(it, pb, logger)
	privHex := hex.EncodeToString(wgPriv[:])
	serverHex, err := b64ToHex(*serverPubB64)
	if err != nil {
		fatalf("server pubkey: %v", err)
	}
	conf := fmt.Sprintf(
		"private_key=%s\npublic_key=%s\nendpoint=%s\npersistent_keepalive_interval=15\nallowed_ip=%s\n\n",
		privHex, serverHex, *endpoint, *allowed,
	)
	if err := dev.IpcSetOperation(bufio.NewReader(strings.NewReader(conf))); err != nil {
		fatalf("IpcSet: %v", err)
	}
	if err := dev.Up(); err != nil {
		fatalf("device Up: %v", err)
	}
	if err := run("ip", "address", "add", *wgIP+"/32", "dev", *iface); err != nil {
		fatalf("%v", err)
	}
	if err := run("ip", "link", "set", *iface, "up"); err != nil {
		fatalf("%v", err)
	}
	if err := run("ip", "route", "add", *allowed, "dev", *iface); err != nil {
		fatalf("%v", err)
	}
	fmt.Printf("punchpeer up: iface=%s ip=%s endpoint=%s allowed=%s\n", *iface, *wgIP, *endpoint, *allowed)

	api := &api{
		base:   strings.TrimRight(*controller, "/"),
		token:  *token,
		uid:    *uid,
		server: *serverID,
		pub:    clientPub,
		http: &http.Client{
			Timeout: 8 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
	inst := map[uuid.UUID]installed{}
	var instMu sync.Mutex
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	var lastAllow string
	var haveAllow bool
	var ready bool
	var complained bool
	for {
		select {
		case payload := <-it.intros:
			apply(dev, it, &instMu, inst, eng.OnPacket(netip.AddrPort{}, payload, time.Now()), *endpoint, pb)
		case pkt := <-pb.in:
			from, err := netip.ParseAddrPort(pkt.from)
			if err != nil {
				continue
			}
			apply(dev, it, &instMu, inst, eng.OnPacket(from, pkt.data, time.Now()), *endpoint, pb)
		case <-tick.C:
			allowAll, entries, text := readAllow(*allowPath)
			if !haveAllow || text != lastAllow {
				if err := announce(*wgIP, gw, allowAll, entries); err != nil {
					fmt.Printf("announce: %v\n", err)
				} else {
					fmt.Printf("announced allowall=%v entries=%v\n", allowAll, entries)
				}
				lastAllow = text
				haveAllow = true
			} else if time.Now().Unix()%2 == 0 {
				_ = announce(*wgIP, gw, allowAll, entries)
			}
			fwOff := false
			cfg, err := api.wgConfig()
			if err != nil {
				if !complained {
					fmt.Printf("wg config: %v\n", err)
					complained = true
				}
			} else if cfg.DiscoKey != "" {
				if key, err := punch.DecodeKey(cfg.DiscoKey); err == nil {
					eng.SetServerDisco(key)
					if !ready {
						fmt.Println("punch ready")
						ready = true
					}
				}
				fwOff = !cfg.EnableFirewall
			}
			apply(dev, it, &instMu, inst, eng.SetLocalPolicy(fwOff, allowAll, entries), *endpoint, pb)
			apply(dev, it, &instMu, inst, eng.Poll(time.Now()), *endpoint, pb)
		}
	}
}

type installed struct {
	wgPub [32]byte
	ip    netip.Addr
}

func apply(dev *device.Device, it *introTUN, mu *sync.Mutex, inst map[uuid.UUID]installed, res punch.Result, server string, pb *punchBind) {
	for _, d := range res.Sends {
		if d.Tunnel {
			msg, err := punch.ParseIntro(d.Data)
			if err != nil || !msg.WGIP.Is4() || !d.To.Addr().Is4() {
				continue
			}
			pkt, err := punch.WrapIPv4UDP(msg.WGIP, d.To.Addr(), punch.IntroPort, punch.IntroPort, d.Data)
			if err != nil {
				continue
			}
			it.queue(pkt)
			continue
		}
		to := d.To.String()
		if d.Server {
			to = server
		}
		if to == "" || len(d.Data) == 0 {
			continue
		}
		ep, err := pb.ParseEndpoint(to)
		if err != nil {
			continue
		}
		if err := pb.Send([][]byte{d.Data}, ep); err != nil {
			fmt.Printf("punch send %s: %v\n", to, err)
		}
	}
	for _, up := range res.Up {
		if !up.Peer.WGIP.Is4() || !up.Via.IsValid() {
			continue
		}
		conf := fmt.Sprintf(
			"public_key=%s\nendpoint=%s\nreplace_allowed_ips=true\nallowed_ip=%s/32\npersistent_keepalive_interval=10\n\n",
			hex.EncodeToString(up.Peer.WGPub[:]), up.Via.String(), up.Peer.WGIP.String(),
		)
		if err := dev.IpcSetOperation(bufio.NewReader(strings.NewReader(conf))); err != nil {
			fmt.Printf("punch install %s: %v\n", up.Peer.WGIP, err)
			continue
		}
		mu.Lock()
		inst[up.Peer.DeviceID] = installed{wgPub: up.Peer.WGPub, ip: up.Peer.WGIP}
		mu.Unlock()
		fmt.Printf("direct up %s via %s\n", up.Peer.WGIP, up.Via)
	}
	for _, id := range res.Down {
		mu.Lock()
		cur, ok := inst[id]
		mu.Unlock()
		if !ok {
			continue
		}
		conf := fmt.Sprintf("public_key=%s\nremove=true\n\n", hex.EncodeToString(cur.wgPub[:]))
		if err := dev.IpcSetOperation(bufio.NewReader(strings.NewReader(conf))); err != nil {
			fmt.Printf("punch remove %s: %v\n", cur.ip, err)
			continue
		}
		mu.Lock()
		delete(inst, id)
		mu.Unlock()
		fmt.Printf("direct down %s\n", cur.ip)
	}
}

func readAllow(path string) (allowAll bool, entries []string, text string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return false, nil, ""
	}
	text = string(b)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line == "*" || strings.EqualFold(line, "allowall") {
			allowAll = true
			continue
		}
		entries = append(entries, line)
	}
	return allowAll, entries, text
}

func announce(local string, gw netip.Addr, allowAll bool, entries []string) error {
	if entries == nil {
		entries = []string{}
	}
	body, err := json.Marshal(map[string]any{"AllowAll": allowAll, "Allowed": entries})
	if err != nil {
		return err
	}
	conn, err := net.DialUDP("udp4",
		&net.UDPAddr{IP: net.ParseIP(local)},
		&net.UDPAddr{IP: net.IP(gw.AsSlice()), Port: 51821},
	)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(time.Second))
	_, err = conn.Write(body)
	return err
}

type api struct {
	base, token, uid, server, pub string
	http                          *http.Client
}

func (a *api) wgConfig() (*wgServer, error) {
	u := a.base + "/client/wg/config?serverID=" + neturl.QueryEscape(a.server) + "&pubKey=" + neturl.QueryEscape(a.pub)
	body, code, err := a.do("GET", u, nil)
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("wg config %d: %s", code, body)
	}
	cfg := new(wgServer)
	if err := json.Unmarshal(body, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

type wgServer struct {
	EnableFirewall bool   `json:"EnableFirewall"`
	DiscoKey       string `json:"DiscoKey"`
}

func (a *api) do(method, url string, body any) ([]byte, int, error) {
	var r *strings.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		r = strings.NewReader(string(b))
	} else {
		r = strings.NewReader("")
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Device-Token", a.token)
	req.Header.Set("X-UID", a.uid)
	resp, err := a.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	buf := make([]byte, 0, 1024)
	tmp := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
		if len(buf) > 1<<20 {
			break
		}
	}
	return buf, resp.StatusCode, nil
}

type datagram struct {
	from string
	data []byte
}

type punchBind struct {
	inner conn.Bind
	in    chan datagram
}

func newPunchBind(inner conn.Bind) *punchBind {
	return &punchBind{inner: inner, in: make(chan datagram, 128)}
}

// Open ...
func (b *punchBind) Open(port uint16) ([]conn.ReceiveFunc, uint16, error) {
	fns, actual, err := b.inner.Open(port)
	if err != nil {
		return nil, 0, err
	}
	out := make([]conn.ReceiveFunc, len(fns))
	for i, fn := range fns {
		out[i] = b.wrap(fn)
	}
	return out, actual, nil
}

func (b *punchBind) wrap(inner conn.ReceiveFunc) conn.ReceiveFunc {
	return func(bufs [][]byte, sizes []int, eps []conn.Endpoint) (int, error) {
		n, err := inner(bufs, sizes, eps)
		if err != nil || n == 0 {
			return n, err
		}
		out := 0
		for i := 0; i < n; i++ {
			data := bufs[i][:sizes[i]]
			if punch.Is(data) {
				pkt := make([]byte, len(data))
				copy(pkt, data)
				from := ""
				if eps[i] != nil {
					from = eps[i].DstToString()
				}
				select {
				case b.in <- datagram{from: from, data: pkt}:
				default:
				}
				continue
			}
			if out != i {
				copy(bufs[out][:sizes[i]], data)
				sizes[out] = sizes[i]
				eps[out] = eps[i]
			}
			out++
		}
		return out, nil
	}
}

// Close ...
func (b *punchBind) Close() error { return b.inner.Close() }

// SetMark ...
func (b *punchBind) SetMark(mark uint32) error { return b.inner.SetMark(mark) }

// Send ...
func (b *punchBind) Send(bufs [][]byte, ep conn.Endpoint) error {
	return b.inner.Send(bufs, ep)
}

// ParseEndpoint ...
func (b *punchBind) ParseEndpoint(s string) (conn.Endpoint, error) {
	return b.inner.ParseEndpoint(s)
}

// BatchSize ...
func (b *punchBind) BatchSize() int { return b.inner.BatchSize() }

func b64ToHex(s string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("expected 32 bytes, got %d", len(raw))
	}
	return hex.EncodeToString(raw), nil
}

func run(args ...string) error {
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %w: %s", args, err, out)
	}
	return nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
