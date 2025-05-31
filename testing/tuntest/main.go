//go:build windows
// +build windows

package main

import (
	"log"
	"net"
	"sync"
	"syscall"
	"unsafe"
)

// --- Windows API Constants and Structs (mimicking parts of x/sys/windows) ---
const (
	FILE_ATTRIBUTE_SYSTEM = 0x4
	FILE_FLAG_OVERLAPPED  = 0x40000000
	GENERIC_READ          = 0x80000000
	GENERIC_WRITE         = 0x40000000
	OPEN_EXISTING         = 3
	FILE_SHARE_READ       = 0x00000001
	FILE_SHARE_WRITE      = 0x00000002

	// IOCTL codes for TAP-Windows driver (from OpenVPN/tap-windows6 source)
	// These are specific to tap-windows6 driver!
	TAP_CONTROL_CODE = 0x000000000000000080000000 | 0x000000000000000000000000 | 0x000000000000000000000004
	//CTL_CODE(FILE_DEVICE_UNKNOWN, 0x9, METHOD_BUFFERED, FILE_ANY_ACCESS)
	TAP_WIN_IOCTL_GET_MAC = (TAP_CONTROL_CODE | (9 << 2)) // Get MAC address
	//CTL_CODE(FILE_DEVICE_UNKNOWN, 0xA, METHOD_BUFFERED, FILE_ANY_ACCESS)
	TAP_WIN_IOCTL_GET_VERSION = (TAP_CONTROL_CODE | (10 << 2)) // Get driver version
	//CTL_CODE(FILE_DEVICE_UNKNOWN, 0xB, METHOD_BUFFERED, FILE_ANY_ACCESS)
	TAP_WIN_IOCTL_GET_MTU = (TAP_CONTROL_CODE | (11 << 2)) // Get MTU
	//CTL_CODE(FILE_DEVICE_UNKNOWN, 0x11, METHOD_BUFFERED, FILE_ANY_ACCESS)
	TAP_WIN_IOCTL_CONFIG_TUN = (TAP_CONTROL_CODE | (17 << 2)) // Configure as TUN device (set local/remote IP, mask)
	//CTL_CODE(FILE_DEVICE_UNKNOWN, 0x12, METHOD_BUFFERED, FILE_ANY_ACCESS)
	TAP_WIN_IOCTL_SET_MEDIA_STATUS = (TAP_CONTROL_CODE | (18 << 2)) // Set media status (connected/disconnected)
	//CTL_CODE(FILE_DEVICE_UNKNOWN, 0x13, METHOD_BUFFERED, FILE_ANY_ACCESS)
	TAP_WIN_IOCTL_CONFIG_DHCP_MASQ = (TAP_CONTROL_CODE | (19 << 2)) // DHCP masquerade
	//CTL_CODE(FILE_DEVICE_UNKNOWN, 0x14, METHOD_BUFFERED, FILE_ANY_ACCESS)
	TAP_WIN_IOCTL_GET_HANDLES = (TAP_CONTROL_CODE | (20 << 2)) // Get read/write events handles

	// Media Status
	NET_IF_MEDIA_CONNECTED    = 1
	NET_IF_MEDIA_DISCONNECTED = 2
)

// Overlapped struct for asynchronous I/O (not strictly used in this blocking example, but common)
type OVERLAPPED struct {
	Internal     uintptr
	InternalHigh uintptr
	Offset       uint32
	OffsetHigh   uint32
	HEvent       syscall.Handle
}

// TAP_WIN_IOCTL_CONFIG_TUN input structure (ip, ip_netmask)
// It's usually 3 DWORDS: local IP, remote IP (gateway), netmask
type TapTunConfig struct {
	LocalIP  uint32
	RemoteIP uint32
	Netmask  uint32
}

// --- Windows API Calls (via syscall) ---
var (
	modkernel32         = syscall.NewLazyDLL("kernel32.dll")
	procCreateFileW     = modkernel32.NewProc("CreateFileW")
	procCloseHandle     = modkernel32.NewProc("CloseHandle")
	procReadFile        = modkernel32.NewProc("ReadFile")
	procWriteFile       = modkernel32.NewProc("WriteFile")
	procDeviceIoControl = modkernel32.NewProc("DeviceIoControl")
)

func _CreateFile(
	name *uint16,
	access uint32,
	mode uint32,
	sa *syscall.SecurityAttributes,
	createmode uint32,
	attrs uint32,
	templatefile syscall.Handle) (handle syscall.Handle, err error) {
	r0, _, e1 := syscall.Syscall9(procCreateFileW.Addr(), 7, uintptr(unsafe.Pointer(name)), uintptr(access), uintptr(mode), uintptr(unsafe.Pointer(sa)), uintptr(createmode), uintptr(attrs), uintptr(templatefile), 0, 0)
	handle = syscall.Handle(r0)
	if handle == syscall.InvalidHandle {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL // Or a more specific error if known
		}
	}
	return
}

