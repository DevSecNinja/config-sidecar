package state

import (
	"log/slog"
	"os"
	"reflect"
	"sort"
	"sync"

	"github.com/home-operations/gatus-sidecar/internal/endpoint"
	"github.com/home-operations/gatus-sidecar/internal/provider"
)

// Manager maintains the global state of all endpoints
type Manager struct {
	mu         sync.Mutex
	endpoints  map[string]*endpoint.Endpoint // keyed by resource key (name-namespace)
	outputFile string
	provider   provider.Provider
}

// NewManager creates a new state manager with the given output provider.
func NewManager(outputFile string, p provider.Provider) *Manager {
	return &Manager{
		endpoints:  make(map[string]*endpoint.Endpoint),
		outputFile: outputFile,
		provider:   p,
	}
}

// AddOrUpdate adds or updates an endpoint and writes state if changed
func (m *Manager) AddOrUpdate(key string, e *endpoint.Endpoint, write bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if this is actually a change
	existing, exists := m.endpoints[key]
	if exists && reflect.DeepEqual(existing, e) {
		return false // No change
	}

	m.endpoints[key] = e

	// Write state if requested
	if write {
		m.writeState()
	}

	return true // Change detected
}

// Remove removes an endpoint and writes state if changed
func (m *Manager) Remove(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.endpoints[key]
	if !exists {
		return false // No change
	}

	delete(m.endpoints, key)
	m.writeState()
	return true // Change detected
}

// ForceWrite forces a write of the current state to disk
func (m *Manager) ForceWrite() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeState()
}

// writeState writes the current state to disk (must be called with mutex held)
func (m *Manager) writeState() {
	endpoints := m.getSortedEndpoints()

	data, err := m.provider.Render(endpoints)
	if err != nil {
		slog.Error("failed to render state", "error", err, "provider", m.provider.Name())
		return
	}

	if err := os.WriteFile(m.outputFile, data, 0o644); err != nil {
		slog.Error("failed to write state to file", "error", err)
		return
	}

	slog.Info("wrote consolidated state file", "file", m.outputFile, "endpoints", len(m.endpoints), "provider", m.provider.Name())
}

// getSortedEndpoints returns endpoints sorted by name for deterministic output
// (must be called with mutex held).
func (m *Manager) getSortedEndpoints() []*endpoint.Endpoint {
	endpoints := make([]*endpoint.Endpoint, 0, len(m.endpoints))
	for _, e := range m.endpoints {
		endpoints = append(endpoints, e)
	}
	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].Name < endpoints[j].Name
	})
	return endpoints
}
