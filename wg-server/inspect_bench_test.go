package wgserver

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"testing"

	"golang.zx2c4.com/wireguard/tun"
)

type benchDevice struct{}

func (benchDevice) File() *os.File                          { return nil }
func (benchDevice) Read([][]byte, []int, int) (int, error)  { return 0, nil }
func (benchDevice) Write(bufs [][]byte, _ int) (int, error) { return len(bufs), nil }
func (benchDevice) MTU() (int, error)                       { return benchMTU, nil }
func (benchDevice) Name() (string, error)                   { return "bench", nil }
func (benchDevice) Events() <-chan tun.Event                { return nil }
func (benchDevice) Close() error                            { return nil }
func (benchDevice) BatchSize() int                          { return benchBatch }

const (
	benchMTU   = 1420
	benchBatch = 128
)

func mustIP(s string) netip.Addr {
	a, err := netip.ParseAddr(s)
	if err != nil {
		panic(err)
	}
	return a
}

func ipv4UDP(src, dst string, sport, dport uint16, size int) []byte {
	if size < 28 {
		size = 28
	}
	s := mustIP(src).As4()
	d := mustIP(dst).As4()
	pkt := make([]byte, size)
	pkt[0] = 0x45
	binary.BigEndian.PutUint16(pkt[2:4], uint16(size))
	pkt[9] = protoUDP
	copy(pkt[12:16], s[:])
	copy(pkt[16:20], d[:])
	udp := pkt[20:]
	binary.BigEndian.PutUint16(udp[0:2], sport)
	binary.BigEndian.PutUint16(udp[2:4], dport)
	binary.BigEndian.PutUint16(udp[4:6], uint16(size-20))
	return pkt
}

func rules(ss ...string) []aclEntry {
	e := make([]aclEntry, 0, len(ss))
	for _, s := range ss {
		a, ok := parseACLEntry(s)
		if !ok {
			panic("bad bench rule: " + s)
		}
		e = append(e, a)
	}
	return e
}

func newBenchInspector(firewall bool) *inspectingTUN {
	if err := initPeerList("10.0.0.0/24", "fd00::/64"); err != nil {
		panic(err)
	}
	insp, err := newInspectingTUN(benchDevice{}, &Config{
		WireGuardSubnet:  "10.0.0.0/24",
		WireGuardSubnet6: "fd00::/64",
		EnableFirewall:   firewall,
	})
	if err != nil {
		panic(err)
	}
	return insp
}

func setRule(dst string, tokens ...string) {
	resetPeer(dst)
	p, ok := fwClassify(mustIP(dst))
	if !ok || p == nil {
		panic("no peer entry for " + dst)
	}
	p.setAllowed(rules(tokens...), false)
}

func batchUnfrag(src, dst string, dport uint16, size int) [][]byte {
	b := make([][]byte, benchBatch)
	for i := range b {
		b[i] = ipv4UDP(src, dst, uint16(20000+i), dport, size)
	}
	return b
}

func buildConntrack(size int) (*inspectingTUN, [][]byte) {
	insp := newBenchInspector(true)
	const R, S = "10.0.0.10", "10.99.0.5"
	resetPeer(R)
	for i := 0; i < benchBatch; i++ {
		insp.allow(ipv4UDP(R, S, uint16(40000+i), 443, 28))
	}
	master := make([][]byte, benchBatch)
	for i := range master {
		master[i] = ipv4UDP(S, R, 443, uint16(40000+i), size)
	}
	return insp, master
}

const (
	benchFragDatagrams = 16
	benchFragsPer      = benchBatch / benchFragDatagrams
)

func buildFrag(rule string, size int) (*inspectingTUN, [][]byte) {
	insp := newBenchInspector(true)
	const R, S = "10.0.0.10", "10.0.0.5"
	setRule(R, rule)

	master := make([][]byte, 0, benchBatch)
	for d := 0; d < benchFragDatagrams; d++ {
		id := uint16(1000 + d)
		head := ipv4UDP(S, R, uint16(30000+d), 5000, size)
		setFrag(head, id, 0, true)
		master = append(master, head)
		for f := 1; f < benchFragsPer; f++ {
			tail := ipv4UDP(S, R, 0, 0, size)
			setFrag(tail, id, uint16(185*f), f < benchFragsPer-1)
			master = append(master, tail)
		}
	}
	return insp, master
}

type benchScenario struct {
	name  string
	build func() (*inspectingTUN, [][]byte)
}

