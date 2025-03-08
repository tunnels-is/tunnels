package core

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/net/idna"
)

func FullCleanDNSCache() {
	defer func() {
		RecoverAndLogToFile()
		DNSCacheLock.Unlock()
	}()
	DNSCacheLock.Lock()

	INFO("Dumping DNS cache")
	DNSCache = nil
	DNSCache = make(map[string]*DNSReply)
}

func CleanDNSCache() {
	defer func() {
		DNSCacheLock.Unlock()
		time.Sleep(60 * time.Second)
	}()
	DNSCacheLock.Lock()
	defer RecoverAndLogToFile()

	INFO("cleaning DNS cache")

	for i, v := range DNSCache {
		if time.Since(v.Expires).Seconds() > 1 {
			delete(DNSCache, i)
		}
	}
}

func InitDNSHandler() {
	DEBUG("Starting DNS Handler")
	DNSClient.Dialer = new(net.Dialer)
	DNSClient.Dialer.Resolver = new(net.Resolver)
	DNSClient.Dialer.Resolver.PreferGo = false
	DNSClient.Timeout = time.Second * 5
	DNSClient.Dialer.Timeout = time.Duration(5 * time.Second)
	DNSClient.WriteTimeout = time.Duration(5 * time.Second)
	DNSClient.ReadTimeout = time.Duration(5 * time.Second)
}

