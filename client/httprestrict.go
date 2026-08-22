package client

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
)

var lookupIPFunc = net.LookupIP

func checkPublicHTTPSRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return fmt.Errorf("too many redirects")
	}
	if req == nil || req.URL == nil {
		return fmt.Errorf("redirect missing URL")
	}
	return requirePublicHTTPSURL(req.URL)
}

func requirePublicHTTPSURL(u *url.URL) error {
	if u == nil {
		return fmt.Errorf("missing URL")
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return fmt.Errorf("refusing non-https URL")
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return fmt.Errorf("URL missing host")
	}
	if isNonPublicHostname(host) {
		return fmt.Errorf("refusing non-public host %q", host)
	}
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicIP(ip) {
			return fmt.Errorf("refusing non-public address %s", ip)
		}
		return nil
	}
	addrs, err := lookupIPFunc(host)
	if err != nil {
		return fmt.Errorf("lookup %s: %w", host, err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("lookup %s: no addresses", host)
	}
	for _, a := range addrs {
		if !isPublicIP(a) {
			return fmt.Errorf("refusing host %s: resolved to non-public address %s", host, a)
		}
	}
	return nil
}

func isNonPublicHostname(host string) bool {
	h := strings.ToLower(host)
	switch h {
	case "localhost", "localhost.localdomain":
		return true
	}
	if strings.HasSuffix(h, ".localhost") || strings.HasSuffix(h, ".local") || strings.HasSuffix(h, ".internal") {
		return true
	}
	return false
}

func isPublicIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil {
		// RFC 6598 CGNAT 100.64.0.0/10
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return false
		}
		// IPv4 link-local (also covered by IsLinkLocalUnicast on some platforms)
		if ip4[0] == 169 && ip4[1] == 254 {
			return false
		}
	}
	return true
}