func benchScenarios() []benchScenario {
	var out []benchScenario
	for _, sz := range []int{benchMTU, 128} {
		sz := sz
		out = append(out,
			benchScenario{fmt.Sprintf("unfrag-barehost/%d", sz), func() (*inspectingTUN, [][]byte) {
				insp := newBenchInspector(true)
				setRule("10.0.0.10", "10.0.0.5")
				return insp, batchUnfrag("10.0.0.5", "10.0.0.10", 5000, sz)
			}},
			benchScenario{fmt.Sprintf("unfrag-portrule/%d", sz), func() (*inspectingTUN, [][]byte) {
				insp := newBenchInspector(true)
				setRule("10.0.0.10", "10.0.0.5:5000")
				return insp, batchUnfrag("10.0.0.5", "10.0.0.10", 5000, sz)
			}},
			benchScenario{fmt.Sprintf("unfrag-anyport/%d", sz), func() (*inspectingTUN, [][]byte) {
				insp := newBenchInspector(true)
				setRule("10.0.0.10", "*:5000")
				return insp, batchUnfrag("10.0.0.5", "10.0.0.10", 5000, sz)
			}},
			benchScenario{fmt.Sprintf("unfrag-conntrack/%d", sz), func() (*inspectingTUN, [][]byte) {
				return buildConntrack(sz)
			}},
			benchScenario{fmt.Sprintf("unfrag-denied/%d", sz), func() (*inspectingTUN, [][]byte) {
				insp := newBenchInspector(true)
				resetPeer("10.0.0.10")
				return insp, batchUnfrag("10.0.0.5", "10.0.0.10", 5000, sz)
			}},
			benchScenario{fmt.Sprintf("frag-barehost/%d", sz), func() (*inspectingTUN, [][]byte) {
				return buildFrag("10.0.0.5", sz)
			}},
			benchScenario{fmt.Sprintf("frag-portrule/%d", sz), func() (*inspectingTUN, [][]byte) {
				return buildFrag("10.0.0.5:5000", sz)
			}},
			benchScenario{fmt.Sprintf("firewall-off/%d", sz), func() (*inspectingTUN, [][]byte) {
				insp := newBenchInspector(false)
				resetPeer("10.0.0.5")
				return insp, batchUnfrag("10.0.0.5", "10.0.0.10", 5000, sz)
			}},
		)
	}
	return out
}

func batchBytes(master [][]byte) int64 {
	var n int64
	for _, p := range master {
		n += int64(len(p))
	}
	return n
}

func BenchmarkInspectWrite(b *testing.B) {
	for _, sc := range benchScenarios() {
		b.Run(sc.name, func(b *testing.B) {
			insp, master := sc.build()
			scratch := make([][]byte, len(master))
			b.SetBytes(batchBytes(master))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {

				copy(scratch, master)
				insp.Write(scratch, 0)
			}
		})
	}
}

func BenchmarkInspectWriteParallel(b *testing.B) {
	for _, sc := range benchScenarios() {
		b.Run(sc.name, func(b *testing.B) {
			insp, master := sc.build()
			b.SetBytes(batchBytes(master))
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				scratch := make([][]byte, len(master))
				for pb.Next() {
					copy(scratch, master)
					insp.Write(scratch, 0)
				}
			})
		})
	}
}

func TestBenchScenariosAdmitAsExpected(t *testing.T) {
	for _, sc := range benchScenarios() {
		insp, master := sc.build()
		scratch := make([][]byte, len(master))
		copy(scratch, master)
		n, err := insp.Write(scratch, 0)
		if err != nil {
			t.Fatalf("%s: Write error: %v", sc.name, err)
		}
		want := len(master)
		if strings.Contains(sc.name, "denied") {
			want = 0
		}
		if n != want {
			t.Fatalf("%s: forwarded %d/%d packets, want %d — benchmark would measure the wrong path",
				sc.name, n, len(master), want)
		}
	}
}

func TestBuildFragShape(t *testing.T) {
	_, master := buildFrag("10.0.0.5:5000", benchMTU)
	if len(master) != benchBatch {
		t.Fatalf("batch size %d, want %d", len(master), benchBatch)
	}
	var heads, trailing int
	for _, pkt := range master {
		_, _, _, _, frag, ok := parseIPHeader(pkt)
		if !ok || !frag.isFragment() {
			t.Fatalf("every packet must be a fragment; got ok=%v isFragment=%v", ok, frag.isFragment())
		}
		if frag.isTrailing() {
			trailing++
		} else {
			heads++
		}
	}
	if heads != benchFragDatagrams {
		t.Fatalf("got %d heads, want %d (one per datagram)", heads, benchFragDatagrams)
	}
	if wantTrailing := benchFragDatagrams * (benchFragsPer - 1); trailing != wantTrailing {
		t.Fatalf("got %d trailing fragments, want %d", trailing, wantTrailing)
	}
}
