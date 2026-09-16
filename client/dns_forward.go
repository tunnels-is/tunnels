package client

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/miekg/dns"
	"github.com/tunnels-is/tunnels/types"
)

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

	cacheDNSReply(r)
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

	cacheDNSReply(r)
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
		switch rm.Question[i].Qtype {
		case dns.TypeA:
			if len(DNS.IP) > 0 {
				for ii := range DNS.IP {
					rm.Answer = append(rm.Answer, &dns.A{
						Hdr: dns.RR_Header{
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Name:   rm.Question[i].Name,
							Ttl:    5,
						},
						A: net.ParseIP(DNS.IP[ii]).To4(),
					})
				}
			}
		case dns.TypeTXT:
			if len(DNS.TXT) > 0 {
				for ii := range DNS.TXT {
					rm.Answer = append(rm.Answer, &dns.TXT{
						Hdr: dns.RR_Header{
							Rrtype: dns.TypeTXT,
							Class:  dns.ClassINET,
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

	cln := newDoHClient()

	server := conf.DNS1Default
	resp, err := postDoH(cln, conf.DNS1Default, x)
	if err != nil {

		if conf.DNS2Default != "" {
			server = conf.DNS2Default
			resp, err = postDoH(cln, conf.DNS2Default, x)
		}

		if err != nil {
			if resp != nil {
				resp.Body.Close()
				ERROR("unable to query dns over https: ", m.Question[0].Name, " code: ", resp.StatusCode)
			} else {
				ERROR("unable to query dns over https: ", m.Question[0].Name, " err: ", err)
			}
			return err
		}
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ERROR("dns over https: non-200 from ", server, ": ", resp.StatusCode)
		return fmt.Errorf("dns over https status %d", resp.StatusCode)
	}

	bb, err := io.ReadAll(io.LimitReader(resp.Body, maxDoHResponseSize))
	if err != nil {
		ERROR("Unable to read DNS over HTTP response body:", err)
		return err
	}

	newx := new(dns.Msg)
	if err = newx.Unpack(bb); err != nil {
		ERROR("Unable to unpack DNS over HTTPS response:", err)
		return err
	}
	cacheDNSReply(newx)
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

func newDoHClient() *http.Client {
	return &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: checkPublicHTTPSRedirect,
		Transport: &http.Transport{
			IdleConnTimeout:     10 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
}

func postDoH(cln *http.Client, host string, packed []byte) (*http.Response, error) {
	dohURL := "https://" + host + "/dns-query"
	if u, perr := url.Parse(dohURL); perr != nil {
		return nil, perr
	} else if err := requirePublicHTTPSURL(u); err != nil {
		ERROR("DoH resolver URL refused: ", err)
		return nil, err
	}
	req, err := http.NewRequest("POST", dohURL, bytes.NewBuffer(packed))
	if err != nil {
		ERROR("unable to create http.request for DNS query")
		return nil, err
	}
	req.Header.Add("accept", "application/dns-message")
	req.Header.Add("content-type", "application/dns-message")
	return cln.Do(req)
}
