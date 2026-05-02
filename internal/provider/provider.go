package provider

import "github.com/home-operations/config-sidecar/internal/endpoint"

// Provider renders a list of endpoints to an output-specific byte slice.
type Provider interface {
	// Name returns the human-readable provider identifier.
	Name() string
	// Render serializes the given endpoints to the provider-specific output format.
	Render(endpoints []*endpoint.Endpoint) ([]byte, error)
}
