package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

func ResetEverything() {
	defer RecoverAndLog()
	tunnelMapRange(func(tun *TUN) bool {
		tunnel := tun.tunnel.Load()
		if tunnel != nil {
			_ = tunnel.Disconnect(tun)
		}
		return true
	})

	RestoreSaneDNSDefaults()
}

func SendRequestToURL(tc *tls.Config, method string, url string, data any, timeoutMS int, validateCert bool, extraHeaders ...map[string]string) ([]byte, int, error) {
	defer RecoverAndLog()

	var body []byte
	var err error
	if data != nil {
		body, err = json.Marshal(data)
		if err != nil {
			return nil, 400, err
		}
	}

	var req *http.Request
	switch method {
	case "POST":
		req, err = http.NewRequest(method, url, bytes.NewBuffer(body))
	case "GET":
		req, err = http.NewRequest(method, url, nil)
	default:
		return nil, 400, errors.New("method not supported:" + method)
	}

	if err != nil {
		return nil, 400, err
	}

	req.Header.Add("Content-Type", "application/json")
	if len(extraHeaders) > 0 {
		for k, v := range extraHeaders[0] {
			req.Header.Set(k, v)
		}
	}

	client := http.Client{Timeout: time.Duration(timeoutMS) * time.Millisecond}
	if tc != nil {
		client.Transport = &http.Transport{
			TLSClientConfig: tc,
		}
	} else {
		if !validateCert {
			warnInsecureHost(req.URL.Host)
		}
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion:         tls.VersionTLS13,
				CurvePreferences:   []tls.CurveID{tls.X25519MLKEM768},
				InsecureSkipVerify: !validateCert,
			},
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		if resp != nil {
			return nil, resp.StatusCode, err
		} else {
			return nil, 400, err
		}
	}

	client.CloseIdleConnections()
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	var respBodyBytes []byte
	respBodyBytes, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return respBodyBytes, resp.StatusCode, nil
}

func authorizeControlServer(s *ControlServer) error {
	if s == nil {
		return errors.New("no control server specified")
	}
	conf := CONFIG.Load()
	for _, cs := range conf.ControlServers {
		if cs.Host == s.Host && cs.Port == s.Port {
			s.ValidateCertificate = cs.ValidateCertificate
			s.CertificatePath = cs.CertificatePath
			return nil
		}
	}

	if s.Port == "" {
		for _, cs := range conf.ControlServers {
			if cs.Host == s.Host {
				s.Port = cs.Port
				s.ValidateCertificate = cs.ValidateCertificate
				s.CertificatePath = cs.CertificatePath
				return nil
			}
		}
	}
	return errors.New("host not in configured control servers")
}

func ForwardToController(FR *FORWARD_REQUEST) (any, int) {
	defer RecoverAndLog()

	if err := authorizeControlServer(FR.Server); err != nil {
		er := new(ErrorResponse)
		er.Error = err.Error()
		return er, 403
	}

	url := FR.Server.GetURL(FR.Path)
	responseBytes, code, err := SendRequestToURL(
		nil,
		FR.Method,
		url,
		FR.JSONData,
		FR.Timeout,
		FR.Server.ValidateCertificate,
		FR.Headers,
	)

	er := new(ErrorResponse)
	if err != nil {
		er.Error = err.Error()
		ERROR("Could not forward request (err): ", err)
		return er, 500
	}

	if code == 0 {
		er.Error = "Unable to contact controller"
		ERROR("Could not forward request (code 0): ", err)
		return er, 500
	}

	var respObj any
	if len(responseBytes) != 0 {
		err = json.Unmarshal(responseBytes, &respObj)
		if err != nil {
			ERROR("Could not parse response data from ", FR.Server.Host, ":", FR.Server.Port, " err:", err)
			er.Error = "Unable to open response from controller"
			return er, code
		}
	}

	return respObj, code
}

var warnedInsecureHosts sync.Map

func warnInsecureHost(host string) {
	if _, loaded := warnedInsecureHosts.LoadOrStore(host, struct{}{}); !loaded {
		SECURITY("TLS certificate verification is DISABLED for ", host,
			" (ValidateCertificate=false) — traffic to this controller can be intercepted")
	}
}

var AZ_CHAR_CHECK = regexp.MustCompile(`^[a-zA-Z0-9]*$`)

var TAG_CHAR_CHECK = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

var allowedConfigFormats = map[string]struct{}{
	"": {}, ".json": {}, ".conf": {}, ".yaml": {}, ".yml": {},
}

func safeTunnelTag(tag string) bool {
	return TAG_CHAR_CHECK.MatchString(tag)
}

func validateTunnelMeta(tun *TunnelMETA, oldTag string) (err []string) {
	ifnamemap := make(map[string]struct{})
	ifFail := AZ_CHAR_CHECK.MatchString(tun.IFName)
	if !ifFail {
		err = append(err, "tunnel names can only contain a-z A-Z 0-9, invalid name: "+tun.IFName)
	}

	if !safeTunnelTag(tun.Tag) {
		err = append(err, "tunnel tag may only contain a-z A-Z 0-9 _ - , invalid tag: "+tun.Tag)
	}
	if oldTag != "" && !safeTunnelTag(oldTag) {
		err = append(err, "invalid old tunnel tag: "+oldTag)
	}
	if _, ok := allowedConfigFormats[tun.ConfigFormat]; !ok {
		err = append(err, "unsupported tunnel config format: "+tun.ConfigFormat)
	}

	if tun.KillSwitch && !tun.EnableDefaultRoute {
		err = append(err, "kill switch requires the default route to be enabled")
	}

	tunnelMetaMapRange(func(t *TunnelMETA) bool {
		if t.Tag == tun.Tag {
			return true
		}
		ifnamemap[strings.ToLower(t.IFName)] = struct{}{}
		return true
	})

	_, ok := ifnamemap[strings.ToLower(tun.IFName)]
	if ok {
		if strings.ToLower(tun.IFName) != oldTag {
			err = append(err,
				"you cannot have two tunnels with the same interface name: "+tun.IFName,
			)
		}
	}

	if len(tun.IFName) < 3 {
		err = append(err, fmt.Sprintf("tunnel name should not be less then 3 characters (%s)", tun.IFName))
	}

	errx := ValidateAdapterID(tun)
	if errx != nil {
		err = append(err, errx.Error())
	}

	for _, h := range tun.AllowedHosts {
		if _, errp := NormalizeAllowedHost(h); errp != nil {
			err = append(err, errp.Error())
		}
	}

	return
}

func BandwidthBytesToString(b int64) string {
	if b <= 999 {
		intS := strconv.FormatInt(b, 10)
		return intS + " B"
	} else if b <= 999_999 {
		intF := float64(b)
		return fmt.Sprintf("%.0f KB", intF/1000)
	} else if b <= 999_999_999 {
		intF := float64(b)
		return fmt.Sprintf("%.1f MB", intF/1_000_000)
	} else if b <= 999_999_999_999 {
		intF := float64(b)
		return fmt.Sprintf("%.1f GB", intF/1_000_000_000)
	} else if b <= 999_999_999_999_999 {
		intF := float64(b)
		return fmt.Sprintf("%.1f TB", intF/1_000_000_000_000)
	}

	return "???"
}
