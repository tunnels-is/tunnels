package wgserver

import (
	"net"
	"sort"
	"strings"

	"github.com/vishvananda/netlink"
)

// RFC ranges that are never a default-route internet destination.
// Link-local (IMDS) often has no local address and no more-specific
// route, so scraping ip addr/route cannot see it.
var rfcNeverInternetV4 = []string{
	"169.254.0.0/16",
	"127.0.0.0/8",
}

var rfcNeverInternetV6 = []string{
	"fe80::/10",
	"::1/128",
}

// Hypervisor fabric anycast: these are intercepted on the default
// interface but look like ordinary unicast via the default gateway
// (no local address, often no extra route). Routing cannot express them.
var fabricAnycastV4 = []string{
	"168.63.129.16/32",   // Azure wireserver
	"100.100.100.200/32", // Alibaba metadata
}

var fabricAnycastV6 = []string{
	"fd00:ec2::254/128", // AWS IPv6 IMDS
}

// attachedPrefix is one address or non-default route on the host.
type attachedPrefix struct {
	IPNet    *net.IPNet
	LinkName string
}

func skipOverlayIfaces(cfg *Config) []string {
	names := []string{cfg.WireGuardIface}
	if cfg.WireGuardMeshPort > 0 && cfg.WireGuardIface != "" {
		names = append(names, meshIface(cfg))
	}
	return names
}

func overlayNets(cfg *Config) []*net.IPNet {
	var out []*net.IPNet
	for _, s := range []string{cfg.WireGuardSubnet, cfg.WireGuardSubnet6} {
		if s == "" {
			continue
		}
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			continue
		}
		out = append(out, n)
	}
	return out
}

// egressDropCIDRs is the destination set dropped on the internet-iface hop:
// RFC never-internet + hypervisor anycast + every non-overlay prefix on this box.
func egressDropCIDRs(cfg *Config) (v4, v6 []string, err error) {
	v4 = mergeCIDRs(rfcNeverInternetV4, fabricAnycastV4)
	v6 = mergeCIDRs(rfcNeverInternetV6, fabricAnycastV6)
	host4, host6, herr := collectHostDropCIDRs(skipOverlayIfaces(cfg), overlayNets(cfg))
	if herr != nil {
		return v4, v6, herr
	}
	return mergeCIDRs(v4, host4), mergeCIDRs(v6, host6), nil
}

func collectHostDropCIDRs(skipIfaces []string, overlay []*net.IPNet) (v4, v6 []string, err error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, nil, err
	}
	indexName := make(map[int]string, len(links))
	var attached []attachedPrefix
	for _, l := range links {
		attrs := l.Attrs()
		if attrs == nil {
			continue
		}
		indexName[attrs.Index] = attrs.Name
		addrs, aerr := netlink.AddrList(l, netlink.FAMILY_ALL)
		if aerr != nil {
			continue
		}
		for i := range addrs {
			a := &addrs[i]
			if a.IPNet != nil {
				attached = append(attached, attachedPrefix{IPNet: a.IPNet, LinkName: attrs.Name})
			}
			if a.Peer != nil {
				attached = append(attached, attachedPrefix{IPNet: a.Peer, LinkName: attrs.Name})
			}
		}
	}
	for _, family := range []int{netlink.FAMILY_V4, netlink.FAMILY_V6} {
		routes, rerr := netlink.RouteList(nil, family)
		if rerr != nil {
			return nil, nil, rerr
		}
		for i := range routes {
			r := &routes[i]
			if isDefaultNet(r.Dst) {
				continue
			}
			name := indexName[r.LinkIndex]
			attached = append(attached, attachedPrefix{IPNet: r.Dst, LinkName: name})
		}
	}
	v4, v6 = dropCIDRsFromAttached(attached, skipIfaces, overlay)
	return v4, v6, nil
}

