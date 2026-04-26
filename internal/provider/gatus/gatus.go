package gatus

import (
	"github.com/home-operations/gatus-sidecar/internal/endpoint"
	"github.com/home-operations/gatus-sidecar/internal/provider"
	"gopkg.in/yaml.v3"
)

// Provider implements the Gatus output format.
// It renders endpoint configurations as Gatus-compatible YAML.
type Provider struct{}

var _ provider.Provider = (*Provider)(nil)

// New creates a new Gatus provider.
func New() *Provider {
	return &Provider{}
}

// Name returns the provider identifier.
func (p *Provider) Name() string {
	return "gatus"
}

// Render serializes the given endpoints to Gatus YAML format.
func (p *Provider) Render(endpoints []*endpoint.Endpoint) ([]byte, error) {
	return yaml.Marshal(map[string]any{"endpoints": endpoints})
}
