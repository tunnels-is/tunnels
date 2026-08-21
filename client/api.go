package client

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// errControllerRedirect is returned when the controller answers with a
// redirect. Redirects are refused so X-Device-Token is never re-sent to
// a different Location (Go does not strip custom auth headers).
var errControllerRedirect = errors.New("refusing to follow controller redirect (X-Device-Token must not leave the configured controller URL)")

func ResetEverything() {
	defer RecoverAndLog()
	tunnelMapRange(func(tun *TUN) bool {
		tunnel := tun.tunnel.Load()
		if tunnel != nil {
			_ = tunnel.Disconnect(tun)
		}
		return true
	})
}

func SendRequestToURL(tc *tls.Config, method string, url string, data any, timeoutMS int, validateCert bool, certPath string, extraHeaders ...map[string]string) ([]byte, int, error) {
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

	client := http.Client{
		Timeout: time.Duration(timeoutMS) * time.Millisecond,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errControllerRedirect
		},
	}
	if tc != nil {
		client.Transport = &http.Transport{
			TLSClientConfig: tc,
		}
	} else {
		if !validateCert {
			warnInsecureHost(req.URL.Host)
		}
		tlsCfg, tlsErr := tlsConfigForController(validateCert, certPath)
		if tlsErr != nil {
			return nil, 400, tlsErr
		}
		client.Transport = &http.Transport{
			TLSClientConfig: tlsCfg,
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

func tlsConfigForController(validateCert bool, certPath string) (*tls.Config, error) {
	cfg := &tls.Config{
		MinVersion:       tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.X25519MLKEM768},
	}
	if !validateCert {
		cfg.InsecureSkipVerify = true
		return cfg, nil
	}
	if certPath == "" {
		return cfg, nil
	}
	pemBytes, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("certificate path: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemBytes) {
		return nil, fmt.Errorf("certificate path %q contains no PEM certificates", certPath)
	}
	cfg.RootCAs = pool
	return cfg, nil
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
		FR.Server.CertificatePath,
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

// safeTunnelTag validates tunnel tags used as on-disk identifiers.
func safeTunnelTag(tag string) bool {
	return TAG_CHAR_CHECK.MatchString(tag)
}

// safeListTag validates DNS blocklist/whitelist tags used as filenames under
// blocklists/ and whitelists/. Same charset as tunnel tags to prevent path traversal.
func safeListTag(tag string) bool {
	return TAG_CHAR_CHECK.MatchString(tag)
}

// listFilePath joins baseDir with a validated, lowercased list tag and ensures
// the result stays inside baseDir (no ".." / separator escapes).
func listFilePath(baseDir, tag string) (string, error) {
	if !safeListTag(tag) {
		return "", fmt.Errorf("invalid list tag %q: only a-z A-Z 0-9 _ - allowed", tag)
	}
	lower := strings.ToLower(tag)
	base := filepath.Clean(baseDir)
	path := filepath.Join(base, lower)
	rel, err := filepath.Rel(base, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("list path escapes base directory")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("list path escapes base directory")
	}
	return path, nil
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
