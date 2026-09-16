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
	"sync"
	"time"
)

// errControllerRedirect is returned when the controller answers with a
// redirect. Redirects are refused so X-Device-Token is never re-sent to
// a different Location (Go does not strip custom auth headers).
var errControllerRedirect = errors.New("refusing to follow controller redirect (X-Device-Token must not leave the configured controller URL)")

const defaultMaxControllerResponseBytes int64 = 50 << 20 // 50 MiB

var (
	maxControllerResponseBytes    = defaultMaxControllerResponseBytes
	errControllerResponseTooLarge = errors.New("controller response exceeds size limit")
)

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

	client, tlsErr := newControllerHTTPClient(tc, timeoutMS, validateCert, certPath, req.URL.Host)
	if tlsErr != nil {
		return nil, 400, tlsErr
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
	if resp.Body == nil {
		return nil, resp.StatusCode, nil
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxControllerResponseBytes+1)
	respBodyBytes, err := io.ReadAll(limited)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if int64(len(respBodyBytes)) > maxControllerResponseBytes {
		return nil, resp.StatusCode, errControllerResponseTooLarge
	}

	return respBodyBytes, resp.StatusCode, nil
}

func newControllerHTTPClient(tc *tls.Config, timeoutMS int, validateCert bool, certPath, host string) (*http.Client, error) {
	client := &http.Client{
		Timeout: time.Duration(timeoutMS) * time.Millisecond,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errControllerRedirect
		},
	}
	if tc != nil {
		client.Transport = &http.Transport{
			TLSClientConfig: tc,
		}
		return client, nil
	}
	if !validateCert {
		warnInsecureHost(host)
	}
	tlsCfg, tlsErr := tlsConfigForController(validateCert, certPath)
	if tlsErr != nil {
		return nil, tlsErr
	}
	client.Transport = &http.Transport{
		TLSClientConfig: tlsCfg,
	}
	return client, nil
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

func ForwardToController(FR *ForwardRequest) (any, int) {
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
