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

// SendRequestToURL sends a JSON request. validateCert mirrors
// Server.ValidateCertificate: true verifies the peer's TLS certificate, false
// skips verification (explicit opt-out for self-signed controllers). The
// parameter was previously named skipVerify while carrying the opposite
// meaning — a footgun that made the zero value fail open.
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

// authorizeControlServer enforces that s names a configured control server and
// takes its transport-security-relevant fields (ValidateCertificate,
// CertificatePath) from the stored config. This prevents a caller that reaches
// the local API from (a) pointing the client at an arbitrary host/port (SSRF)
// and (b) supplying ValidateCertificate=false / a custom cert path to strip TLS.
//
// The allowlist is the set of (Host, Port) pairs in ControlServers — a host may
// legitimately be configured on multiple ports (e.g. api.tunnels.is:443 and
// :444), so matching on Host alone and rewriting the port would force every
// request onto whichever entry happened to be listed first. Match on Host+Port
// and keep the requested port; only fall back to filling the port from the
// config when the caller left it empty.
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
	// Caller passed only a host (empty port): resolve it from the first host
	// match, preserving the previous "fill port from config" behaviour.
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

// warnedInsecureHosts tracks hosts we've already warned about, so an
// unverified-TLS controller shows up in the logs once instead of per request.
var warnedInsecureHosts sync.Map

func warnInsecureHost(host string) {
	if _, loaded := warnedInsecureHosts.LoadOrStore(host, struct{}{}); !loaded {
		SECURITY("TLS certificate verification is DISABLED for ", host,
			" (ValidateCertificate=false) — traffic to this controller can be intercepted")
	}
}

var AZ_CHAR_CHECK = regexp.MustCompile(`^[a-zA-Z0-9]*$`)

// TAG_CHAR_CHECK bounds a tunnel Tag to a safe filename charset. Tag is
// concatenated into on-disk paths (writeTunnelsToDisk, delete handlers), so
// path separators or ".." would be a traversal (arbitrary file write/delete).
var TAG_CHAR_CHECK = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// allowedConfigFormats is the whitelist of tunnel-file extensions. Anything
// else is attacker-controlled input that would land in a filesystem path.
var allowedConfigFormats = map[string]struct{}{
	"": {}, ".json": {}, ".conf": {}, ".yaml": {}, ".yml": {},
}

// safeTunnelTag reports whether tag is a safe, non-traversing filename component.
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

	// The kill switch works by blackholing the default route when the tunnel
	// drops; it is meaningless without a default route (there is nothing to
	// blackhole) and at runtime handleTunnelDeath already gates on both flags.
	// Reject the incoherent config so the toggle can't imply protection it will
	// never provide.
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
