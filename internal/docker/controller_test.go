package docker

import (
	"testing"
	"time"

	"github.com/home-operations/gatus-sidecar/internal/config"
)

func TestStateKey(t *testing.T) {
	tests := []struct {
		name        string
		containerID string
		want        string
	}{
		{
			name:        "long container ID is truncated to 12 chars",
			containerID: "abcdef123456789000",
			want:        "container-abcdef123456",
		},
		{
			name:        "short container ID used as-is",
			containerID: "abcdef",
			want:        "container-abcdef",
		},
		{
			name:        "exactly 12 char ID",
			containerID: "abcdef123456",
			want:        "container-abcdef123456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stateKey(tt.containerID)
			if got != tt.want {
				t.Errorf("stateKey(%q) = %q, want %q", tt.containerID, got, tt.want)
			}
		})
	}
}

func TestContainerName(t *testing.T) {
	tests := []struct {
		name  string
		names []string
		want  string
	}{
		{
			name:  "returns first name",
			names: []string{"/myapp", "/other"},
			want:  "/myapp",
		},
		{
			name:  "empty names returns unknown",
			names: []string{},
			want:  "unknown",
		},
		{
			name:  "nil names returns unknown",
			names: nil,
			want:  "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containerName(tt.names)
			if got != tt.want {
				t.Errorf("containerName(%v) = %q, want %q", tt.names, got, tt.want)
			}
		})
	}
}

func TestProcessingLogic(t *testing.T) {
	cfg := &config.Config{
		DefaultInterval:       time.Minute,
		DockerDefaultProtocol: "https",
		LabelConfig:           "gatus.endpoint",
		LabelEnabled:          "gatus.enabled",
	}

	tests := []struct {
		name          string
		labels        map[string]string
		shouldProcess bool
		expectedURL   string
		expectedName  string
	}{
		{
			name: "traefik container is processed",
			labels: map[string]string{
				"traefik.http.routers.app.rule": "Host(`app.example.com`)",
			},
			shouldProcess: true,
			expectedURL:   "https://app.example.com",
			expectedName:  "webapp",
		},
		{
			name: "container with gatus.url is processed",
			labels: map[string]string{
				"gatus.url": "https://custom.example.com",
			},
			shouldProcess: true,
			expectedURL:   "https://custom.example.com",
			expectedName:  "webapp",
		},
		{
			name: "container without traefik or gatus.url is skipped",
			labels: map[string]string{
				"some.other.label": "value",
			},
			shouldProcess: false,
		},
		{
			name: "disabled container is skipped",
			labels: map[string]string{
				"traefik.http.routers.app.rule": "Host(`app.example.com`)",
				"gatus.enabled":                 "false",
			},
			shouldProcess: false,
		},
		{
			name: "container disabled with 0 is skipped",
			labels: map[string]string{
				"traefik.http.routers.app.rule": "Host(`app.example.com`)",
				"gatus.enabled":                 "0",
			},
			shouldProcess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasRouter := hasTraefikRouter(tt.labels)
			hasURL := tt.labels["gatus.url"] != ""
			enabled := isEnabled(tt.labels, cfg.LabelEnabled)

			shouldProcess := (hasRouter || hasURL) && enabled

			if shouldProcess != tt.shouldProcess {
				t.Errorf("shouldProcess = %v, want %v", shouldProcess, tt.shouldProcess)
			}

			if tt.shouldProcess {
				e := buildEndpoint(tt.expectedName, tt.labels, cfg)
				if e.URL != tt.expectedURL {
					t.Errorf("URL = %q, want %q", e.URL, tt.expectedURL)
				}
				if e.Name != tt.expectedName {
					t.Errorf("Name = %q, want %q", e.Name, tt.expectedName)
				}
			}
		})
	}
}
