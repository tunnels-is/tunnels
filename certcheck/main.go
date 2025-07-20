package main

import (
	"bytes"
	"crypto/md5"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

var (
	tag     = "(cert verification) "
	role    = ""
	webhook = ""
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		panic(err)
	}
	role = os.Getenv("ROLE")
	webhook = os.Getenv("WEBHOOK")
	var prevsum [16]byte
	start := time.Now()
	isFirst := true
	SendDiscordWebhook(webhook, fmt.Sprintf("Starting certificate scanner for %s %s", os.Args[1], role))
	for {

		sum, code, err := ResolveAndRequest(os.Args[1])
		if err != nil {
			SendDiscordWebhook(webhook, fmt.Sprintf("%s %s %s", role, tag, err.Error()))
			continue
		}
		if code != 200 {
			SendDiscordWebhook(webhook, fmt.Sprintf("%s %s Invalid response code from controller", role, tag))
		}
		if prevsum != sum && !isFirst {
			SendDiscordWebhook(webhook, fmt.Sprintf("%s %s CHECKSUM MISSMATCH (CONTACT SUPPORT ASAP)", role, tag))
		}
		prevsum = sum
		if time.Since(start).Hours() > 1 {
			SendDiscordWebhook(webhook, fmt.Sprintf(" %s %s%x > %d", role, tag, sum, code))
			start = time.Now()
		}
		isFirst = false
		time.Sleep(10 * time.Second)
	}
}

func ResolveAndRequest(domain string) ([16]byte, int, error) {
	var err error
	ips, err := net.LookupIP(domain)
	if err != nil {
		return [16]byte{}, 0, err
	}

	if len(ips) == 0 {
		return [16]byte{}, 0, fmt.Errorf("no ips found for domain")
	}

	preHashString := ""
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return nil
			}

			for i, rawCert := range rawCerts {
				cert, err := x509.ParseCertificate(rawCert)
				if err != nil {
					return fmt.Errorf("error parsing certificate %d: %v", i, err)
				}

				preHashString += cert.Subject.CommonName
				preHashString += cert.Issuer.CommonName
				preHashString += cert.Issuer.SerialNumber
				preHashString += fmt.Sprintf("%d", cert.SerialNumber)
				preHashString += cert.NotBefore.Format(time.RFC1123)
				preHashString += cert.NotAfter.Format(time.RFC1123)
				preHashString += strings.Join(cert.DNSNames, ", ")

				if len(cert.IPAddresses) > 0 {
					for _, ip := range cert.IPAddresses {
						preHashString += ip.String()
					}
				}
			}
			return nil
		},
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	client := &http.Client{
		Transport: transport,
	}

	url := fmt.Sprintf("https://%s", domain)
	resp, err := client.Get(url)
	if err != nil {
		return [16]byte{}, 0, err
	}
	defer resp.Body.Close()
	sum := md5.Sum([]byte(preHashString))
	return sum, resp.StatusCode, nil
}

func SendDiscordWebhook(webhookURL string, message string) error {
	type Payload struct {
		Content string `json:"content"`
	}

	payload := Payload{Content: message}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("discord responded with status: %d", resp.StatusCode)
	}

	return nil
}
