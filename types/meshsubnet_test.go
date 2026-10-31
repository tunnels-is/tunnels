package types

import (
	"net/netip"
	"reflect"
	"testing"
)

func TestValidateMeshSubnet(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"10.3.0.0/16", true},
		{"10.4.0.5/32", true},
		{"10.0.0.0/8", true},
		{"100.64.0.0/10", true},
		{"fd00::/64", true},
		{"fd00:1234::/32", true},
		{"2001:db8::/48", true},
		{"FD00::/64", true}, // case-insensitive parse; canonical bits

		{"", false},
		{" 10.3.0.0/16", false},
		{"10.3.0.0/16 ", false},
		{"not-a-cidr", false},
		{"10.0.0.1", false},
		{"10.0.0.0/33", false},
		{"0.0.0.0/0", false},
		{"::/0", false},
		{"10.0.0.0/7", false},          // broader than /8
		{"0.0.0.0/1", false},           // half the internet
		{"128.0.0.0/1", false},         // other half
		{"fd00::/31", false},           // broader than /32
		{"2000::/3", false},            // all global unicast
		{"10.0.0.1/16", false},         // host bits
		{"fd00::1/64", false},          // host bits
		{"0.0.0.0/8", false},           // unspecified
		{"::/32", false},               // unspecified
		{"127.0.0.0/8", false},         // loopback
		{"::1/128", false},             // loopback
		{"169.254.0.0/16", false},      // link-local
		{"fe80::/64", false},           // link-local
		{"224.0.0.0/8", false},         // multicast
		{"ff02::/32", false},           // multicast
		{"::ffff:10.0.0.0/104", false}, // IPv4-mapped
		{"fe80::%eth0/64", false},      // zone
	}
	for _, c := range cases {
		p, err := ValidateMeshSubnet(c.in)
		if (err == nil) != c.ok {
			t.Errorf("ValidateMeshSubnet(%q) err=%v, want ok=%v", c.in, err, c.ok)
			continue
		}
		if c.ok && p != p.Masked() {
			t.Errorf("ValidateMeshSubnet(%q) returned non-canonical %s", c.in, p)
		}
	}
}