func dropCIDRsFromAttached(attached []attachedPrefix, skipIfaces []string, overlay []*net.IPNet) (v4, v6 []string) {
	skip := make(map[string]bool, len(skipIfaces))
	for _, n := range skipIfaces {
		if n != "" {
			skip[n] = true
		}
	}
	var nets []*net.IPNet
	for _, a := range attached {
		if a.IPNet == nil || a.IPNet.IP == nil {
			continue
		}
		if a.LinkName != "" && skip[a.LinkName] {
			continue
		}
		n := canonicalizeNet(a.IPNet)
		if n == nil || isDefaultNet(n) {
			continue
		}
		if prefixInOverlay(n, overlay) {
			continue
		}
		nets = append(nets, n)
	}
	return splitFamilyCIDRs(nets)
}

func isDefaultNet(n *net.IPNet) bool {
	if n == nil || n.IP == nil || n.Mask == nil {
		return true
	}
	ones, bits := n.Mask.Size()
	return bits > 0 && ones == 0
}

func prefixInOverlay(n *net.IPNet, overlay []*net.IPNet) bool {
	if n == nil {
		return false
	}
	nOnes, nBits := n.Mask.Size()
	for _, o := range overlay {
		if o == nil || o.IP == nil {
			continue
		}
		oOnes, oBits := o.Mask.Size()
		if nBits != oBits {
			continue
		}
		if o.Contains(n.IP) && nOnes >= oOnes {
			return true
		}
	}
	return false
}

func canonicalizeNet(n *net.IPNet) *net.IPNet {
	if n == nil || n.IP == nil || n.Mask == nil {
		return nil
	}
	ip := n.IP
	mask := n.Mask
	if v4 := ip.To4(); v4 != nil {
		ip = v4
		if len(mask) == net.IPv6len {
			mask = mask[12:]
		}
		if len(mask) != net.IPv4len {
			return nil
		}
	} else {
		ip = ip.To16()
		if ip == nil || len(mask) != net.IPv6len {
			return nil
		}
	}
	ones, bits := mask.Size()
	if bits == 0 || ones < 0 {
		return nil
	}
	return &net.IPNet{IP: ip.Mask(mask), Mask: mask}
}

func splitFamilyCIDRs(nets []*net.IPNet) (v4, v6 []string) {
	seen4 := map[string]struct{}{}
	seen6 := map[string]struct{}{}
	for _, n := range nets {
		if n == nil {
			continue
		}
		s := n.String()
		if n.IP.To4() != nil {
			if _, ok := seen4[s]; ok {
				continue
			}
			seen4[s] = struct{}{}
			v4 = append(v4, s)
		} else {
			if _, ok := seen6[s]; ok {
				continue
			}
			seen6[s] = struct{}{}
			v6 = append(v6, s)
		}
	}
	sort.Strings(v4)
	sort.Strings(v6)
	return v4, v6
}

func mergeCIDRs(base, extra []string) []string {
	parsed := make([]*net.IPNet, 0, len(base)+len(extra))
	seen := map[string]struct{}{}
	for _, s := range append(append([]string{}, base...), extra...) {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			continue
		}
		n = canonicalizeNet(n)
		if n == nil || isDefaultNet(n) {
			continue
		}
		key := n.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		parsed = append(parsed, n)
	}
	keep := make([]*net.IPNet, 0, len(parsed))
	for i, a := range parsed {
		covered := false
		for j, b := range parsed {
			if i == j {
				continue
			}
			if netContainsStrict(b, a) {
				covered = true
				break
			}
		}
		if !covered {
			keep = append(keep, a)
		}
	}
	out := make([]string, 0, len(keep))
	for _, n := range keep {
		out = append(out, n.String())
	}
	sort.Strings(out)
	return out
}

func netContainsStrict(outer, inner *net.IPNet) bool {
	if outer == nil || inner == nil {
		return false
	}
	oOnes, oBits := outer.Mask.Size()
	iOnes, iBits := inner.Mask.Size()
	if oBits != iBits {
		return false
	}
	return outer.Contains(inner.IP) && oOnes < iOnes
}

func destDropArgs(chain, netIface, cidr string) []string {
	return []string{"-A", chain, "-o", netIface, "-d", cidr, "-j", "DROP"}
}
