package main

import (
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall" // Keep for signal handling

	"github.com/songgao/water" // Import the water library
	"golang.org/x/sys/unix"    // Keep for socket options
)

const (
	// VPN Internal Network Config
	VPN_SERVER_IP = "10.9.0.1"
	// VPN_CLIENT_IP_EXPECTED = "10.9.0.2" // Less relevant now, client configures its IP
	VPN_SUBNET           = "10.9.0.0/24"
	TUN_INTERFACE_NAME   = "tunsrvw0" // Suggested name for water library
	TUN_INTERFACE_C_NAME = "tuncliw0" // Suggested name for water library

	// UDP Listener Config
	SERVER_LISTEN_ADDR = "0.0.0.0:1195" // Use a consistent port

	// Packet Handling

	VPN_CLIENT_IP = "10.9.0.2" // IP for the client's TUN interface

	// UDP Connection Config
	UDP_SOCKET_BUFFER = 10 * 1024 * 1024 // 10 MB buffer

	// Packet Handling
	PACKET_BUFFER_SIZE = 2048 // MTU + headroom (match server)
	TUN_MTU            = 1400 // Must match server TUN MTU!
)

func main() {
	if os.Args[1] == "client" {
		client(os.Args[2])
	} else {
		server()
	}

}

// Buffer pool for packet buffers
var packetPool = sync.Pool{
	New: func() interface{} {
		// Return a pointer to a byte slice to avoid pool copying the slice header
		b := make([]byte, PACKET_BUFFER_SIZE)
		return &b
	},
}

// runCommand - Helper to execute shell commands
func runCommand(cmdStr string) error {
	log.Printf("RUN CMD: %s", cmdStr)
	cmd := exec.Command("sh", "-c", cmdStr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("CMD ERR (%s): %s\nOutput:\n%s", cmdStr, err, string(output))
		return err
	}
	// log.Printf("CMD OUT:\n%s", string(output)) // Can be verbose
	return nil
}

