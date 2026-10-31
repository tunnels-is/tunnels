package types

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

// Mesh subnet limits (security finding A-14).
//
// A mesh peer's AllowedSubnets are installed on the hub's mesh WireGuard
// device with replace_allowed_ips=true and as kernel routes, so an overly
// broad prefix lets a misconfigured or compromised controller pull
// arbitrary traffic into the mesh. These limits bound how broad a single
// mesh subnet may be. Narrower (longer) prefixes are always allowed.
const (
	// MeshMinPrefixLenV4 is the broadest IPv4 mesh subnet allowed (/8).
	MeshMinPrefixLenV4 = 8
	// MeshMinPrefixLenV6 is the broadest IPv6 mesh subnet allowed (/32).
	MeshMinPrefixLenV6 = 32
)

// ValidateMeshSubnet is the single validation rule for a mesh subnet, used
// by the controller (when a server joins a mesh group and when building the
// /wg/mesh response) and by wg-server (before installing a mesh peer).
//
// The subnet must:
//   - parse as a CIDR prefix (no zone, no surrounding whitespace),
//   - be in canonical form (no host bits set: 10.0.0.0/16, not 10.0.0.1/16),
//   - not be an IPv4-mapped IPv6 prefix (::ffff:a.b.c.d/n),
//   - not be broader than MeshMinPrefixLenV4 / MeshMinPrefixLenV6 (so /0 is
//     always rejected),
//   - not start at an unspecified, loopback, link-local or multicast
//     address.
func ValidateMeshSubnet(cidr string) (netip.Prefix, error) {
	if cidr == "" {
		return netip.Prefix{}, errors.New("empty mesh subnet")
	}
	if strings.TrimSpace(cidr) != cidr {
		return netip.Prefix{}, fmt.Errorf("mesh subnet %q has surrounding whitespace", cidr)
	}
	p, err := netip.ParsePrefix(cidr)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("invalid mesh subnet %q: %w", cidr, err)
	}
	addr := p.Addr()
	if addr.Is4In6() {
		return netip.Prefix{}, fmt.Errorf("mesh subnet %q is an IPv4-mapped IPv6 prefix", cidr)
	}
	if p.Masked() != p {
		return netip.Prefix{}, fmt.Errorf("mesh subnet %q has host bits set (want %s)", cidr, p.Masked())
	}
	minLen := MeshMinPrefixLenV6
	if addr.Is4() {
		minLen = MeshMinPrefixLenV4
	}
	if p.Bits() < minLen {
		return netip.Prefix{}, fmt.Errorf("mesh subnet %q is broader than /%d", cidr, minLen)
	}
	switch {
	case addr.IsUnspecified():
		return netip.Prefix{}, fmt.Errorf("mesh subnet %q starts at the unspecified address", cidr)
	case addr.IsLoopback():
		return netip.Prefix{}, fmt.Errorf("mesh subnet %q is loopback", cidr)
	case addr.IsLinkLocalUnicast():
		return netip.Prefix{}, fmt.Errorf("mesh subnet %q is link-local", cidr)
	case addr.IsMulticast():
		return netip.Prefix{}, fmt.Errorf("mesh subnet %q is multicast", cidr)
	}
	return p, nil
}

// MeshSubnetsOverlap reports whether two prefixes share any address. Invalid
// prefixes and prefixes of different address families never overlap.
func MeshSubnetsOverlap(a, b netip.Prefix) bool {
	if !a.IsValid() || !b.IsValid() {
		return false
	}
	return a.Masked().Overlaps(b.Masked())
}

// ParseLocalSubnet leniently parses a server's own configured subnet (which
// may legitimately be written in gateway form such as 10.0.0.1/22) for use
// in overlap checks. It returns the masked prefix, or false if s is empty or
// unparseable. It is NOT a substitute for ValidateMeshSubnet.
func ParseLocalSubnet(s string) (netip.Prefix, bool) {
	if s == "" {
		return netip.Prefix{}, false
	}
	p, err := netip.ParsePrefix(s)
	if err != nil {
		return netip.Prefix{}, false
	}
	return p.Masked(), true
}

// MeshPeerRejection describes a mesh peer, or one subnet of a peer, that
// ValidateMeshPeers refused. Subnet is empty when the whole peer was
// rejected.
type MeshPeerRejection struct {
	PublicKeyHex string
	Subnet       string
	Err          error
}

