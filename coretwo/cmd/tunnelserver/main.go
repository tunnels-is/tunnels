package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tunnels-is/tunnels/coretwo/pkg/crypto"
	"github.com/tunnels-is/tunnels/coretwo/pkg/logger"
)

func main() {
	// Parse command line flags
	port := flag.Int("port", 8080, "Server port")
	protocol := flag.String("protocol", "tcp", "Protocol (tcp or udp)")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	jsonLogs := flag.Bool("json-logs", false, "Output logs in JSON format")
	enableEncryption := flag.Bool("encrypt", false, "Enable encryption")
	encryptionKey := flag.String("key", "default-secure-key", "Encryption key")
	flag.Parse()

	// Initialize logger
	var level logger.Level
	switch *logLevel {
	case "debug":
		level = logger.DebugLevel
	case "info":
		level = logger.InfoLevel
	case "warn":
		level = logger.WarnLevel
	case "error":
		level = logger.ErrorLevel
	default:
		level = logger.InfoLevel
	}

	log := logger.New(level, nil, *jsonLogs)
	log.Info("Starting tunnel echo server", map[string]any{
		"port":       *port,
		"protocol":   *protocol,
		"encryption": *enableEncryption,
	})

	// Initialize encryption key if enabled
	var key *crypto.Key
	var err error
	if *enableEncryption {
		key, err = crypto.NewKey(*encryptionKey)
		if err != nil {
			log.Error("Failed to initialize encryption key", map[string]any{
				"error": err,
			})
			os.Exit(1)
		}
		log.Info("Encryption enabled")
	}

	// Set up signal handling for graceful shutdown
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	// Start server based on protocol
	switch *protocol {
	case "tcp":
		startTCPServer(log, *port, key, signalCh)
	case "udp":
		startUDPServer(log, *port, key, signalCh)
	default:
		log.Error("Unsupported protocol", map[string]any{
			"protocol": *protocol,
		})
		os.Exit(1)
	}
}

func startTCPServer(log *logger.Logger, port int, key *crypto.Key, signalCh <-chan os.Signal) {
	// Create listener
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error("Failed to create TCP listener", map[string]any{
			"error": err,
		})
		os.Exit(1)
	}
	defer listener.Close()

	log.Info("TCP server listening", map[string]any{
		"addr": addr,
	})

	// Channel to signal shutdown to all connection handlers
	shutdown := make(chan struct{})

	// Accept connections in a goroutine
	connections := make(chan net.Conn)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-shutdown:
					return // Server is shutting down
				default:
					log.Error("Failed to accept connection", map[string]any{
						"error": err,
					})
					continue
				}
			}
			connections <- conn
		}
	}()

	// Handle connections and signals
	var activeConns []net.Conn
	for {
		select {
		case conn := <-connections:
			activeConns = append(activeConns, conn)
			go handleTCPConnection(log, conn, key)
			log.Info("New connection accepted", map[string]any{
				"remote": conn.RemoteAddr().String(),
				"total":  len(activeConns),
			})

		case <-signalCh:
			log.Info("Received shutdown signal")
			close(shutdown)
			// Close all active connections
			for _, conn := range activeConns {
				conn.Close()
			}
			log.Info("Server shutdown complete")
			return
		}
	}
}