func StartUDPDNSHandler() {
	defer RecoverAndLogToFile()

	udpHandler := dns.NewServeMux()
	udpHandler.HandleFunc(".", DNSQuery)

	ip := STATEOLD.C.DNSServerIP
	if ip == "" {
		ip = "127.0.0.1"
	}
	port := STATEOLD.C.DNSServerPort
	if port == "" {
		port = "53"
	}

	UDPDNSServer = &dns.Server{
		Addr:         ip + ":" + port,
		Net:          "udp4",
		Handler:      udpHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	err := UDPDNSServer.ListenAndServe()
	if err != nil {
		ERROR("DNS SERVER SHUTDOWN: ", err)
	}
}

func ResolveDomainLocal(V *Tunnel, m *dns.Msg, w dns.ResponseWriter) {
	if GlobalBlockEnabled(m, w) {
		return
	}
	dialer := new(net.Dialer)
	dialer.LocalAddr = &net.UDPAddr{
		IP: DEFAULT_INTERFACE.To4(),
	}
	localClient := new(dns.Client)
	localClient.Dialer = dialer
	localClient.Dialer.Resolver = DNSClient.Dialer.Resolver
	localClient.Dialer.Timeout = time.Duration(5 * time.Second)
	localClient.Timeout = time.Second * 5

	start := time.Now()
	var r *dns.Msg
	var err error
	var server string

	defer func() {
		if err != nil {
			ERROR("DNS: ", m.Question[0].Name, " || ", fmt.Sprintf("(%d)ms ", time.Since(start).Milliseconds()), " || ", V.Meta.Tag, " || ", err)
		} else {
			if STATEOLD.C.LogAllDomains {
				INFO("DNS: ", m.Question[0].Name, fmt.Sprintf("(%d)ms ", time.Since(start).Milliseconds()), " @ ", V.Meta.Tag, " @ ", server)
			}
			if STATEOLD.C.DNSstats {
				IncrementDNSStats(m.Question[0].Name, false, "", r.Answer)
			}
		}
	}()

	if len(V.CRR.DNSServers) == 0 {
		return
	}

	r, _, err = localClient.Exchange(m, V.CRR.DNSServers[0]+":53")
	server = V.CRR.DNSServers[0]
	if err != nil && len(V.CRR.DNSServers) > 1 {
		r, _, err = localClient.Exchange(m, V.CRR.DNSServers[1]+":53")
		server = V.CRR.DNSServers[1]
	}

	if err != nil {
		return
	}

	CacheDnsReply(r)
	err = w.WriteMsg(r)
	w.Close()
	if err != nil {
		ERROR("Unable to  write dns reply:", err)
	}
}

func ResolveDomain(m *dns.Msg, w dns.ResponseWriter) {
	if GlobalBlockEnabled(m, w) {
		return
	}

	start := time.Now()
	var err error
	var r *dns.Msg
	var server string
	defer func() {
		if err != nil {
			ERROR("DNS: ", m.Question[0].Name, " || ", fmt.Sprintf("(%d)ms ", time.Since(start).Milliseconds()), " || ", err)
		} else {
			if STATEOLD.C.LogAllDomains {
				INFO("DNS: ", m.Question[0].Name, fmt.Sprintf("(%d)ms ", time.Since(start).Milliseconds()), " @  ", server)
			}
			if STATEOLD.C.DNSstats {
				IncrementDNSStats(m.Question[0].Name, false, "", r.Answer)
			}
		}
	}()

	r, _, err = DNSClient.Exchange(m, C.DNS1Default+":53")
	server = C.DNS1Default
	if err != nil && C.DNS2Default != "" {
		r, _, err = DNSClient.Exchange(m, C.DNS2Default+":53")
		server = C.DNS2Default
	}

	if err != nil {
		return
	}

	CacheDnsReply(r)
	err = w.WriteMsg(r)
	w.Close()
	if err != nil {
		ERROR("Unable to  write dns reply:", err)
	}
}

func ProcessDNSMsg(m *dns.Msg, DNS *ServerDNS) (rm *dns.Msg) {
	rm = new(dns.Msg)
	rm.SetReply(m)
	rm.Authoritative = true
	rm.Compress = true

	for i := range rm.Question {
		if rm.Question[i].Qtype == dns.TypeA {
			if DNS.CNAME != "" {
				rm.Answer = append(rm.Answer, &dns.CNAME{
					Hdr: dns.RR_Header{
						Class:  dns.ClassNONE,
						Rrtype: dns.TypeCNAME,
						Name:   rm.Question[i].Name,
						Ttl:    5,
					},
					Target: DNS.CNAME + ".",
				})
			} else if len(DNS.IP) > 0 {
				for ii := range DNS.IP {
					rm.Answer = append(rm.Answer, &dns.A{
						Hdr: dns.RR_Header{
							Class:  dns.TypeA,
							Rrtype: dns.ClassINET,
							Name:   rm.Question[i].Name,
							Ttl:    5,
						},
						A: net.ParseIP(DNS.IP[ii]).To4(),
					})
				}
			}
		} else if rm.Question[i].Qtype == dns.TypeTXT {
			if len(DNS.TXT) > 0 {
				for ii := range DNS.TXT {
					rm.Answer = append(rm.Answer, &dns.TXT{
						Hdr: dns.RR_Header{
							Class:  dns.ClassNONE,
							Rrtype: dns.TypeTXT,
							Name:   rm.Question[i].Name,
							Ttl:    30,
						},
						Txt: []string{DNS.TXT[ii]},
					})
				}
			}
		} else if rm.Question[i].Qtype == dns.TypeCNAME {
			if DNS.CNAME != "" {
				rm.Answer = append(rm.Answer, &dns.CNAME{
					Hdr: dns.RR_Header{
						Class:  dns.ClassNONE,
						Rrtype: dns.TypeCNAME,
						Name:   rm.Question[i].Name,
						Ttl:    30,
					},
					Target: DNS.CNAME + ".",
				})
			}
		}
	}

	return
}

func GlobalBlockEnabled(m *dns.Msg, w dns.ResponseWriter) bool {
	if BLOCK_DNS_QUERIES {
		_ = w.WriteMsg(m)
		w.Close()
		INFO("DNS BLOCKED (connection switching in progress): ", m.Question[0].Name)
		return true
	}
	return false
}

func DNSQuery(w dns.ResponseWriter, m *dns.Msg) {
	defer RecoverAndLogToFile()
	// ip := strings.Split(w.RemoteAddr().String(), ":")[0]

	if isAppDNS(m, w) {
		return
	}

	if !isValidDomain(m, w) {
		return
	}

	if DNSCacheCheck(m, w) {
		return
	}

	blocked, tag := isBlocked(m)

	var Connection *Tunnel
	var ServerDNS *ServerDNS
	for i, con := range TunList {
		if con == nil {
			continue
		}

		if !con.Connected {
			continue
		}

		if con.Meta.DNSBlocking && blocked {
			continue
		}

		if con.CRR == nil {
			continue
		}

		ServerDNS = DNSAMapping(con.CRR.DNS, m.Question[0].Name)
		if ServerDNS != nil {
			Connection = TunList[i]
			break
		}
	}

	if ServerDNS == nil {
		ServerDNS = DNSAMapping(C.DNSRecords, m.Question[0].Name)
	}

	if blocked && ServerDNS == nil {
		if STATEOLD.C.DNSstats {
			IncrementDNSStats(m.Question[0].Name, true, tag, nil)
		}

		if STATEOLD.C.LogBlockedDomains {
			INFO("DNS BLOCKED: ", m.Question[0].Name)
		}
		err := w.WriteMsg(m)
		if err != nil {
			ERROR("Unable to  write dns reply:", err)
		}
		w.Close()
		return
	}

	if ServerDNS != nil {
		hasInfo := false
		if len(ServerDNS.IP) > 0 {
			hasInfo = true
		} else if ServerDNS.CNAME != "" {
			hasInfo = true
		} else if len(ServerDNS.TXT) > 0 {
			hasInfo = true
		}

		if !hasInfo {
			DEBUG("Redirect DNS to VPN: ", m.Question[0].Name)
			// Redirect DNS query to local VPN network if we
			// have the domain on record but no records.
			ResolveDomainLocal(Connection, m, w)
			return
		}

		if STATEOLD.C.LogAllDomains {
			if Connection != nil {
				INFO("DNS @ server:", Connection.Meta.Tag, " >> ", m.Question[0].Name, " >> local record found")
			} else {
				INFO("DNS @ local:", m.Question[0].Name, " >> local record found")
			}
		}

		outMsg := ProcessDNSMsg(m, ServerDNS)
		err := w.WriteMsg(outMsg)
		if err != nil {
			ERROR("Unable to  write dns reply:", err)
		}
		w.Close()
		if STATEOLD.C.DNSstats {
			IncrementDNSStats(m.Question[0].Name, false, tag, outMsg.Answer)
		}
		return

	}

	if strings.HasSuffix(m.Question[0].Name, ".lan.") {
		INFO("Dropping query for: ", m.Question[0].Name)
		err := w.WriteMsg(m)
		if err != nil {
			ERROR("Unable to  write dns reply:", err)
		}

		w.Close()
		return
	}

	if C.DNSOverHTTPS {
		ResolveDNSAsHTTPS(m, w)
	} else {
		ResolveDomain(m, w)
	}
}

func isValidDomain(m *dns.Msg, w dns.ResponseWriter) bool {
	shouldDrop := false
	_, err := idna.Lookup.ToASCII(m.Question[0].Name)
	if err != nil {
		shouldDrop = true
		goto DONE
	}

	if strings.HasSuffix(m.Question[0].Name, ".arpa.") {
		shouldDrop = true
		goto DONE
	}

DONE:
	if shouldDrop {
		_ = w.WriteMsg(m)
		w.Close()
		INFO("Invalid domain: ", m.Question[0].Name)
		return false
	}

	return true
}

func CacheDnsReply(reply *dns.Msg) {
	if len(reply.Answer) == 0 {
		return
	}

	name := reply.Question[0].Name + strconv.FormatUint(uint64(reply.Question[0].Qtype), 10)
	RP := new(DNSReply)
	RP.A = make([]dns.RR, len(reply.Answer))
	copy(RP.A, reply.Answer)
	TTL := int(reply.Answer[0].Header().Ttl)
	RP.Expires = time.Now().Add(time.Second * time.Duration(TTL))
	DNSCacheLock.Lock()
	DNSCache[name] = RP
	DNSCacheLock.Unlock()
}

func DNSCacheCheck(m *dns.Msg, w dns.ResponseWriter) bool {
	nameAndType := m.Question[0].Name + strconv.FormatUint(uint64(m.Question[0].Qtype), 10)

	DNSCacheLock.Lock()
	cachedReply, ok := DNSCache[nameAndType]
	DNSCacheLock.Unlock()
	if !ok {
		return false
	}

	if time.Since(cachedReply.Expires) > 1 {
		return false
	}

	m.Answer = cachedReply.A
	m.Response = true
	m.Authoritative = true
	m.RecursionAvailable = false

	_ = w.WriteMsg(m)
	w.Close()
	if STATEOLD.C.LogAllDomains {
		INFO(
			"DNS CACHE: ",
			m.Question[0].Name,
			" | TYPE: ",
			strconv.FormatUint(uint64(m.Question[0].Qtype), 10),
			" | Expires(seconds): ",
			fmt.Sprintf("%.2f", time.Until(cachedReply.Expires).Seconds()),
		)
	}

	IncrementDNSStats(m.Question[0].Name, false, "", cachedReply.A)
	return true
}

func isBlocked(m *dns.Msg) (bool, string) {
	name := strings.TrimSuffix(m.Question[0].Name, ".")
	DNSBlockLock.Lock()
	tag, ok := DNSBlockList[name]
	DNSBlockLock.Unlock()

	return ok, tag
}

func ResolveDNSAsHTTPS(m *dns.Msg, w dns.ResponseWriter) {
	if GlobalBlockEnabled(m, w) {
		return
	}
	start := time.Now()
	x, err := m.Pack()
	if err != nil {
		ERROR("unable to prepare DNS msg as HTTPS msg")
		return
	}

	cln := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			IdleConnTimeout:     10 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	var req1 *http.Request
	var req2 *http.Request
	server := C.DNS1Default
	req1, err = http.NewRequest("POST", "https://"+C.DNS1Default+"/dns-query", bytes.NewBuffer(x))
	if err != nil {
		ERROR("unable to create http.request for DNS query")
		return
	}

	req1.Header.Add("accept", "application/dns-message")
	req1.Header.Add("content-type", "application/dns-message")
	resp, err := cln.Do(req1)
	if err != nil {

		if C.DNS2Default != "" {
			server = C.DNS2Default
			req2, err = http.NewRequest("POST", "https://"+C.DNS2Default+"/dns-query", bytes.NewBuffer(x))
			if err != nil {
				ERROR("unable to create http.request for DNS query")
				return
			}

			req2.Header.Add("accept", "application/dns-message")
			req2.Header.Add("content-type", "application/dns-message")
			resp, err = cln.Do(req2)
		}

		if err != nil {
			if resp != nil {
				ERROR("unable to query dns over https: ", m.Question[0].Name, " code: ", resp.StatusCode)
			} else {
				ERROR("unable to query dns over https: ", m.Question[0].Name, " err: ", err)
			}
			return
		}
	}

	bb, err := io.ReadAll(resp.Body)
	if err != nil {
		ERROR("Unable to read DNS over HTTP response body:", err)
		return
	}

	newx := new(dns.Msg)
	newx.Unpack(bb)
	CacheDnsReply(newx)
	err = w.WriteMsg(newx)
	w.Close()
	if err != nil {
		ERROR("Unable to  write dns reply:", err)
		return
	}

	INFO("DNS(https): ", m.Question[0].Name, fmt.Sprintf("(%d)ms ", time.Since(start).Milliseconds()), " @  ", server)
	if STATEOLD.C.DNSstats {
		IncrementDNSStats(m.Question[0].Name, false, "", newx.Answer)
	}
}

func IncrementDNSStats(domain string, blocked bool, tag string, answers []dns.RR) {
	DNSLock.Lock()
	defer RecoverAndLogToFile()
	defer DNSLock.Unlock()

	var s *DNSStats
	var ok bool
	if blocked {
		s, ok = DNSBlockedList[domain]
		if !ok {
			DNSBlockedList[domain] = new(DNSStats)
			s = DNSBlockedList[domain]
			s.Tag = tag
			s.FirstSeen = time.Now()
		}
	} else {
		s, ok = DNSResolvedList[domain]
		if !ok {
			DNSResolvedList[domain] = new(DNSStats)
			s = DNSResolvedList[domain]
			s.Tag = tag
			s.FirstSeen = time.Now()
		}
	}

	s.Count++
	s.LastSeen = time.Now()
	for _, v := range answers {
		s.Answers = append(s.Answers, v.String())
	}
	return
}
