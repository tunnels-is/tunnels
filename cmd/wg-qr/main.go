package main

import (
	"flag"
	"fmt"
	"os"

	qrcode "github.com/skip2/go-qrcode"
	"github.com/mdp/qrterminal/v3"
)

func main() {
	privKey    := flag.String("privkey",    "", "client private key (base64)")
	address    := flag.String("address",    "", "client VPN address, e.g. 10.1.0.2/32")
	dns        := flag.String("dns",        "1.1.1.1", "DNS server")
	serverPub  := flag.String("serverpub",  "", "server WireGuard public key (base64)")
	endpoint   := flag.String("endpoint",   "", "server endpoint, e.g. 1.2.3.4:442")
	allowedIPs := flag.String("allowed",    "0.0.0.0/0", "AllowedIPs")
	keepalive  := flag.Int("keepalive",     25, "PersistentKeepalive in seconds")
	out        := flag.String("out",        "wg-client.png", "output PNG filename")
	flag.Parse()

	if *privKey == "" || *address == "" || *serverPub == "" || *endpoint == "" {
		fmt.Fprintln(os.Stderr, "usage: wg-qr -privkey <key> -address <ip/mask> -serverpub <key> -endpoint <host:port>")
		fmt.Fprintln(os.Stderr, "optional: -dns -allowed -keepalive -out")
		flag.PrintDefaults()
		os.Exit(1)
	}

	conf := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s
DNS = %s

[Peer]
PublicKey = %s
Endpoint = %s
AllowedIPs = %s
PersistentKeepalive = %d
`, *privKey, *address, *dns, *serverPub, *endpoint, *allowedIPs, *keepalive)

	fmt.Println("--- WireGuard config ---")
	fmt.Print(conf)
	fmt.Println("------------------------")

	qrterminal.GenerateHalfBlock(conf, qrterminal.L, os.Stdout)

	if err := qrcode.WriteFile(conf, qrcode.High, 512, *out); err != nil {
		fmt.Fprintln(os.Stderr, "error generating QR code:", err)
		os.Exit(1)
	}

	fmt.Println("QR code also saved to:", *out)
}
