package docker

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

// PortManager manages port allocation for contexts.
type PortManager struct {
	Base int
	Step int
}

// NewPortManager creates a new port manager.
func NewPortManager(base, step int) *PortManager {
	return &PortManager{
		Base: base,
		Step: step,
	}
}

// GetPorts returns the port mapping for a context.
func (pm *PortManager) GetPorts(index int) map[string]int {
	ports := make(map[string]int)
	offset := index * pm.Step

	// Common service ports
	ports["app"] = pm.Base + offset
	ports["web"] = pm.Base + offset + 1
	ports["api"] = pm.Base + offset + 2
	ports["db"] = 5432 + index
	ports["redis"] = 6379 + index
	ports["mongo"] = 27017 + index
	ports["es"] = 9200 + index

	return ports
}

// IsPortAvailable checks if a port is available.
func IsPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// FindAvailablePort finds the next available port starting from base.
func FindAvailablePort(base int) int {
	for port := base; port < base+1000; port++ {
		if IsPortAvailable(port) {
			return port
		}
	}
	return -1
}

// PortState tracks allocated ports.
type PortState struct {
	Contexts map[string]int // context name -> port offset
}

// LoadPortState loads the port state from disk.
func LoadPortState(projectPath string) (*PortState, error) {
	statePath := filepath.Join(projectPath, ".dcell", "ports.json")
	state := &PortState{
		Contexts: make(map[string]int),
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return nil, err
	}

	// Simple JSON parsing
	// For simplicity, we'll use a simple format: name=index\n
	lines := string(data)
	for _, line := range splitLines(lines) {
		if line == "" {
			continue
		}
		parts := splitKV(line, '=')
		if len(parts) == 2 {
			if idx, err := strconv.Atoi(parts[1]); err == nil {
				state.Contexts[parts[0]] = idx
			}
		}
	}

	return state, nil
}

// Save saves the port state to disk.
func (ps *PortState) Save(projectPath string) error {
	statePath := filepath.Join(projectPath, ".dcell", "ports.json")
	if err := os.MkdirAll(filepath.Dir(statePath), 0755); err != nil {
		return err
	}

	var data string
	for name, idx := range ps.Contexts {
		data += fmt.Sprintf("%s=%d\n", name, idx)
	}

	return os.WriteFile(statePath, []byte(data), 0644)
}

// Allocate allocates a port index for a context.
func (ps *PortState) Allocate(name string) int {
	if idx, ok := ps.Contexts[name]; ok {
		return idx
	}

	// Find next available index
	used := make(map[int]bool)
	for _, idx := range ps.Contexts {
		used[idx] = true
	}

	idx := 0
	for used[idx] {
		idx++
	}

	ps.Contexts[name] = idx
	return idx
}

// Release releases a port index.
func (ps *PortState) Release(name string) {
	delete(ps.Contexts, name)
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func splitKV(s string, sep byte) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}
