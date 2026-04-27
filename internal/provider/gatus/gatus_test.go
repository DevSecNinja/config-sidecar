package gatus_test

import (
	"strings"
	"testing"

	"github.com/home-operations/gatus-sidecar/internal/endpoint"
	gatusprovider "github.com/home-operations/gatus-sidecar/internal/provider/gatus"
)

func TestGatusProvider_Name(t *testing.T) {
	p := gatusprovider.New()
	if got := p.Name(); got != "gatus" {
		t.Errorf("Name() = %q, want %q", got, "gatus")
	}
}

func TestGatusProvider_Render_Empty(t *testing.T) {
	p := gatusprovider.New()
	data, err := p.Render(nil)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !strings.Contains(string(data), "endpoints") {
		t.Errorf("expected 'endpoints' key in output, got:\n%s", string(data))
	}
}

func TestGatusProvider_Render_SingleEndpoint(t *testing.T) {
	p := gatusprovider.New()
	endpoints := []*endpoint.Endpoint{
		{
			Name:       "my-app",
			URL:        "https://myapp.example.com",
			Interval:   "1m",
			Conditions: []string{"[STATUS] == 200"},
		},
	}

	data, err := p.Render(endpoints)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "my-app") {
		t.Errorf("expected endpoint name in output, got:\n%s", content)
	}
	if !strings.Contains(content, "https://myapp.example.com") {
		t.Errorf("expected URL in output, got:\n%s", content)
	}
	if !strings.Contains(content, "[STATUS] == 200") {
		t.Errorf("expected condition in output, got:\n%s", content)
	}
	if !strings.Contains(content, "endpoints:") {
		t.Errorf("expected 'endpoints:' wrapper in output, got:\n%s", content)
	}
}

func TestGatusProvider_Render_MultipleEndpoints(t *testing.T) {
	p := gatusprovider.New()
	endpoints := []*endpoint.Endpoint{
		{Name: "app-a", URL: "https://a.example.com", Interval: "1m"},
		{Name: "app-b", URL: "https://b.example.com", Interval: "30s"},
	}

	data, err := p.Render(endpoints)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "app-a") {
		t.Errorf("expected app-a in output, got:\n%s", content)
	}
	if !strings.Contains(content, "app-b") {
		t.Errorf("expected app-b in output, got:\n%s", content)
	}
}
