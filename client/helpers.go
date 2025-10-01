package client

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

func CreateConnectionUUID() string {
	return "{" + strings.ToUpper(uuid.NewString()) + "}"
}

func IsAlphanumeric(s string) bool {
	matched, _ := regexp.MatchString(`^[a-z0-9]+$`, s)
	return matched
}

func InitBaseFoldersAndPaths() {
	defer RecoverAndLog()
	DEBUG("Creating base folders and paths")
	s := STATE.Load()

	basePath := s.BasePath
	basePath, _ = strings.CutSuffix(basePath, string(os.PathSeparator))

	if basePath != "" {
		basePath = s.BasePath + string(os.PathSeparator)
	} else {
		ex, err := os.Executable()
		if err != nil {
			wd, err := os.Getwd()
			if err != nil {
				panic(err)
			}
			basePath = wd + string(os.PathSeparator)
		} else {
			basePath = filepath.Dir(ex) + string(os.PathSeparator)
		}
	}

	s.BasePath = basePath
	s.TunnelsPath = s.BasePath + "tunnel" + string(os.PathSeparator)
	CreateFolder(s.TunnelsPath)

	s.UserPath = s.BasePath + "users" + string(os.PathSeparator)
	CreateFolder(s.UserPath)

	CreateFolder(s.BasePath)
	s.ConfigFileName = s.BasePath + "tunnels" + configFileSuffix

	s.LogPath = s.BasePath + "logs" + string(os.PathSeparator)
	CreateFolder(s.LogPath)
	s.LogFileName = s.LogPath + time.Now().Format("2006-01-02") + ".log"

	s.BlockListPath = s.BasePath + "blocklists" + string(os.PathSeparator)
	CreateFolder(s.BlockListPath)
	STATE.Store(s)
}

func RenameFile(oldName, newName string) (err error) {
	_, err = os.Stat(oldName)
	if err != nil {
		if os.IsNotExist(err) {
			DEBUG("File does not exist: ", oldName)
			return nil
		}
		ERROR("Unable to check file: ", err)
		return
	}

	err = os.Rename(oldName, newName)
	if err != nil {
		ERROR("Unable to rename file: ", err)
		return
	}

	DEBUG("File renamed: ", oldName, " -> ", newName)
	return nil
}

func RemoveFile(file string) (err error) {
	_, err = os.Stat(file)
	if err != nil {
		if os.IsNotExist(err) {
			DEBUG("File does not exist: ", file)
			return nil
		}
		ERROR("Unable to check file: ", err)
		return
	}

	err = os.Remove(file)
	if err != nil {
		ERROR("Unable to remove file: ", err)
		return
	}

	DEBUG("File removed: ", file)
	return nil
}

func CreateFile(file string) (f *os.File, err error) {
	f, err = os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o777)
	if err != nil {
		ERROR("Unable to open file: ", err)
		return
	}

	DEBUG("File opened: ", f.Name())
	return
}

func CreateFolder(path string) {
	_, err := os.Stat(path)
	if err != nil {
		err = os.Mkdir(path, 0o777)
		if err != nil {
			ERROR("Unable to create base folder: ", err)
			return
		}
	}
	DEBUG("New directory:", path)
}

func IsDefaultConnection(IFName string) bool {
	return strings.EqualFold(IFName, DefaultTunnelName)
}

func RecoverAndLog() {
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

// We don't want to use this yet, but I want to keep it around for now.
// We don't want to use this yet, but I want to keep it around for now.
// We don't want to use this yet, but I want to keep it around for now.
// func isAppDNS(m *dns.Msg, w dns.ResponseWriter) bool {
// 	domain, subdomain := GetDomainAndSubDomain(m.Question[0].Name)
// 	if domain == "" {
// 		return false
// 	}
//
// 	start := time.Now()
// 	rm := new(dns.Msg)
// 	rm.SetReply(m)
// 	rm.Authoritative = true
// 	rm.Compress = true
// 	var full string
// 	if subdomain != "" {
// 		full = subdomain + "." + domain
// 	} else {
// 		full = domain
// 	}
//
// 	for _, v := range C.APICertDomains {
// 		if full == v+"." {
// 			rm.Answer = append(rm.Answer, &dns.A{
// 				Hdr: dns.RR_Header{
// 					Class:  dns.TypeA,
// 					Rrtype: dns.ClassINET,
// 					Name:   rm.Question[0].Name,
// 					Ttl:    5,
// 				},
// 				A: net.ParseIP(C.APICertIPs[0]).To4(),
// 			})
// 			err := w.WriteMsg(rm)
// 			if err != nil {
// 				ERROR("Unable to write app dns reply:", err)
// 			} else {
// 				INFO("DNS: ", m.Question[0].Name, fmt.Sprintf("(%d)ms ", time.Since(start).Milliseconds()), " @ local")
// 			}
// 			w.Close()
// 			return true
// 		}
// 	}
//
// 	return false
// }

func DNSAMapping(DNS []*types.DNSRecord, fullDomain string) *types.DNSRecord {
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
	return strings.Contains(s, ".")
}

func tunnelMapRange(do func(tun *TUN) bool) {
	TunnelMap.Range(func(key string, value *TUN) bool {
		return do(value)
	})
}

func tunnelMetaMapRange(do func(tun *TunnelMETA) bool) {
	TunnelMetaMap.Range(func(key string, value *TunnelMETA) bool {
		return do(value)
	})
}

func doEvent(channel chan *event, method func()) {
	defer RecoverAndLog()
	select {
	case channel <- &event{
		method: method,
	}:
	default:
		panic("priority channel full")
	}
}

func (e *event) Wait(method func(any), timeout time.Duration) {
	defer RecoverAndLog()
	tick := time.NewTimer(timeout)
	select {
	case done := <-e.done:
		method(done)
		return
	case <-tick.C:
		method(errors.New("timeout waiting"))
	}
}

func newConcurrentSignal(tag string, ctx context.Context, method func()) {
	defer RecoverAndLog()
	select {
	case concurrencyMonitor <- &goSignal{
		monitor: concurrencyMonitor,
		tag:     tag,
		ctx:     ctx,
		method:  method,
	}:
	default:
		panic("concurrency monitor is full")
	}
}

func (s *goSignal) execute() {
	defer RecoverAndLog()
	s.method()
	time.Sleep(1 * time.Second)

	select {
	case s.monitor <- s:
	default:
		panic("monitor channel is full")
	}
}