func server() {
	defaultIface := "enp1s0"
	log.Println("Starting GoVPN Server (water library)...")

	// --- System Configuration (Requires Root) ---
	runCommand("sudo sysctl -w net.core.rmem_max=" + strconv.Itoa(UDP_SOCKET_BUFFER*2))
	runCommand("sudo sysctl -w net.core.wmem_max=" + strconv.Itoa(UDP_SOCKET_BUFFER*2))

	// --- Create TUN Interface using water ---
	config := water.Config{
		DeviceType: water.TUN,
		PlatformSpecificParams: water.PlatformSpecificParams{
			Name: TUN_INTERFACE_NAME, // Suggest name (OS might override)
		},
	}
	iface, err := water.New(config)
	if err != nil {
		log.Fatalf("Failed to create TUN interface using water: %v", err)
	}
	defer iface.Close() // Ensure TUN interface is closed on exit

	tunName := iface.Name() // Get the actual assigned interface name
	log.Printf("TUN Interface '%s' created using water library.", tunName)

	// --- Configure TUN Interface ---
	err = runCommand("sudo ip addr add " + VPN_SERVER_IP + "/24 dev " + tunName)
	if err != nil {
		log.Fatalf("Failed to set IP address for %s: %v", tunName, err)
	}
	err = runCommand("sudo ip link set dev " + tunName + " mtu " + strconv.Itoa(TUN_MTU) + " up")
	if err != nil {
		log.Fatalf("Failed to bring up interface %s: %v", tunName, err)
	}

	// --- Enable IP Forwarding & NAT (Example) ---
	err = runCommand("sudo sysctl -w net.ipv4.ip_forward=1")
	if err != nil {
		log.Printf("Warning: Failed to enable IP forwarding: %v", err)
	}
	log.Printf("Assuming default interface is %s for NAT setup. CHANGE IF NECESSARY.", defaultIface)
	err = runCommand("sudo iptables -t nat -A POSTROUTING -s " + VPN_SUBNET + " -o " + defaultIface + " -j MASQUERADE")
	if err != nil {
		log.Printf("Warning: Failed to set up NAT using iptables: %v.", err)
	}
	mssValue := TUN_MTU - 40 // Typical TCP/IP overhead
	err = runCommand("sudo iptables -t mangle -A FORWARD -p tcp --tcp-flags SYN,RST SYN -s " + VPN_SUBNET + " -j TCPMSS --set-mss " + strconv.Itoa(mssValue))
	if err != nil {
		log.Printf("Warning: Failed to set up TCP MSS clamping: %v.", err)
	}

	log.Printf("Interface '%s' configured and up.", tunName)

	// --- Setup UDP Listener ---
	listenAddr, err := net.ResolveUDPAddr("udp", SERVER_LISTEN_ADDR)
	if err != nil {
		log.Fatalf("Failed to resolve UDP address: %v", err)
	}
	udpConn, err := net.ListenUDP("udp", listenAddr)
	if err != nil {
		log.Fatalf("Failed to listen on UDP port: %v", err)
	}
	defer udpConn.Close()

	// --- Optimize UDP Socket ---
	rawConn, err := udpConn.SyscallConn()
	if err != nil {
		log.Fatalf("Failed to get raw UDP connection: %v", err)
	}
	err = rawConn.Control(func(fd uintptr) {
		err1 := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_RCVBUF, UDP_SOCKET_BUFFER)
		if err1 != nil {
			log.Printf("Warning: Failed to set SO_RCVBUF: %v", err1)
		}
		err2 := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_SNDBUF, UDP_SOCKET_BUFFER)
		if err2 != nil {
			log.Printf("Warning: Failed to set SO_SNDBUF: %v", err2)
		}
	})
	if err != nil {
		log.Fatalf("Failed rawConn.Control: %v", err)
	}
	log.Printf("UDP Socket buffers requested: %d bytes", UDP_SOCKET_BUFFER)
	log.Printf("Listening on %s", SERVER_LISTEN_ADDR)

	// --- Packet Handling ---
	var clientAddr *net.UDPAddr
	var clientAddrMutex sync.RWMutex

	// Goroutine: Read from TUN -> Write to UDP
	go func() {
		for {
			bufferPtr := packetPool.Get().(*[]byte)
			buffer := *bufferPtr // Dereference pointer to get the slice

			// Use iface.Read() from the water library
			n, err := iface.Read(buffer)
			if err != nil {
				packetPool.Put(bufferPtr)
				log.Printf("Error reading from TUN interface %s: %v", tunName, err)
				// water usually returns io.EOF or similar on close
				if err.Error() == "EOF" || err.Error() == "read /dev/net/tun: file already closed" {
					log.Println("TUN interface closed, exiting read goroutine.")
					return
				}
				continue
			}

			clientAddrMutex.RLock()
			currentClientAddr := clientAddr
			clientAddrMutex.RUnlock()

			if currentClientAddr != nil && n > 0 {
				_, err = udpConn.WriteToUDP(buffer[:n], currentClientAddr)
				if err != nil {
					log.Printf("Error writing to UDP %s: %v", currentClientAddr.String(), err)
				}
			}
			packetPool.Put(bufferPtr) // Return buffer to pool
		}
	}()

	// Goroutine: Read from UDP -> Write to TUN
	go func() {
		for {
			bufferPtr := packetPool.Get().(*[]byte)
			buffer := *bufferPtr

			n, remoteAddr, err := udpConn.ReadFromUDP(buffer)
			if err != nil {
				packetPool.Put(bufferPtr)
				log.Printf("Error reading from UDP: %v", err)
				if netErr, ok := err.(net.Error); ok && !netErr.Timeout() && !netErr.Temporary() {
					log.Println("Non-recoverable UDP read error, exiting write goroutine.")
					return
				}
				continue
			}

			// Update client address
			clientAddrMutex.Lock()
			if clientAddr == nil || !clientAddr.IP.Equal(remoteAddr.IP) || clientAddr.Port != remoteAddr.Port {
				log.Printf("Client connected/updated from %s", remoteAddr.String())
				clientAddr = remoteAddr
			}
			clientAddrMutex.Unlock()

			if n > 0 {
				// Use iface.Write() from the water library
				_, err = iface.Write(buffer[:n])
				if err != nil {
					// Check if TUN interface was closed
					if err.Error() == "write /dev/net/tun: file already closed" {
						packetPool.Put(bufferPtr)
						log.Println("TUN interface closed, exiting write goroutine.")
						return
					}
					log.Printf("Error writing to TUN interface %s: %v", tunName, err)
				}
			}
			packetPool.Put(bufferPtr) // Return buffer to pool
		}
	}()

	// --- Wait for Shutdown Signal ---
	log.Println("Server running. Press Ctrl+C to exit.")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down server...")
	// Cleanup happens via defers
	// Optional: Remove iptables rules if desired
}

