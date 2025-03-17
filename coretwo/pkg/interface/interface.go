package iface

import (
	"fmt"
	"net"
	"sync"
)

// Interface represents a network interface
type Interface struct {
	Name      string
	Index     int
	Flags     net.Flags
	Addresses []net.Addr
	MTU       int
}

// Manager manages network interfaces
type Manager struct {
	mu         sync.RWMutex
	interfaces map[string]*Interface
}

// NewManager creates a new interface manager
func NewManager() *Manager {
	return &Manager{
		interfaces: make(map[string]*Interface),
	}
}

// GetInterface returns an interface by name
func (m *Manager) GetInterface(name string) (*Interface, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	iface, exists := m.interfaces[name]
	if !exists {
		return nil, fmt.Errorf("interface %s not found", name)
	}

	return iface, nil
}

// ListInterfaces returns all managed interfaces
func (m *Manager) ListInterfaces() []*Interface {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Interface, 0, len(m.interfaces))
	for _, iface := range m.interfaces {
		result = append(result, iface)
	}

	return result
}

// AddInterface adds a new interface
func (m *Manager) AddInterface(iface *Interface) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.interfaces[iface.Name]; exists {
		return fmt.Errorf("interface %s already exists", iface.Name)
	}

	m.interfaces[iface.Name] = iface
	return nil
}

// RemoveInterface removes an interface
func (m *Manager) RemoveInterface(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.interfaces[name]; !exists {
		return fmt.Errorf("interface %s not found", name)
	}

	delete(m.interfaces, name)
	return nil
}

// UpdateInterface updates an existing interface
func (m *Manager) UpdateInterface(iface *Interface) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.interfaces[iface.Name]; !exists {
		return fmt.Errorf("interface %s not found", iface.Name)
	}

	m.interfaces[iface.Name] = iface
	return nil
}

// GetDefaultInterface returns the default network interface
func (m *Manager) GetDefaultInterface() (*Interface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces: %w", err)
	}

	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		// Get addresses for the interface
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		// Look for a non-loopback address
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				return &Interface{
					Name:      iface.Name,
					Index:     iface.Index,
					Flags:     iface.Flags,
					Addresses: addrs,
					MTU:       iface.MTU,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("no default interface found")
}

// RefreshInterfaces refreshes the list of interfaces from the system
func (m *Manager) RefreshInterfaces() error {
	interfaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("failed to get interfaces: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear existing interfaces
	m.interfaces = make(map[string]*Interface)

	// Add all interfaces
	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		m.interfaces[iface.Name] = &Interface{
			Name:      iface.Name,
			Index:     iface.Index,
			Flags:     iface.Flags,
			Addresses: addrs,
			MTU:       iface.MTU,
		}
	}

	return nil
}