func _CloseHandle(handle syscall.Handle) (err error) {
	r1, _, e1 := syscall.Syscall(procCloseHandle.Addr(), 1, uintptr(handle), 0, 0)
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func _ReadFile(
	handle syscall.Handle,
	buf *byte,
	nBytes uint32,
	bytesRead *uint32,
	overlapped *OVERLAPPED) (err error) {
	r1, _, e1 := syscall.Syscall6(procReadFile.Addr(), 5, uintptr(handle), uintptr(unsafe.Pointer(buf)), uintptr(nBytes), uintptr(unsafe.Pointer(bytesRead)), uintptr(unsafe.Pointer(overlapped)), 0)
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func _WriteFile(
	handle syscall.Handle,
	buf *byte,
	nBytes uint32,
	bytesWritten *uint32,
	overlapped *OVERLAPPED) (err error) {
	r1, _, e1 := syscall.Syscall6(procWriteFile.Addr(), 5, uintptr(handle), uintptr(unsafe.Pointer(buf)), uintptr(nBytes), uintptr(unsafe.Pointer(bytesWritten)), uintptr(unsafe.Pointer(overlapped)), 0)
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func _DeviceIoControl(
	handle syscall.Handle,
	ioctlCode uint32,
	inBuf uintptr,
	inBufLen uint32,
	outBuf uintptr,
	outBufLen uint32,
	bytesReturned *uint32,
	overlapped *OVERLAPPED) (err error) {
	r1, _, e1 := syscall.Syscall9(procDeviceIoControl.Addr(), 8,
		uintptr(handle),
		uintptr(ioctlCode),
		inBuf,
		uintptr(inBufLen),
		outBuf,
		uintptr(outBufLen),
		uintptr(unsafe.Pointer(bytesReturned)),
		uintptr(unsafe.Pointer(overlapped)),
		0)
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

// Convert net.IP to uint32 (IPv4)
func ipToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func main() {
	// IMPORTANT: You MUST run this program as Administrator.
	// IMPORTANT: The TAP-Windows driver MUST be installed.
	// IMPORTANT: You need to find the correct device path manually.
	// The path often looks like "\\.\Global\OemTap0901" or "\\.\Global\{GUID}"
	// You can use `netsh interface show interface` to see adapter names, then try to guess.
	// Or, temporarily use `songgao/water` to find it, then use that path here.
	// For example, if `water` says "Ethernet 2", you might have to check device manager
	// or registry to figure out its GUID and thus the \\.\Global\{GUID} path.
	// For this example, we'll try a common one, but it might not be yours.
	devicePath := `\\.\Global\OemTap0901` // Common path for the first TAP device

	log.Printf("Attempting to open TAP device at: %s", devicePath)

	pathPtr, err := syscall.UTF16PtrFromString(devicePath)
	if err != nil {
		log.Fatalf("Failed to convert device path: %v", err)
	}

	handle, err := _CreateFile(
		pathPtr,
		GENERIC_READ|GENERIC_WRITE,
		FILE_SHARE_READ|FILE_SHARE_WRITE,
		nil, // No security attributes
		OPEN_EXISTING,
		FILE_ATTRIBUTE_SYSTEM, // Essential for opening driver handles
		0)                     // No template file
	if err != nil {
		log.Fatalf("Failed to open TAP device '%s'. Ensure driver is installed and you are running as Administrator: %v", devicePath, err)
	}
	log.Printf("Successfully opened TAP device: %s (Handle: %d)", devicePath, handle)
	defer func() {
		_CloseHandle(handle)
		log.Println("Closed TAP device handle.")
	}()

	var bytesReturned uint32

	// --- 1. Set Media Status to Connected ---
	mediaStatus := uint32(NET_IF_MEDIA_CONNECTED)
	err = _DeviceIoControl(
		handle,
		TAP_WIN_IOCTL_SET_MEDIA_STATUS,
		uintptr(unsafe.Pointer(&mediaStatus)),
		uint32(unsafe.Sizeof(mediaStatus)),
		0, 0, // No output buffer
		&bytesReturned,
		nil)
	if err != nil {
		log.Fatalf("Failed to set media status to connected: %v", err)
	}
	log.Println("Set media status to connected.")

	// --- 2. Configure as TUN device (assign virtual IPs and Netmask to the driver) ---
	// This tells the driver what IP range it should consider "local" to the tunnel.
	// These are NOT the IP address configured on the Windows adapter, but internal to the driver.
	// The driver uses these to filter traffic and build IP headers if it's operating as a TUN.
	// `water` library sets these to 0,0,0 and relies on Windows' actual IP config.
	// Let's also set them to 0,0,0 to let Windows handle IP configuration.
	// If you want the driver to strictly enforce a tunnel (e.g. for a simple bridge),
	// you'd set these to local TUN IP, gateway TUN IP, and netmask for the virtual network.
	tunConfig := TapTunConfig{
		LocalIP:  0, // Set to 0 to let Windows manage actual IP configuration
		RemoteIP: 0,
		Netmask:  0,
	}

	err = _DeviceIoControl(
		handle,
		TAP_WIN_IOCTL_CONFIG_TUN,
		uintptr(unsafe.Pointer(&tunConfig)),
		uint32(unsafe.Sizeof(tunConfig)),
		0, 0, // No output buffer
		&bytesReturned,
		nil)
	if err != nil {
		log.Fatalf("Failed to configure TUN parameters: %v", err)
	}
	log.Println("Configured TAP device as TUN (internal driver config).")

	// --- 3. Manually Configure the Windows Network Adapter's IP (Outside of Go) ---
	log.Println("\n--- IMPORTANT: MANUAL STEP REQUIRED ---")
	log.Println("Please manually configure the IP address of the TAP adapter:")
	log.Println("1. Open an Administrator Command Prompt/PowerShell.")
	log.Println("2. Find your new TAP adapter's name (e.g., 'Ethernet 2', 'Local Area Connection X').")
	log.Println("   You can see it in 'Network Connections' or by running `netsh interface show interface`.")
	log.Println("3. Run: `netsh interface ip set address \"Your Adapter Name\" static 192.168.10.1 255.255.255.0`")
	log.Println("   (Replace \"Your Adapter Name\" with the actual name of the TAP device).")
	log.Println("--- END MANUAL STEP ---\n")

	// --- 4. Read/Write Loop ---
	log.Println("TUN interface is active. Reading packets (press Ctrl+C to stop)...")

	// Use a Mutex for concurrent Read/Write operations if they were to happen on the same handle
	// For this simple example, we only read and write in sequence.
	var mu sync.Mutex
	packet := make([]byte, 2000) // MTU is typically 1500, but allow more for safety

	for {
		var nRead uint32
		mu.Lock() // Lock for read
		err := _ReadFile(handle, &packet[0], uint32(len(packet)), &nRead, nil)
		mu.Unlock() // Unlock after read
		if err != nil {
			// A common error during graceful shutdown (Ctrl+C) might be file closed.
			// Or, if the adapter is disabled externally.
			if err == syscall.ERROR_BROKEN_PIPE || err == syscall.ERROR_OPERATION_ABORTED || err == syscall.EINVAL {
				log.Println("Read operation interrupted or device closed. Exiting.")
				break
			}
			log.Printf("Error reading from TUN interface: %v", err)
			continue
		}

		if nRead == 0 {
			// This might happen if the handle is closed from another thread, or during shutdown.
			continue
		}

		// Simple packet inspection: print source/destination IP for IPv4 packets
		if nRead >= 20 && (packet[0]&0xF0) == 0x40 { // Check if it's an IPv4 packet (version 4)
			srcIP := net.IPv4(packet[12], packet[13], packet[14], packet[15]).String()
			dstIP := net.IPv4(packet[16], packet[17], packet[18], packet[19]).String()
			protocol := packet[9] // e.g., 1=ICMP, 6=TCP, 17=UDP
			log.Printf("Received IPv4 packet (len %d): %s -> %s (Protocol: %d)", nRead, srcIP, dstIP, protocol)

			// Very basic ICMP Echo Request (Ping) response demonstration
			// This is highly simplified and lacks proper checksum recalculation.
			// For a real network stack, use libraries like gopacket.
			if protocol == 1 && nRead >= 20 && packet[20] == 8 { // ICMP Echo Request (Type 8)
				log.Printf("  -> Detected ICMP Echo Request. Naively attempting to respond...")
				// Flip source and destination IPs
				copy(packet[12:16], net.ParseIP(dstIP).To4())
				copy(packet[16:20], net.ParseIP(srcIP).To4())
				// Change type to Echo Reply (0)
				packet[20] = 0
				// Clear checksum (proper recalculation needed for correctness)
				packet[22] = 0
				packet[23] = 0

				var nWritten uint32
				mu.Lock() // Lock for write
				if err := _WriteFile(handle, &packet[0], nRead, &nWritten, nil); err != nil {
					log.Printf("Error writing ICMP response: %v", err)
				} else {
					log.Printf("  -> Sent (naive) ICMP Echo Reply (wrote %d bytes).", nWritten)
				}
				mu.Unlock() // Unlock after write
			}
		} else {
			log.Printf("Received non-IPv4 packet (len %d)", nRead)
		}
	}
}
