package docker

import (
	"testing"
	"time"

	"github.com/home-operations/config-sidecar/internal/config"
	"github.com/home-operations/config-sidecar/internal/endpoint"
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
		LabelGroup:            "gatus.group",
	}

	cfgWithGroup := &config.Config{
		DefaultInterval:       time.Minute,
		DockerDefaultProtocol: "https",
		DockerDefaultGroup:    "Docker",
		LabelConfig:           "gatus.endpoint",
		LabelEnabled:          "gatus.enabled",
		LabelGroup:            "gatus.group",
	}

	tests := []struct {
		name      string
		container string
		labels    map[string]string
		cfg       *config.Config
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
			cfg:       cfg,
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
			cfg:       cfg,
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
			cfg:       cfg,
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
			cfg:       cfg,
			wantName:  "myapp",
			wantURL:   "https://custom.example.com/health",
			wantConds: []string{"[STATUS] == 200"},
		},
		{
			name:      "default group from config",
			container: "myapp",
			labels: map[string]string{
				"traefik.http.routers.myapp.rule": "Host(`myapp.example.com`)",
			},
			cfg:       cfgWithGroup,
			wantName:  "myapp",
			wantURL:   "https://myapp.example.com",
			wantGroup: "Docker",
			wantConds: []string{"[STATUS] == 200"},
		},
		{
			name:      "per-container group label overrides default",
			container: "myapp",
			labels: map[string]string{
				"traefik.http.routers.myapp.rule": "Host(`myapp.example.com`)",
				"gatus.group":                     "infrastructure",
			},
			cfg:       cfgWithGroup,
			wantName:  "myapp",
			wantURL:   "https://myapp.example.com",
			wantGroup: "infrastructure",
			wantConds: []string{"[STATUS] == 200"},
		},
		{
			name:      "per-container group label without default",
			container: "myapp",
			labels: map[string]string{
				"traefik.http.routers.myapp.rule": "Host(`myapp.example.com`)",
				"gatus.group":                     "web-apps",
			},
			cfg:       cfg,
			wantName:  "myapp",
			wantURL:   "https://myapp.example.com",
			wantGroup: "web-apps",
			wantConds: []string{"[STATUS] == 200"},
		},
		{
			name:      "template group overrides default but label overrides template",
			container: "myapp",
			labels: map[string]string{
				"traefik.http.routers.myapp.rule": "Host(`myapp.example.com`)",
				"gatus.endpoint":                  "group: from-template",
				"gatus.group":                     "from-label",
			},
			cfg:       cfgWithGroup,
			wantName:  "myapp",
			wantURL:   "https://myapp.example.com",
			wantGroup: "from-label",
			wantConds: []string{"[STATUS] == 200"},
		},
		{
			name:      "template group overrides default when no label",
			container: "myapp",
			labels: map[string]string{
				"traefik.http.routers.myapp.rule": "Host(`myapp.example.com`)",
				"gatus.endpoint":                  "group: from-template",
			},
			cfg:       cfgWithGroup,
			wantName:  "myapp",
			wantURL:   "https://myapp.example.com",
			wantGroup: "from-template",
			wantConds: []string{"[STATUS] == 200"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCfg := tt.cfg
			if testCfg == nil {
				testCfg = cfg
			}
			e := buildEndpoint(tt.container, tt.labels, testCfg)
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

func TestApplyGatusLabels(t *testing.T) {
	cfg := &config.Config{
		LabelConfig:  "gatus.endpoint",
		LabelEnabled: "gatus.enabled",
		LabelGroup:   "gatus.group",
	}

	t.Run("scalar labels set endpoint fields", func(t *testing.T) {
		e := &endpoint.Endpoint{Name: "app", URL: "https://example.com", Interval: "1m"}
		labels := map[string]string{
			"gatus.interval": "30s",
			"gatus.method":   "POST",
			"gatus.body":     `{"key":"val"}`,
			"gatus.graphql":  "true",
		}
		applyGatusLabels(e, labels, cfg)
		if e.Interval != "30s" {
			t.Errorf("Interval = %q, want %q", e.Interval, "30s")
		}
		if e.Method != "POST" {
			t.Errorf("Method = %q, want %q", e.Method, "POST")
		}
		if e.Body != `{"key":"val"}` {
			t.Errorf("Body = %q, want %q", e.Body, `{"key":"val"}`)
		}
		if !e.GraphQL {
			t.Errorf("GraphQL = false, want true")
		}
	})

	t.Run("graphql false when not true", func(t *testing.T) {
		e := &endpoint.Endpoint{Name: "app", URL: "https://example.com", GraphQL: true}
		applyGatusLabels(e, map[string]string{"gatus.graphql": "false"}, cfg)
		if e.GraphQL {
			t.Errorf("GraphQL = true, want false")
		}
	})

	t.Run("gatus.conditions JSON list", func(t *testing.T) {
		e := &endpoint.Endpoint{Name: "app", URL: "https://example.com"}
		applyGatusLabels(e, map[string]string{
			"gatus.conditions": `["[STATUS] == 200","[RESPONSE_TIME] < 1000"]`,
		}, cfg)
		want := []string{"[STATUS] == 200", "[RESPONSE_TIME] < 1000"}
		if len(e.Conditions) != len(want) {
			t.Fatalf("Conditions = %v, want %v", e.Conditions, want)
		}
		for i, c := range e.Conditions {
			if c != want[i] {
				t.Errorf("Conditions[%d] = %q, want %q", i, c, want[i])
			}
		}
	})

	t.Run("gatus.conditions invalid JSON does not change field", func(t *testing.T) {
		e := &endpoint.Endpoint{Name: "app", URL: "https://example.com", Conditions: []string{"[STATUS] == 200"}}
		applyGatusLabels(e, map[string]string{"gatus.conditions": "not-json"}, cfg)
		if len(e.Conditions) != 1 || e.Conditions[0] != "[STATUS] == 200" {
			t.Errorf("Conditions unexpectedly changed: %v", e.Conditions)
		}
	})

	t.Run("gatus.alerts JSON list of objects", func(t *testing.T) {
		e := &endpoint.Endpoint{Name: "app", URL: "https://example.com"}
		applyGatusLabels(e, map[string]string{
			"gatus.alerts": `[{"type":"email"},{"type":"custom"}]`,
		}, cfg)
		if len(e.Alerts) != 2 {
			t.Fatalf("Alerts len = %d, want 2", len(e.Alerts))
		}
		if e.Alerts[0]["type"] != "email" {
			t.Errorf("Alerts[0].type = %v, want email", e.Alerts[0]["type"])
		}
	})

	t.Run("gatus.client JSON object", func(t *testing.T) {
		e := &endpoint.Endpoint{Name: "app", URL: "https://example.com"}
		applyGatusLabels(e, map[string]string{
			"gatus.client": `{"ignore-redirect":true}`,
		}, cfg)
		if e.Client == nil {
			t.Fatal("Client is nil")
		}
		if e.Client["ignore-redirect"] != true {
			t.Errorf("Client[ignore-redirect] = %v, want true", e.Client["ignore-redirect"])
		}
	})
}

func TestApplyGatusLabels_Headers(t *testing.T) {
	cfg := &config.Config{
		LabelConfig:  "gatus.endpoint",
		LabelEnabled: "gatus.enabled",
		LabelGroup:   "gatus.group",
	}

	t.Run("gatus.headers dot-separated keys assemble headers map", func(t *testing.T) {
		e := &endpoint.Endpoint{Name: "app", URL: "https://example.com"}
		applyGatusLabels(e, map[string]string{
			"gatus.headers.Host":          "sonarr.example.com",
			"gatus.headers.Authorization": "Bearer token123",
		}, cfg)
		if e.Headers == nil {
			t.Fatal("Headers is nil")
		}
		if e.Headers["Host"] != "sonarr.example.com" {
			t.Errorf("Headers[Host] = %q, want %q", e.Headers["Host"], "sonarr.example.com")
		}
		if e.Headers["Authorization"] != "Bearer token123" {
			t.Errorf("Headers[Authorization] = %q, want %q", e.Headers["Authorization"], "Bearer token123")
		}
	})

	t.Run("gatus.headers merges into existing headers", func(t *testing.T) {
		e := &endpoint.Endpoint{Name: "app", URL: "https://example.com", Headers: map[string]string{"X-Existing": "yes"}}
		applyGatusLabels(e, map[string]string{"gatus.headers.Host": "example.com"}, cfg)
		if e.Headers["X-Existing"] != "yes" {
			t.Errorf("existing header lost: %v", e.Headers)
		}
		if e.Headers["Host"] != "example.com" {
			t.Errorf("Headers[Host] = %q, want %q", e.Headers["Host"], "example.com")
		}
	})

	t.Run("reserved labels are skipped without warning", func(t *testing.T) {
		e := &endpoint.Endpoint{Name: "app", URL: "https://example.com", Interval: "1m"}
		// These are reserved – none should be misinterpreted as unknown labels.
		labels := map[string]string{
			"gatus.url":      "https://example.com",
			"gatus.enabled":  "true",
			"gatus.group":    "mygroup",
			"gatus.endpoint": "interval: 5m",
		}
		applyGatusLabels(e, labels, cfg)
		// Interval must NOT be set from the blob (that's handled by buildEndpoint, not applyGatusLabels)
		if e.Interval != "1m" {
			t.Errorf("Interval unexpectedly changed to %q", e.Interval)
		}
	})

	t.Run("individual labels override blob values", func(t *testing.T) {
		blobCfg := &config.Config{
			DefaultInterval: time.Minute,
			LabelConfig:     "gatus.endpoint",
			LabelEnabled:    "gatus.enabled",
			LabelGroup:      "gatus.group",
		}
		e := buildEndpoint("app", map[string]string{
			"gatus.url":      "https://example.com",
			"gatus.endpoint": "interval: 5m\nmethod: DELETE",
			"gatus.interval": "10s",
		}, blobCfg)
		// Individual gatus.interval overrides the blob's interval
		if e.Interval != "10s" {
			t.Errorf("Interval = %q, want %q", e.Interval, "10s")
		}
		// Method from blob is still applied (individual label did not override it)
		if e.Method != "DELETE" {
			t.Errorf("Method = %q, want %q", e.Method, "DELETE")
		}
	})

	t.Run("gatus.dns JSON object", func(t *testing.T) {
		e := &endpoint.Endpoint{Name: "app", URL: "https://example.com"}
		applyGatusLabels(e, map[string]string{
			"gatus.dns": `{"query-name":"example.com","query-type":"A"}`,
		}, cfg)
		if e.DNS == nil {
			t.Fatal("DNS is nil")
		}
		if e.DNS["query-name"] != "example.com" {
			t.Errorf("DNS[query-name] = %v, want example.com", e.DNS["query-name"])
		}
	})

	t.Run("gatus.ui JSON object", func(t *testing.T) {
		e := &endpoint.Endpoint{Name: "app", URL: "https://example.com"}
		applyGatusLabels(e, map[string]string{
			"gatus.ui": `{"hide-url":true}`,
		}, cfg)
		if e.UI == nil {
			t.Fatal("UI is nil")
		}
		if e.UI["hide-url"] != true {
			t.Errorf("UI[hide-url] = %v, want true", e.UI["hide-url"])
		}
	})

	t.Run("gatus.ssh JSON object", func(t *testing.T) {
		e := &endpoint.Endpoint{Name: "app", URL: "https://example.com"}
		applyGatusLabels(e, map[string]string{
			"gatus.ssh": `{"username":"admin"}`,
		}, cfg)
		if e.SSH == nil {
			t.Fatal("SSH is nil")
		}
		if e.SSH["username"] != "admin" {
			t.Errorf("SSH[username] = %v, want admin", e.SSH["username"])
		}
	})

	t.Run("gatus.maintenance JSON list of objects", func(t *testing.T) {
		e := &endpoint.Endpoint{Name: "app", URL: "https://example.com"}
		applyGatusLabels(e, map[string]string{
			"gatus.maintenance": `[{"start":"23:00","duration":"1h"}]`,
		}, cfg)
		if len(e.Maintenance) != 1 {
			t.Fatalf("Maintenance len = %d, want 1", len(e.Maintenance))
		}
		if e.Maintenance[0]["start"] != "23:00" {
			t.Errorf("Maintenance[0].start = %v, want 23:00", e.Maintenance[0]["start"])
		}
	})
}
