package core

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/miekg/dns"
)

func CreateConnectionUUID() string {
	return "{" + strings.ToUpper(uuid.NewString()) + "}"
}

func IsAlphanumeric(s string) bool {
	matched, _ := regexp.MatchString(`^[a-z0-9]+$`, s)
	return matched
}

func CreateConfig(flag *bool) {
	C.DebugLogging = true
	if *flag {
		InitPaths()
		CreateBaseFolder()
		LoadConfig()
		os.Exit(1)
	}
}

func InitPaths() {
	GLOBAL_STATE.BasePath = GenerateBaseFolderPath()
}

const (
	CODE_ConnectToNode = 152
)

func IsDefaultConnection(IFName string) bool {
	return strings.EqualFold(IFName, DefaultTunnelName)
}

func RecoverAndLogToFile() {
	if r := recover(); r != nil {
		ERROR(r, string(debug.Stack()))
	}
}

func CopySlice(in []byte) (out []byte) {
	out = make([]byte, len(in))
	_ = copy(out, in)
	return
}

func GetDomainAndSubDomain(domain string) (d, s string) {
	parts := strings.Split(domain, ".")
	// parts = parts[:len(parts)-1]
	if len(parts) == 2 {
		d = strings.Join(parts[len(parts)-2:], ".")
	} else if len(parts) > 2 {
		d = strings.Join(parts[len(parts)-3:], ".")
		s = strings.Join(parts[:len(parts)-3], ".")
	} else {
		return "", ""
	}

	return
}

func isAppDNS(m *dns.Msg, w dns.ResponseWriter) bool {
	domain, subdomain := GetDomainAndSubDomain(m.Question[0].Name)
	if domain == "" {
		return false
	}

	start := time.Now()
	rm := new(dns.Msg)
	rm.SetReply(m)
	rm.Authoritative = true
	rm.Compress = true
	var full string
	if subdomain != "" {
		full = subdomain + "." + domain
	} else {
		full = domain
	}

	for _, v := range C.APICertDomains {
		if full == v+"." {
			rm.Answer = append(rm.Answer, &dns.A{
				Hdr: dns.RR_Header{
					Class:  dns.TypeA,
					Rrtype: dns.ClassINET,
					Name:   rm.Question[0].Name,
					Ttl:    5,
				},
				A: net.ParseIP(C.APICertIPs[0]).To4(),
			})
			err := w.WriteMsg(rm)
			if err != nil {
				ERROR("Unable to write app dns reply:", err)
			} else {
				INFO("DNS: ", m.Question[0].Name, fmt.Sprintf("(%d)ms ", time.Since(start).Milliseconds()), " @ local")
			}
			w.Close()
			return true
		}
	}

	return false
}

func DNSAMapping(DNS []*ServerDNS, fullDomain string) *ServerDNS {
	domain, subdomain := GetDomainAndSubDomain(fullDomain)
	if domain == "" {
		return nil
	}
	domain = strings.TrimSuffix(domain, ".")

	for i, record := range DNS {
		// There is a slight chance someone might
		// saves something like "null" into the array.
		// the record == nil will make sure we do not crash on it.
		if record == nil {
			continue
		}
		if subdomain != "" {
			if record.Domain == subdomain+"."+domain {
				return DNS[i]
			}
		}

		if record.Domain == domain {
			if subdomain == "" {
				return DNS[i]
			} else if record.Wildcard {
				return DNS[i]
			}
		}

	}

	return nil
}

func CheckIfPlainDomain(s string) bool {
	if strings.Contains(s, ".") {
		return true
	}
	return false
}