func client(ip string) {
	log.Println("Starting GoVPN Client (water library)...")
	// --- Sanity Check Server Address ---

	// --- System Configuration (Requires Root) ---
	runCommand("sudo sysctl -w net.core.rmem_max=" + strconv.Itoa(UDP_SOCKET_BUFFER*2))
	runCommand("sudo sysctl -w net.core.wmem_max=" + strconv.Itoa(UDP_SOCKET_BUFFER*2))

	// --- Create TUN Interface using water ---
	config := water.Config{
		DeviceType: water.TUN,
		PlatformSpecificParams: water.PlatformSpecificParams{
			Name: TUN_INTERFACE_NAME, // Suggest name
		},
	}
	iface, err := water.New(config)
	if err != nil {
		log.Fatalf("Failed to create TUN interface using water: %v", err)
	}
	defer iface.Close()

	tunName := iface.Name() // Get the actual assigned interface name
	log.Printf("TUN Interface '%s' created using water library.", tunName)

	// --- Configure TUN Interface ---
	err = runCommand("sudo ip addr add " + VPN_CLIENT_IP + "/24 dev " + tunName)
	if err != nil {
		log.Fatalf("Failed to set IP address for %s: %v", tunName, err)
	}
	err = runCommand("sudo ip link set dev " + tunName + " mtu " + strconv.Itoa(TUN_MTU) + " up")
	if err != nil {
		log.Fatalf("Failed to bring up interface %s: %v", tunName, err)
	}

	// --- Configure Routing ---
	log.Println("Configuring routes...")
	// Basic route for the VPN subnet itself
	err = runCommand("sudo ip route add " + VPN_SUBNET + " dev " + tunName + " src " + VPN_CLIENT_IP)
	if err != nil {
		log.Printf("Warning: Failed to add route for VPN subnet %s: %v", VPN_SUBNET, err)
	}
	// Add more routes here if needed (e.g., for full tunnel - see previous examples for logic)
	// Remember to add a specific route for SERVER_PUBLIC_ADDR via the original gateway BEFORE
	// changing the default route for full tunnel.

	log.Printf("Interface '%s' configured and up.", tunName)
	log.Println("!!! Verify routes using 'ip route' command !!!")

	// --- Connect to Server via UDP ---
	serverIP := ip + ":1195"
	serverAddr, err := net.ResolveUDPAddr("udp", serverIP)
	if err != nil {
		log.Fatalf("Failed to resolve server address %s: %v", serverIP, err)
	}
	udpConn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		log.Fatalf("Failed to connect to server %s: %v", serverIP, err)
	}
	defer udpConn.Close()

	// --- Optimize UDP Socket ---
	rawConn, err := udpConn.SyscallConn()
	if err != nil {
		log.Fatalf("Failed to get raw UDP connection: %v", err)
	}
	err = rawConn.Control(func(fd uintptr) {
		err1 := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_RCVBUF, UDP_SOCKET_BUFFER)
		if err1 != nil {
			log.Printf("Warning: Failed to set SO_RCVBUF: %v", err1)
		}
		err2 := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_SNDBUF, UDP_SOCKET_BUFFER)
		if err2 != nil {
			log.Printf("Warning: Failed to set SO_SNDBUF: %v", err2)
		}
	})
	if err != nil {
		log.Fatalf("Failed rawConn.Control: %v", err)
	}
	log.Printf("UDP Socket buffers requested: %d bytes", UDP_SOCKET_BUFFER)
	log.Printf("Connected to server at %s", serverIP)

	// --- Packet Handling ---

	// Goroutine: Read from TUN -> Write to UDP
	go func() {
		for {
			bufferPtr := packetPool.Get().(*[]byte)
			buffer := *bufferPtr

			// Use iface.Read() from water library
			n, err := iface.Read(buffer)
			if err != nil {
				packetPool.Put(bufferPtr)
				log.Printf("Error reading from TUN interface %s: %v", tunName, err)
				if err.Error() == "EOF" || err.Error() == "read /dev/net/tun: file already closed" {
					log.Println("TUN interface closed, exiting read goroutine.")
					// Optionally trigger connection closure or exit
					udpConn.Close() // Close UDP connection if TUN dies
					return
				}
				continue
			}

			if n > 0 {
				_, err = udpConn.Write(buffer[:n])
				if err != nil {
					log.Printf("Error writing to UDP: %v", err)
					if netErr, ok := err.(net.Error); ok && !netErr.Timeout() && !netErr.Temporary() {
						packetPool.Put(bufferPtr)
						log.Println("Non-recoverable UDP write error, exiting read loop.")
						return
					}
				}
			}
			packetPool.Put(bufferPtr)
		}
	}()

	// Goroutine: Read from UDP -> Write to TUN
	go func() {
		for {
			bufferPtr := packetPool.Get().(*[]byte)
			buffer := *bufferPtr

			n, err := udpConn.Read(buffer)
			if err != nil {
				packetPool.Put(bufferPtr)
				log.Printf("Error reading from UDP: %v", err)
				if netErr, ok := err.(net.Error); ok && !netErr.Timeout() && !netErr.Temporary() {
					log.Println("Non-recoverable UDP read error, exiting write loop.")
					// Optionally trigger TUN closure or exit
					iface.Close() // Close TUN if UDP dies
					return
				}
				// If UDP read error due to connection close, exit loop
				if strings.Contains(err.Error(), "use of closed network connection") {
					log.Println("UDP connection closed, exiting write loop.")
					return
				}
				continue
			}

			if n > 0 {
				// Use iface.Write() from water library
				_, err = iface.Write(buffer[:n])
				if err != nil {
					packetPool.Put(bufferPtr) // Return buffer even on error
					// Check if TUN interface was closed
					if err.Error() == "write /dev/net/tun: file already closed" {
						log.Println("TUN interface closed, exiting write goroutine.")
						return
					}
					log.Printf("Error writing to TUN interface %s: %v", tunName, err)
				} else {
					// Only return buffer on successful write or non-fatal error in the UDP read case
					packetPool.Put(bufferPtr)
				}
			} else {
				// If n is 0, still return the buffer
				packetPool.Put(bufferPtr)
			}
		}
	}()

	// --- Wait for Shutdown Signal ---
	log.Println("Client running. Press Ctrl+C to exit.")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down client...")
	// Cleanup happens via defers
	// Optional: Restore routes if necessary
}
