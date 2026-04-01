package docker

import (
	"testing"
	"time"

	"github.com/home-operations/gatus-sidecar/internal/config"
)

func TestExtractURL(t *testing.T) {
	tests := []struct {
		name            string
		labels          map[string]string
		defaultProtocol string
		want            string
	}{
		{
			name: "explicit gatus.url takes precedence",
			labels: map[string]string{
				"traefik.http.routers.app.rule": "Host(`app.example.com`)",
				"gatus.url":                     "https://custom.example.com/health",
			},
			defaultProtocol: "https",
			want:            "https://custom.example.com/health",
		},
		{
			name: "traefik host rule with default https",
			labels: map[string]string{
				"traefik.http.routers.app.rule": "Host(`app.example.com`)",
			},
			defaultProtocol: "https",
			want:            "https://app.example.com",
		},
		{
			name: "traefik host rule with default http",
			labels: map[string]string{
				"traefik.http.routers.app.rule": "Host(`app.example.com`)",
			},
			defaultProtocol: "http",
			want:            "http://app.example.com",
		},
		{
			name: "traefik host rule with tls=true overrides default",
			labels: map[string]string{
				"traefik.http.routers.myapp.rule": "Host(`secure.example.com`)",
				"traefik.http.routers.myapp.tls":  "true",
			},
			defaultProtocol: "http",
			want:            "https://secure.example.com",
		},
		{
			name: "traefik host rule with websecure entrypoint",
			labels: map[string]string{
				"traefik.http.routers.web.rule":        "Host(`web.example.com`)",
				"traefik.http.routers.web.entrypoints": "websecure",
			},
			defaultProtocol: "http",
			want:            "https://web.example.com",
		},
		{
			name: "traefik host rule with https entrypoint",
			labels: map[string]string{
				"traefik.http.routers.web.rule":        "Host(`web.example.com`)",
				"traefik.http.routers.web.entrypoints": "https",
			},
			defaultProtocol: "http",
			want:            "https://web.example.com",
		},
		{
			name:            "no relevant labels returns empty",
			labels:          map[string]string{"some.other.label": "value"},
			defaultProtocol: "https",
			want:            "",
		},
		{
			name: "traefik rule without Host returns empty",
			labels: map[string]string{
				"traefik.http.routers.app.rule": "PathPrefix(`/api`)",
			},
			defaultProtocol: "https",
			want:            "",
		},
		{
			name:            "empty labels returns empty",
			labels:          map[string]string{},
			defaultProtocol: "https",
			want:            "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractURL(tt.labels, tt.defaultProtocol)
			if got != tt.want {
				t.Errorf("extractURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsEnabled(t *testing.T) {
	tests := []struct {
		name         string
		labels       map[string]string
		enabledLabel string
		want         bool
	}{
		{
			name:         "missing label defaults to true",
			labels:       map[string]string{},
			enabledLabel: "gatus.enabled",
			want:         true,
		},
		{
			name:         "label set to true",
			labels:       map[string]string{"gatus.enabled": "true"},
			enabledLabel: "gatus.enabled",
			want:         true,
		},
		{
			name:         "label set to false",
			labels:       map[string]string{"gatus.enabled": "false"},
			enabledLabel: "gatus.enabled",
			want:         false,
		},
		{
			name:         "label set to 0",
			labels:       map[string]string{"gatus.enabled": "0"},
			enabledLabel: "gatus.enabled",
			want:         false,
		},
		{
			name:         "label set to 1",
			labels:       map[string]string{"gatus.enabled": "1"},
			enabledLabel: "gatus.enabled",
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEnabled(tt.labels, tt.enabledLabel)
			if got != tt.want {
				t.Errorf("isEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasTraefikRouter(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   bool
	}{
		{
			name:   "has traefik router rule",
			labels: map[string]string{"traefik.http.routers.app.rule": "Host(`app.example.com`)"},
			want:   true,
		},
		{
			name:   "no traefik labels",
			labels: map[string]string{"some.label": "value"},
			want:   false,
		},
		{
			name:   "traefik label but not rule",
			labels: map[string]string{"traefik.http.routers.app.tls": "true"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasTraefikRouter(tt.labels)
			if got != tt.want {
				t.Errorf("hasTraefikRouter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildEndpoint(t *testing.T) {
	cfg := &config.Config{
		DefaultInterval:       time.Minute,
		DockerDefaultProtocol: "https",
		LabelConfig:           "gatus.endpoint",
		LabelEnabled:          "gatus.enabled",
	}

	tests := []struct {
		name      string
		container string
		labels    map[string]string
		wantName  string
		wantURL   string
		wantGroup string
		wantConds []string
	}{
		{
			name:      "basic traefik container",
			container: "myapp",
			labels: map[string]string{
				"traefik.http.routers.myapp.rule": "Host(`myapp.example.com`)",
			},
			wantName:  "myapp",
			wantURL:   "https://myapp.example.com",
			wantConds: []string{"[STATUS] == 200"},
		},
		{
			name:      "strips leading slash from name",
			container: "/myapp",
			labels: map[string]string{
				"traefik.http.routers.myapp.rule": "Host(`myapp.example.com`)",
			},
			wantName:  "myapp",
			wantURL:   "https://myapp.example.com",
			wantConds: []string{"[STATUS] == 200"},
		},
		{
			name:      "template override via label",
			container: "myapp",
			labels: map[string]string{
				"traefik.http.routers.myapp.rule": "Host(`myapp.example.com`)",
				"gatus.endpoint":                  "group: infrastructure\nconditions:\n  - \"[STATUS] == 200\"\n  - \"[RESPONSE_TIME] < 1000\"",
			},
			wantName:  "myapp",
			wantURL:   "https://myapp.example.com",
			wantGroup: "infrastructure",
			wantConds: []string{"[STATUS] == 200", "[RESPONSE_TIME] < 1000"},
		},
		{
			name:      "explicit gatus.url",
			container: "myapp",
			labels: map[string]string{
				"gatus.url": "https://custom.example.com/health",
			},
			wantName:  "myapp",
			wantURL:   "https://custom.example.com/health",
			wantConds: []string{"[STATUS] == 200"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := buildEndpoint(tt.container, tt.labels, cfg)
			if e.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", e.Name, tt.wantName)
			}
			if e.URL != tt.wantURL {
				t.Errorf("URL = %q, want %q", e.URL, tt.wantURL)
			}
			if e.Group != tt.wantGroup {
				t.Errorf("Group = %q, want %q", e.Group, tt.wantGroup)
			}
			if len(e.Conditions) != len(tt.wantConds) {
				t.Errorf("Conditions = %v, want %v", e.Conditions, tt.wantConds)
			} else {
				for i, c := range e.Conditions {
					if c != tt.wantConds[i] {
						t.Errorf("Conditions[%d] = %q, want %q", i, c, tt.wantConds[i])
					}
				}
			}
		})
	}
}
