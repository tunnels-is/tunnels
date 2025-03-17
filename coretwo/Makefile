.PHONY: all build test clean keyutil tunneltest tunnelserver

# Default target
all: build

# Build all binaries
build: tunnels keyutil tunneltest tunnelserver

# Build the main tunnels application
tunnels:
	go build -o bin/tunnels ./cmd/tunnels

# Build the key utility
keyutil:
	go build -o bin/keyutil ./cmd/keyutil

# Build the tunnel test client
tunneltest:
	go build -o bin/tunneltest ./cmd/tunneltest

# Build the tunnel server
tunnelserver:
	go build -o bin/tunnelserver ./cmd/tunnelserver

# Run all tests
test:
	go test -v ./...

# Run the tunnel test with encryption
test-encrypted: tunneltest tunnelserver
	@echo "Starting encrypted tunnel test..."
	@KEY=$$(./bin/keyutil -generate | cut -d: -f2) && \
	echo "Using key: $$KEY" && \
	./bin/tunnelserver -port 8080 -protocol tcp -encrypt -key "$$KEY" -log-level debug & \
	SERVER_PID=$$! && \
	sleep 1 && \
	./bin/tunneltest -server 127.0.0.1 -port 8080 -protocol tcp -encrypt -key "$$KEY" -log-level debug & \
	CLIENT_PID=$$! && \
	echo "Test running... Press Enter to stop" && \
	read && \
	kill $$SERVER_PID $$CLIENT_PID

# Generate a new encryption key
gen-key:
	./bin/keyutil -generate

# Test an encryption key
test-key:
	@read -p "Enter key to test: " KEY && \
	./bin/keyutil -test "$$KEY" -message "Test encryption with this message"

# Clean built binaries
clean:
	rm -rf bin/

# Install binaries to /usr/local/bin (requires sudo)
install: build
	sudo cp bin/tunnels /usr/local/bin/
	sudo cp bin/keyutil /usr/local/bin/
	sudo cp bin/tunneltest /usr/local/bin/
	sudo cp bin/tunnelserver /usr/local/bin/

# Create necessary directories
bin:
	mkdir -p bin 