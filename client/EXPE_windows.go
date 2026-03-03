//go:build windows

package client

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
	AF_INET                   = 2
	TCP_TABLE_OWNER_PID_ALL   = 5
	ERROR_INSUFFICIENT_BUFFER = 122
	MIB_TCP_STATE_DELETE_TCB  = 12
)

type MIB_TCPROW_OWNER_PID struct {
	DwState      uint32
	DwLocalAddr  uint32
	DwLocalPort  uint32
	DwRemoteAddr uint32
	DwRemotePort uint32
	DwOwningPid  uint32
}

type MIB_TCPTABLE_OWNER_PID struct {
	DwNumEntries uint32
	Table        [1]MIB_TCPROW_OWNER_PID
}

type MIB_TCPROW struct {
	DwState      uint32
	DwLocalAddr  uint32
	DwLocalPort  uint32
	DwRemoteAddr uint32
	DwRemotePort uint32
}

var (
	iphlpapi                = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetExtendedTcpTable = iphlpapi.NewProc("GetExtendedTcpTable")
	procSetTcpEntry         = iphlpapi.NewProc("SetTcpEntry")
)

func ipString(ip uint32) string {

	ipBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(ipBytes, ip)
	return net.IP(ipBytes).String()
}

func portString(port uint32) string {

	portBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(portBytes, port)
	return fmt.Sprintf("%d", binary.BigEndian.Uint16(portBytes[2:]))
}

func closeAllOpenTCPconnections() (err error) {
	defer RecoverAndLog()

	var pdwSize uint32
	var ret uintptr

	ret, _, err = procGetExtendedTcpTable.Call(
		0,
		uintptr(unsafe.Pointer(&pdwSize)),
		0,
		uintptr(AF_INET),
		uintptr(TCP_TABLE_OWNER_PID_ALL),
		0,
	)

	if ret == 0 && pdwSize == 0 {
		return err
	}
	if err != syscall.Errno(ERROR_INSUFFICIENT_BUFFER) {
		return err

	}
	if pdwSize == 0 {
		return fmt.Errorf("GetExtendedTcpTable returned size 0.")
	}

	buffer := make([]byte, pdwSize)

	ret, _, err = procGetExtendedTcpTable.Call(
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&pdwSize)),
		0,
		uintptr(AF_INET),
		uintptr(TCP_TABLE_OWNER_PID_ALL),
		0,
	)
	if ret != 0 {

		return err
	}

	tcpTable := (*MIB_TCPTABLE_OWNER_PID)(unsafe.Pointer(&buffer[0]))

	tableEntryPtr := uintptr(unsafe.Pointer(&buffer[0])) + unsafe.Offsetof(tcpTable.DwNumEntries)
	entrySize := unsafe.Sizeof(MIB_TCPROW_OWNER_PID{})

	log.Printf("Found %d IPv4 TCP connections.\n", tcpTable.DwNumEntries)

	closedCount := 0
	for i := uint32(0); i < tcpTable.DwNumEntries; i++ {

		entry := (*MIB_TCPROW_OWNER_PID)(unsafe.Pointer(tableEntryPtr + uintptr(i)*entrySize))

		if entry.DwState == 2 {
			continue
		}

		localAddr := ipString(entry.DwLocalAddr)
		localPort := portString(entry.DwLocalPort)
		remoteAddr := ipString(entry.DwRemoteAddr)
		remotePort := portString(entry.DwRemotePort)

		log.Printf("Attempting to close connection: Local: %s:%s, Remote: %s:%s, PID: %d, State: %d\n",
			localAddr, localPort, remoteAddr, remotePort, entry.DwOwningPid, entry.DwState)

		tcpRow := MIB_TCPROW{
			DwState:      MIB_TCP_STATE_DELETE_TCB,
			DwLocalAddr:  entry.DwLocalAddr,
			DwLocalPort:  entry.DwLocalPort,
			DwRemoteAddr: entry.DwRemoteAddr,
			DwRemotePort: entry.DwRemotePort,
		}

		ret, _, err = procSetTcpEntry.Call(uintptr(unsafe.Pointer(&tcpRow)))
		if ret != 0 {

		} else {
			log.Println(" -> Connection closure requested successfully.")
			closedCount++
		}
	}

	return nil
}
