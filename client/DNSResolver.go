package client

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
	"github.com/tunnels-is/tunnels/types"
	"golang.org/x/net/idna"
)

func FullCleanDNSCache() {
	defer RecoverAndLogToFile()
	INFO("Dumping DNS cache")
	DNSCache.Clear()
}

func CleanDNSCache() {
	defer func() {
		time.Sleep(30 * time.Second)
	}()
	defer RecoverAndLogToFile()

	INFO("Cleaning DNS cache")

	DNSCache.Range(func(key, value any) bool {
		dr, ok := value.(*DNSReply)
		if !ok {
			return true
		}

		if time.Since(dr.Expires).Seconds() > 1 {
			DNSCache.Delete(key)
		}

		return true
	})
}

// func updateDNSHandlerInterface(dnsInterface net.IP) {
// 	if DNSClient != nil && DNSClient.Dialer != nil {
// 		if dnsInterface == nil {
// 			DNSClient.Dialer.LocalAddr = nil
// 		} else {
// 			DNSClient.Dialer.LocalAddr = &net.UDPAddr{
// 				IP: dnsInterface,
// 			}
// 		}
// 	}
// }

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

	conf := CONFIG.Load()
	ip := conf.DNSServerIP
	if ip == "" {
		ip = DefaultDNSIP
	}

	port := conf.DNSServerPort
	if port == "" {
		port = DefaultDNSPort
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

func ResolveDomainLocal(tun *TUN, m *dns.Msg, w dns.ResponseWriter) {
	if len(tun.ServerResponse.DNSServers) == 0 {
		return
	}

	if GlobalBlockEnabled(m, w) {
		return
	}

	start := time.Now()
	var r *dns.Msg
	var err error
	var server string
	conf := CONFIG.Load()

	defer func() {
		meta := tun.meta.Load()
		if err != nil {
			ERROR("DNS: ", m.Question[0].Name, " || ", fmt.Sprintf("(%d)ms ", time.Since(start).Milliseconds()), " || ", meta.Tag, " || ", err)
		} else {
			if conf.LogAllDomains {
				INFO("DNS: ", m.Question[0].Name, fmt.Sprintf("(%d)ms ", time.Since(start).Milliseconds()), " @ ", meta.Tag, " @ ", server)
			}
			if conf.DNSstats {
				IncrementDNSStats(m.Question[0].Name, false, "", r.Answer)
			}
		}
	}()

	r, _, err = tun.localDNSClient.Exchange(m, tun.ServerResponse.DNSServers[0]+":53")
	server = tun.ServerResponse.DNSServers[0]

	if err != nil && len(tun.ServerResponse.DNSServers) > 1 {
		r, _, err = tun.localDNSClient.Exchange(m, tun.ServerResponse.DNSServers[1]+":53")
		server = tun.ServerResponse.DNSServers[1]
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

func ResolveDomain(m *dns.Msg, w dns.ResponseWriter) (err error) {
	if GlobalBlockEnabled(m, w) {
		DEBUG("global dns lock enabled due to connection switching")
		return fmt.Errorf("dns lock enabled")
	}

	start := time.Now()
	var r *dns.Msg
	var server string
	conf := CONFIG.Load()

	defer func() {
		if err != nil {
			ERROR("DNS: ", m.Question[0].Name+" >> ", fmt.Sprintf("(%d)ms >>  ", time.Since(start).Milliseconds()), err)
		} else {
			if conf.LogAllDomains {
				INFO("DNS: ", m.Question[0].Name, fmt.Sprintf("(%d)ms ", time.Since(start).Milliseconds()), " @  ", server)
			}
			if conf.DNSstats {
				IncrementDNSStats(m.Question[0].Name, false, "", r.Answer)
			}
		}
	}()

	r, _, err = DNSClient.Exchange(m, conf.DNS1Default+":53")
	server = conf.DNS1Default
	if err != nil && conf.DNS2Default != "" {
		r, _, err = DNSClient.Exchange(m, conf.DNS2Default+":53")
		server = conf.DNS2Default
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
	return nil
}

func ProcessDNSMsg(m *dns.Msg, DNS *types.DNSRecord) (rm *dns.Msg) {
	rm = new(dns.Msg)
	rm.SetReply(m)
	rm.Authoritative = true
	rm.Compress = true

	for i := range rm.Question {
		if rm.Question[i].Qtype == dns.TypeA {
			if len(DNS.IP) > 0 {
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
		}
	}

	return
}

func GlobalBlockEnabled(m *dns.Msg, w dns.ResponseWriter) bool {
	if DNSGlobalBlock.Load() {
		_ = w.WriteMsg(m)
		w.Close()
		INFO("DNS BLOCKED (connection switching in progress): ", m.Question[0].Name)
		return true
	}
	return false
}

func DNSQuery(w dns.ResponseWriter, m *dns.Msg) {
	defer RecoverAndLogToFile()

	// if isAppDNS(m, w) {
	// 	return
	// }

	if !isValidDomain(m, w) {
		return
	}

	if DNSCacheCheck(m, w) {
		return
	}

	blocked, tag := isBlocked(m)

	var DNSTunnel *TUN
	var ServerDNS *types.DNSRecord
	tunnelMapRange(func(tun *TUN) bool {
		if tun.GetState() != TUN_Connected {
			return true
		}

		meta := tun.meta.Load()
		if meta == nil {
			return true
		}

		if meta.DNSBlocking && blocked {
			return true
		}

		if tun.ServerResponse == nil {
			return true
		}

		ServerDNS = DNSAMapping(tun.ServerResponse.DNSRecords, m.Question[0].Name)
		if ServerDNS != nil {
			DNSTunnel = tun
			return false
		}

		return true
	})

	conf := CONFIG.Load()
	if ServerDNS == nil {
		ServerDNS = DNSAMapping(conf.DNSRecords, m.Question[0].Name)
	}

	if blocked && ServerDNS == nil {
		if conf.DNSstats {
			IncrementDNSStats(m.Question[0].Name, true, tag, nil)
		}

		if conf.LogBlockedDomains {
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
		} else if len(ServerDNS.TXT) > 0 {
			hasInfo = true
		}

		if !hasInfo {
			DEBUG("Redirect DNS to VPN: ", m.Question[0].Name)
			// Redirect DNS query to local VPN network if we
			// have the domain on record but no records.
			ResolveDomainLocal(DNSTunnel, m, w)
			return
		}

		if conf.LogAllDomains {
			if DNSTunnel != nil {
				meta := DNSTunnel.meta.Load()
				INFO("DNS @ server:", meta.Tag, " >> ", m.Question[0].Name, " >> local record found")
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
		if conf.DNSstats {
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

	if conf.DNSOverHTTPS {
		err := ResolveDNSAsHTTPS(m, w)
		if err != nil {
			_ = w.WriteMsg(m)
		}
	} else {
		err := ResolveDomain(m, w)
		if err != nil {
			_ = w.WriteMsg(m)
		}
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
		DEBUG("Invalid domain: ", m.Question[0].Name)
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
	DNSCache.Store(name, RP)
}

func DNSCacheCheck(m *dns.Msg, w dns.ResponseWriter) bool {
	nameAndType := m.Question[0].Name + strconv.FormatUint(uint64(m.Question[0].Qtype), 10)

	value, ok := DNSCache.Load(nameAndType)
	if !ok {
		return false
	}
	cachedReply, ok := value.(*DNSReply)
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
	conf := CONFIG.Load()
	if conf.LogAllDomains {
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

func isBlocked(m *dns.Msg) (ok bool, tag string) {
	name := strings.TrimSuffix(m.Question[0].Name, ".")
	bl := DNSBlockList.Load()
	if bl == nil {
		return false, ""
	}

	tagString, ok := bl.Load(name)
	if ok && tagString != nil {
		bl := tagString.(*BlockList)
		tag = bl.Tag
	}

	return ok, tag
}

func ResolveDNSAsHTTPS(m *dns.Msg, w dns.ResponseWriter) (err error) {
	if GlobalBlockEnabled(m, w) {
		DEBUG("global dns lock enabled due to connection switching")
		return fmt.Errorf("dns lock enabled")
	}

	conf := CONFIG.Load()
	start := time.Now()
	x, err := m.Pack()
	if err != nil {
		ERROR("unable to prepare DNS msg as HTTPS msg")
		return err
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
	server := conf.DNS1Default
	req1, err = http.NewRequest("POST", "https://"+conf.DNS1Default+"/dns-query", bytes.NewBuffer(x))
	if err != nil {
		ERROR("unable to create http.request for DNS query")
		return err
	}

	req1.Header.Add("accept", "application/dns-message")
	req1.Header.Add("content-type", "application/dns-message")
	resp, err := cln.Do(req1)
	if err != nil {

		if conf.DNS2Default != "" {
			server = conf.DNS2Default
			req2, err = http.NewRequest("POST", "https://"+conf.DNS2Default+"/dns-query", bytes.NewBuffer(x))
			if err != nil {
				ERROR("unable to create http.request for DNS query")
				return err
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
			return err
		}
	}

	bb, err := io.ReadAll(resp.Body)
	if err != nil {
		ERROR("Unable to read DNS over HTTP response body:", err)
		return err
	}

	newx := new(dns.Msg)
	newx.Unpack(bb)
	CacheDnsReply(newx)
	err = w.WriteMsg(newx)
	w.Close()
	if err != nil {
		ERROR("Unable to  write dns reply:", err)
		return err
	}

	INFO("DNS(https): ", m.Question[0].Name, fmt.Sprintf("(%d)ms ", time.Since(start).Milliseconds()), " @  ", server)
	if conf.DNSstats {
		IncrementDNSStats(m.Question[0].Name, false, "", newx.Answer)
	}
	return nil
}

func IncrementDNSStats(domain string, blocked bool, tag string, answers []dns.RR) {
	defer RecoverAndLogToFile()

	tn := time.Now()
	dnsint, ok := DNSStatsMap.LoadOrStore(domain, &DNSStats{})
	dnsStats := dnsint.(*DNSStats)

	dnsStats.m.Lock()
	if !ok {
		dnsStats.FirstSeen = tn
	}
	if blocked {
		dnsStats.LastBlocked = tn
	} else {
		dnsStats.LastResolved = tn
	}
	dnsStats.Tag = tag
	dnsStats.Count++
	dnsStats.LastSeen = tn
	for _, v := range answers {
		dnsStats.Answers = append(dnsStats.Answers, v.String())
	}
	dnsStats.m.Unlock()
}