func TestMeshSubnetsOverlap(t *testing.T) {
	mp := netip.MustParsePrefix
	cases := []struct {
		a, b netip.Prefix
		want bool
	}{
		{mp("10.3.0.0/16"), mp("10.3.1.0/24"), true},
		{mp("10.3.1.0/24"), mp("10.3.0.0/16"), true},
		{mp("10.3.0.0/16"), mp("10.3.0.0/16"), true},
		{mp("10.3.0.0/16"), mp("10.4.0.0/16"), false},
		{mp("10.0.0.0/22"), mp("10.0.4.0/22"), false},
		{mp("10.0.0.5/22"), mp("10.0.3.0/24"), true}, // unmasked input is masked
		{mp("fd00::/64"), mp("fd00::/48"), true},
		{mp("fd00::/64"), mp("fd00:0:0:1::/64"), false},
		{mp("10.0.0.0/8"), mp("fd00::/64"), false}, // different families
		{netip.Prefix{}, mp("10.0.0.0/8"), false},
	}
	for _, c := range cases {
		if got := MeshSubnetsOverlap(c.a, c.b); got != c.want {
			t.Errorf("MeshSubnetsOverlap(%s, %s) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestParseLocalSubnet(t *testing.T) {
	if p, ok := ParseLocalSubnet("10.0.0.1/22"); !ok || p != netip.MustParsePrefix("10.0.0.0/22") {
		t.Fatalf("gateway-form local subnet: got %s %v", p, ok)
	}
	for _, s := range []string{"", "junk"} {
		if _, ok := ParseLocalSubnet(s); ok {
			t.Fatalf("ParseLocalSubnet(%q) should fail", s)
		}
	}
}

func TestValidateMeshPeers(t *testing.T) {
	peer := func(k string, subnets ...string) WGMeshPeer {
		return WGMeshPeer{PublicKeyHex: k, Endpoint: "1.1.1.1:51821", AllowedSubnets: subnets}
	}
	keys := func(ps []WGMeshPeer) map[string][]string {
		m := map[string][]string{}
		for _, p := range ps {
			m[p.PublicKeyHex] = p.AllowedSubnets
		}
		return m
	}
	own := []string{"10.0.0.1/22", "fd00::/64"}

	cases := []struct {
		name  string
		peers []WGMeshPeer
		want  map[string][]string
	}{
		{
			name:  "valid peers pass through",
			peers: []WGMeshPeer{peer("a", "10.3.0.0/16", "fd00:3::/64"), peer("b", "10.4.0.0/16")},
			want:  map[string][]string{"a": {"10.3.0.0/16", "fd00:3::/64"}, "b": {"10.4.0.0/16"}},
		},
		{
			name:  "invalid subnet dropped, rest kept",
			peers: []WGMeshPeer{peer("a", "0.0.0.0/0", "10.3.0.0/16", "10.9.0.1/16")},
			want:  map[string][]string{"a": {"10.3.0.0/16"}},
		},
		{
			name:  "peer with only invalid subnets dropped",
			peers: []WGMeshPeer{peer("a", "0.0.0.0/1", "::/0")},
			want:  map[string][]string{},
		},
		{
			name:  "overlap with own v4 subnet rejected",
			peers: []WGMeshPeer{peer("a", "10.0.0.0/16"), peer("b", "10.0.2.0/24", "10.5.0.0/16")},
			want:  map[string][]string{"b": {"10.5.0.0/16"}},
		},
		{
			name:  "overlap with own v6 subnet rejected",
			peers: []WGMeshPeer{peer("a", "10.3.0.0/16", "fd00::/48")},
			want:  map[string][]string{"a": {"10.3.0.0/16"}},
		},
		{
			name:  "cross-peer overlap rejects both claims",
			peers: []WGMeshPeer{peer("a", "10.3.0.0/16", "10.6.0.0/16"), peer("b", "10.3.1.0/24"), peer("c", "10.7.0.0/16")},
			want:  map[string][]string{"a": {"10.6.0.0/16"}, "c": {"10.7.0.0/16"}},
		},
		{
			name:  "cross-peer overlap is order independent",
			peers: []WGMeshPeer{peer("b", "10.3.1.0/24"), peer("a", "10.3.0.0/16", "10.6.0.0/16"), peer("c", "10.7.0.0/16")},
			want:  map[string][]string{"a": {"10.6.0.0/16"}, "c": {"10.7.0.0/16"}},
		},
		{
			name:  "duplicate subnet within one peer collapsed",
			peers: []WGMeshPeer{peer("a", "10.3.0.0/16", "10.3.0.0/16")},
			want:  map[string][]string{"a": {"10.3.0.0/16"}},
		},
		{
			name:  "duplicate key rejects all entries",
			peers: []WGMeshPeer{peer("a", "10.3.0.0/16"), peer("a", "10.4.0.0/16"), peer("b", "10.5.0.0/16")},
			want:  map[string][]string{"b": {"10.5.0.0/16"}},
		},
		{
			name:  "empty key rejected",
			peers: []WGMeshPeer{peer("", "10.3.0.0/16")},
			want:  map[string][]string{},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, rej := ValidateMeshPeers(own, c.peers)
			if gk := keys(got); !reflect.DeepEqual(gk, c.want) {
				t.Fatalf("got %v, want %v (rejections: %+v)", gk, c.want, rej)
			}
			if len(got) != len(c.peers) && len(rej) == 0 {
				t.Fatal("dropped peers must produce rejections")
			}
		})
	}

	// input must not be mutated
	in := []WGMeshPeer{peer("a", "0.0.0.0/0", "10.3.0.0/16")}
	_, _ = ValidateMeshPeers(nil, in)
	if len(in[0].AllowedSubnets) != 2 {
		t.Fatal("ValidateMeshPeers mutated its input")
	}
}
