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

func SendRequestToURL(tc *tls.Config, method string, url string, data any, timeoutMS int, skipVerify bool) ([]byte, int, error) {
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
	if method == "POST" {
		req, err = http.NewRequest(method, url, bytes.NewBuffer(body))
	} else if method == "GET" {
		req, err = http.NewRequest(method, url, nil)
	} else {
		return nil, 400, errors.New("method not supported:" + method)
	}

	if err != nil {
		return nil, 400, err
	}

	req.Header.Add("Content-Type", "application/json")

	client := http.Client{Timeout: time.Duration(timeoutMS) * time.Millisecond}
	if tc != nil {
		client.Transport = &http.Transport{
			TLSClientConfig: tc,
		}
	} else {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion:         tls.VersionTLS13,
				CurvePreferences:   []tls.CurveID{tls.X25519MLKEM768},
				InsecureSkipVerify: !skipVerify,
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

func ForwardToController(FR *FORWARD_REQUEST) (any, int) {
	defer RecoverAndLog()

	// make sure api.tunnels.is is always secure
	if strings.Contains(FR.Server.Host, "api.tunnels.is") {
		FR.Server.ValidateCertificate = true
	}

	url := FR.Server.GetURL(FR.Path)
	responseBytes, code, err := SendRequestToURL(
		nil,
		FR.Method,
		url,
		FR.JSONData,
		FR.Timeout,
		FR.Server.ValidateCertificate,
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
			ERROR("Could not parse response data: ", err)
			er.Error = "Unable to open response from controller"
			return er, code
		}
	}

	return respObj, code
}

var AZ_CHAR_CHECK = regexp.MustCompile(`^[a-zA-Z0-9]*$`)

func validateTunnelMeta(tun *TunnelMETA, oldTag string) (err []string) {
	ifnamemap := make(map[string]struct{})
	ifFail := AZ_CHAR_CHECK.MatchString(tun.IFName)
	if !ifFail {
		err = append(err, "tunnel names can only contain a-z A-Z 0-9, invalid name: "+tun.IFName)
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

	// this is windows only
	errx := ValidateAdapterID(tun)
	if errx != nil {
		err = append(err, errx.Error())
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