func handleTCPConnection(log *logger.Logger, conn net.Conn, key *crypto.Key) {
	defer conn.Close()
	remoteAddr := conn.RemoteAddr().String()

	log.Info("Handling connection", map[string]any{
		"remote": remoteAddr,
	})

	buffer := make([]byte, 65536) // Maximum packet size
	var totalBytes uint64
	var packetCount uint64
	statsInterval := time.NewTicker(5 * time.Second)
	defer statsInterval.Stop()

	// Connection stats
	go func() {
		for range statsInterval.C {
			// Check if connection is still valid
			if _, err := conn.Write([]byte{}); err != nil {
				return // Connection closed
			}

			log.Info("Connection statistics", map[string]any{
				"remote":        remoteAddr,
				"total_bytes":   totalBytes,
				"total_packets": packetCount,
			})
		}
	}()

	for {
		// Read from the connection
		n, err := conn.Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Error("Failed to read from connection", map[string]any{
					"remote": remoteAddr,
					"error":  err,
				})
			}
			break
		}

		data := buffer[:n]
		totalBytes += uint64(n)
		packetCount++

		log.Debug("Received packet", map[string]any{
			"remote": remoteAddr,
			"size":   n,
		})

		// Process data if encryption is enabled
		if key != nil {
			// Decrypt incoming data
			decrypted, err := key.Decrypt(data)
			if err != nil {
				log.Error("Failed to decrypt data", map[string]any{
					"remote": remoteAddr,
					"error":  err,
				})
				continue
			}

			log.Debug("Decrypted packet", map[string]any{
				"remote": remoteAddr,
				"size":   len(decrypted),
			})

			// Echo the data after re-encrypting
			encrypted, err := key.Encrypt(decrypted)
			if err != nil {
				log.Error("Failed to encrypt data", map[string]any{
					"remote": remoteAddr,
					"error":  err,
				})
				continue
			}

			data = encrypted
		}

		// Echo the data back
		_, err = conn.Write(data)
		if err != nil {
			log.Error("Failed to write to connection", map[string]any{
				"remote": remoteAddr,
				"error":  err,
			})
			break
		}
	}

	log.Info("Connection closed", map[string]any{
		"remote": remoteAddr,
	})
}

func startUDPServer(log *logger.Logger, port int, key *crypto.Key, signalCh <-chan os.Signal) {
	// Create UDP listener
	addr := fmt.Sprintf(":%d", port)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Error("Failed to resolve UDP address", map[string]any{
			"error": err,
		})
		os.Exit(1)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Error("Failed to create UDP listener", map[string]any{
			"error": err,
		})
		os.Exit(1)
	}
	defer conn.Close()

	log.Info("UDP server listening", map[string]any{
		"addr": addr,
	})

	// Handle UDP packets
	buffer := make([]byte, 65536) // Maximum UDP packet size
	var totalBytes uint64
	var packetCount uint64
	statsInterval := time.NewTicker(5 * time.Second)
	defer statsInterval.Stop()

	// Stats reporting goroutine
	go func() {
		for range statsInterval.C {
			log.Info("UDP statistics", map[string]any{
				"total_bytes":   totalBytes,
				"total_packets": packetCount,
			})
		}
	}()

	// Signal handling goroutine
	go func() {
		<-signalCh
		log.Info("Received shutdown signal")
		conn.Close()
		log.Info("Server shutdown complete")
		os.Exit(0)
	}()

	// Main packet handling loop
	for {
		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Error("Failed to read UDP packet", map[string]any{
				"error": err,
			})
			break
		}

		data := buffer[:n]
		totalBytes += uint64(n)
		packetCount++

		log.Debug("Received UDP packet", map[string]any{
			"remote": addr.String(),
			"size":   n,
		})

		// Process data if encryption is enabled
		if key != nil {
			// Decrypt incoming data
			decrypted, err := key.Decrypt(data)
			if err != nil {
				log.Error("Failed to decrypt UDP data", map[string]any{
					"remote": addr.String(),
					"error":  err,
				})
				continue
			}

			log.Debug("Decrypted UDP packet", map[string]any{
				"remote": addr.String(),
				"size":   len(decrypted),
			})

			// Echo the data after re-encrypting
			encrypted, err := key.Encrypt(decrypted)
			if err != nil {
				log.Error("Failed to encrypt UDP data", map[string]any{
					"remote": addr.String(),
					"error":  err,
				})
				continue
			}

			data = encrypted
		}

		// Echo the data back
		_, err = conn.WriteToUDP(data, addr)
		if err != nil {
			log.Error("Failed to write UDP packet", map[string]any{
				"remote": addr.String(),
				"error":  err,
			})
			continue
		}
	}
}
