//go:build windows

package core

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	AF_INET                   = 2 // Address family for IPv4
	TCP_TABLE_OWNER_PID_ALL   = 5 // MIB_TCP_TABLE_OWNER_PID structure type
	ERROR_INSUFFICIENT_BUFFER = 122
	MIB_TCP_STATE_DELETE_TCB  = 12 // State to request connection termination
)

// MIB_TCPROW_OWNER_PID structure for GetExtendedTcpTable (IPv4)
// https://docs.microsoft.com/en-us/windows/win32/api/tcpmib/ns-tcpmib-mib_tcprow_owner_pid
type MIB_TCPROW_OWNER_PID struct {
	DwState      uint32
	DwLocalAddr  uint32 // Stored in network byte order
	DwLocalPort  uint32 // Stored in network byte order
	DwRemoteAddr uint32 // Stored in network byte order
	DwRemotePort uint32 // Stored in network byte order
	DwOwningPid  uint32
}

// MIB_TCPTABLE_OWNER_PID structure for GetExtendedTcpTable (IPv4)
// https://docs.microsoft.com/en-us/windows/win32/api/tcpmib/ns-tcpmib-mib_tcptable_owner_pid
type MIB_TCPTABLE_OWNER_PID struct {
	DwNumEntries uint32
	Table        [1]MIB_TCPROW_OWNER_PID // Placeholder for the first element
}

// MIB_TCPROW structure for SetTcpEntry (IPv4) - Note the slightly different structure
// https://docs.microsoft.com/en-us/windows/win32/api/tcpmib/ns-tcpmib-mib_tcprow
type MIB_TCPROW struct {
	DwState      uint32
	DwLocalAddr  uint32 // Network byte order
	DwLocalPort  uint32 // Network byte order
	DwRemoteAddr uint32 // Network byte order
	DwRemotePort uint32 // Network byte order
}

var (
	iphlpapi                = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetExtendedTcpTable = iphlpapi.NewProc("GetExtendedTcpTable")
	procSetTcpEntry         = iphlpapi.NewProc("SetTcpEntry")
)

func ipString(ip uint32) string {
	// IP is stored in network byte order (big-endian),
	// but net.IP expects host byte order representation in the byte slice.
	// However, common interpretation treats the uint32 directly.
	// Let's convert carefully.
	ipBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(ipBytes, ip) // Treat dw*Addr as big-endian
	return net.IP(ipBytes).String()
}

func portString(port uint32) string {
	// Port is stored in network byte order. Need to convert to host byte order for display.
	portBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(portBytes, port)
	return fmt.Sprintf("%d", binary.BigEndian.Uint16(portBytes[2:])) // Extract the 16-bit port
}

func closeAllOpenTCPconnections() (err error) {
	log.Println("Attempting to close all IPv4 TCP connections. Requires Administrator privileges.")

	var pdwSize uint32
	var ret uintptr
	// First call to get the required buffer size
	ret, _, err = procGetExtendedTcpTable.Call(
		0,                                 // NULL buffer pointer
		uintptr(unsafe.Pointer(&pdwSize)), // Pointer to size variable
		0,                                 // Order (FALSE)
		uintptr(AF_INET),                  // Address Family (IPv4)
		uintptr(TCP_TABLE_OWNER_PID_ALL),  // Table class
		0,                                 // Reserved
	)
	// We expect ERROR_INSUFFICIENT_BUFFER (122)
	if ret == 0 && pdwSize == 0 {
		log.Fatalf("GetExtendedTcpTable failed to get size: %v", err) // Use syscall.GetLastError() for error code
		return
	}
	if err != syscall.Errno(ERROR_INSUFFICIENT_BUFFER) {
		log.Printf("Warning: GetExtendedTcpTable (size check) returned unexpected error code: %v\n", err)
		// Continue anyway, pdwSize might be set correctly
	}
	if pdwSize == 0 {
		log.Fatalf("GetExtendedTcpTable returned size 0.")
	}

	buffer := make([]byte, pdwSize)

	// Second call to get the actual table data
	ret, _, err = procGetExtendedTcpTable.Call(
		uintptr(unsafe.Pointer(&buffer[0])), // Pointer to buffer
		uintptr(unsafe.Pointer(&pdwSize)),   // Pointer to size variable
		0,                                   // Order (FALSE)
		uintptr(AF_INET),                    // Address Family (IPv4)
		uintptr(TCP_TABLE_OWNER_PID_ALL),    // Table class
		0,                                   // Reserved
	)
	if ret != 0 { // NO_ERROR (0) indicates success
		log.Fatalf("GetExtendedTcpTable failed to get data: %v (Error code: %d)", err, ret)
		return
	}

	// Cast the buffer to the table structure
	tcpTable := (*MIB_TCPTABLE_OWNER_PID)(unsafe.Pointer(&buffer[0]))

	// Calculate the starting address of the actual table entries
	tableEntryPtr := uintptr(unsafe.Pointer(&buffer[0])) + unsafe.Offsetof(tcpTable.DwNumEntries)
	entrySize := unsafe.Sizeof(MIB_TCPROW_OWNER_PID{})

	log.Printf("Found %d IPv4 TCP connections.\n", tcpTable.DwNumEntries)

	closedCount := 0
	for i := uint32(0); i < tcpTable.DwNumEntries; i++ {
		// Get the pointer to the current entry
		entry := (*MIB_TCPROW_OWNER_PID)(unsafe.Pointer(tableEntryPtr + uintptr(i)*entrySize))

		// Skip invalid entries if necessary (e.g., local address 0.0.0.0 might indicate listening socket)
		// We typically want established or closing connections. States: 5 (ESTABLISHED), 8 (CLOSE_WAIT) etc.
		// Let's try closing all non-listening states for this example. State 2 = LISTENING
		if entry.DwState == 2 { // MIB_TCP_STATE_LISTEN = 2
			continue
		}

		localAddr := ipString(entry.DwLocalAddr)
		localPort := portString(entry.DwLocalPort)
		remoteAddr := ipString(entry.DwRemoteAddr)
		remotePort := portString(entry.DwRemotePort)

		log.Printf("Attempting to close connection: Local: %s:%s, Remote: %s:%s, PID: %d, State: %d\n",
			localAddr, localPort, remoteAddr, remotePort, entry.DwOwningPid, entry.DwState)

		// Prepare the structure for SetTcpEntry
		tcpRow := MIB_TCPROW{
			DwState:      MIB_TCP_STATE_DELETE_TCB, // Request deletion
			DwLocalAddr:  entry.DwLocalAddr,
			DwLocalPort:  entry.DwLocalPort,
			DwRemoteAddr: entry.DwRemoteAddr,
			DwRemotePort: entry.DwRemotePort,
		}

		// Call SetTcpEntry
		ret, _, err = procSetTcpEntry.Call(uintptr(unsafe.Pointer(&tcpRow)))
		if ret != 0 { // NO_ERROR (0) indicates success
			log.Printf(" -> Failed to close connection: %v (Error code: %d)\n", err, ret)
		} else {
			log.Println(" -> Connection closure requested successfully.")
			closedCount++
		}
	}

	log.Printf("Finished. Requested closure for %d connections.\n", closedCount)

	return nil
}