// ValidateMeshPeers applies the shared mesh rules to a full peer list as
// seen by one hub whose own WireGuard subnets are ownSubnets (lenient
// parse; empty/unparseable entries are ignored).
//
// Rules, in order:
//  1. A peer with an empty PublicKeyHex is rejected.
//  2. A PublicKeyHex that appears more than once is ambiguous; every entry
//     for it is rejected.
//  3. Each subnet must pass ValidateMeshSubnet; exact duplicates within one
//     peer are collapsed.
//  4. A subnet overlapping any of ownSubnets is rejected.
//  5. A subnet overlapping a subnet claimed by a DIFFERENT peer is rejected
//     for BOTH peers (fail closed, independent of response order or key
//     sort order: an overlap means the controller is misconfigured or
//     compromised, and neither claim can be trusted).
//  6. A peer left with no subnets is rejected.
//
// The returned peers preserve input order and carry only the accepted
// subnets. The input slice is not modified.
func ValidateMeshPeers(ownSubnets []string, peers []WGMeshPeer) ([]WGMeshPeer, []MeshPeerRejection) {
	var rejected []MeshPeerRejection

	own := make([]netip.Prefix, 0, len(ownSubnets))
	for _, s := range ownSubnets {
		if p, ok := ParseLocalSubnet(s); ok {
			own = append(own, p)
		}
	}

	keyCount := make(map[string]int, len(peers))
	for _, p := range peers {
		keyCount[p.PublicKeyHex]++
	}

	type cand struct {
		peer     WGMeshPeer
		prefixes []netip.Prefix
	}
	cands := make([]cand, 0, len(peers))
	for _, p := range peers {
		if p.PublicKeyHex == "" {
			rejected = append(rejected, MeshPeerRejection{Err: errors.New("empty public key")})
			continue
		}
		if keyCount[p.PublicKeyHex] > 1 {
			rejected = append(rejected, MeshPeerRejection{PublicKeyHex: p.PublicKeyHex, Err: errors.New("duplicate public key in mesh peer list")})
			continue
		}
		c := cand{peer: p}
		seen := make(map[netip.Prefix]bool, len(p.AllowedSubnets))
		for _, s := range p.AllowedSubnets {
			pfx, err := ValidateMeshSubnet(s)
			if err != nil {
				rejected = append(rejected, MeshPeerRejection{PublicKeyHex: p.PublicKeyHex, Subnet: s, Err: err})
				continue
			}
			if seen[pfx] {
				continue
			}
			seen[pfx] = true
			var ownHit netip.Prefix
			for _, o := range own {
				if MeshSubnetsOverlap(pfx, o) {
					ownHit = o
					break
				}
			}
			if ownHit.IsValid() {
				rejected = append(rejected, MeshPeerRejection{PublicKeyHex: p.PublicKeyHex, Subnet: s, Err: fmt.Errorf("overlaps local subnet %s", ownHit)})
				continue
			}
			c.prefixes = append(c.prefixes, pfx)
		}
		cands = append(cands, c)
	}

	// Cross-peer overlap: mark conflicting subnets on both sides.
	conflict := make([][]string, len(cands))
	for i := range cands {
		conflict[i] = make([]string, len(cands[i].prefixes))
	}
	for i := range cands {
		for j := i + 1; j < len(cands); j++ {
			for a, pa := range cands[i].prefixes {
				for b, pb := range cands[j].prefixes {
					if !MeshSubnetsOverlap(pa, pb) {
						continue
					}
					if conflict[i][a] == "" {
						conflict[i][a] = cands[j].peer.PublicKeyHex
					}
					if conflict[j][b] == "" {
						conflict[j][b] = cands[i].peer.PublicKeyHex
					}
				}
			}
		}
	}

	out := make([]WGMeshPeer, 0, len(cands))
	for i, c := range cands {
		subnets := make([]string, 0, len(c.prefixes))
		for a, pfx := range c.prefixes {
			if other := conflict[i][a]; other != "" {
				rejected = append(rejected, MeshPeerRejection{PublicKeyHex: c.peer.PublicKeyHex, Subnet: pfx.String(), Err: fmt.Errorf("overlaps subnet of mesh peer %s", other)})
				continue
			}
			subnets = append(subnets, pfx.String())
		}
		if len(subnets) == 0 {
			rejected = append(rejected, MeshPeerRejection{PublicKeyHex: c.peer.PublicKeyHex, Err: errors.New("no valid subnets")})
			continue
		}
		p := c.peer
		p.AllowedSubnets = subnets
		out = append(out, p)
	}
	return out, rejected
}
