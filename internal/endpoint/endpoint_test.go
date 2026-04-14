package endpoint

import (
	"testing"
)

func TestEndpoint_ApplyTemplate(t *testing.T) {
	tests := []struct {
		name     string
		endpoint *Endpoint
		template map[string]any
		want     *Endpoint
	}{
		{
			name: "nil template does nothing",
			endpoint: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
			},
			template: nil,
			want: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
			},
		},
		{
			name: "override string fields",
			endpoint: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
				Group:    "old-group",
			},
			template: map[string]any{
				"name":     "new-name",
				"url":      "https://new.example.com",
				"interval": "30s",
				"group":    "new-group",
			},
			want: &Endpoint{
				Name:     "new-name",
				URL:      "https://new.example.com",
				Interval: "30s",
				Group:    "new-group",
			},
		},
		{
			name: "set conditions from string slice",
			endpoint: &Endpoint{
				Name:       "test",
				URL:        "https://example.com",
				Interval:   "1m",
				Conditions: []string{"[STATUS] == 200"},
			},
			template: map[string]any{
				"conditions": []string{"[STATUS] == 200", "[RESPONSE_TIME] < 500"},
			},
			want: &Endpoint{
				Name:       "test",
				URL:        "https://example.com",
				Interval:   "1m",
				Conditions: []string{"[STATUS] == 200", "[RESPONSE_TIME] < 500"},
			},
		},
		{
			name: "set conditions from any slice",
			endpoint: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
			},
			template: map[string]any{
				"conditions": []any{"[STATUS] == 200", "[RESPONSE_TIME] < 500"},
			},
			want: &Endpoint{
				Name:       "test",
				URL:        "https://example.com",
				Interval:   "1m",
				Conditions: []string{"[STATUS] == 200", "[RESPONSE_TIME] < 500"},
			},
		},
		{
			name: "set conditions from single string",
			endpoint: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
			},
			template: map[string]any{
				"conditions": "[STATUS] == 200",
			},
			want: &Endpoint{
				Name:       "test",
				URL:        "https://example.com",
				Interval:   "1m",
				Conditions: []string{"[STATUS] == 200"},
			},
		},
		{
			name: "set dns config",
			endpoint: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
			},
			template: map[string]any{
				"dns": map[string]any{
					"query-name": "example.com",
					"query-type": "A",
				},
			},
			want: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
				DNS: map[string]any{
					"query-name": "example.com",
					"query-type": "A",
				},
			},
		},
		{
			name: "merge dns config",
			endpoint: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
				DNS: map[string]any{
					"query-name": "old.example.com",
				},
			},
			template: map[string]any{
				"dns": map[string]any{
					"query-type": "AAAA",
				},
			},
			want: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
				DNS: map[string]any{
					"query-name": "old.example.com",
					"query-type": "AAAA",
				},
			},
		},
		{
			name: "set method and body",
			endpoint: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
			},
			template: map[string]any{
				"method": "POST",
				"body":   `{"query":"{}"}`,
			},
			want: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
				Method:   "POST",
				Body:     `{"query":"{}"}`,
			},
		},
		{
			name: "set graphql flag",
			endpoint: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
			},
			template: map[string]any{
				"graphql": true,
			},
			want: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
				GraphQL:  true,
			},
		},
		{
			name: "set enabled flag",
			endpoint: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
			},
			template: map[string]any{
				"enabled": false,
			},
			want: func() *Endpoint {
				f := false
				return &Endpoint{
					Name:     "test",
					URL:      "https://example.com",
					Interval: "1m",
					Enabled:  &f,
				}
			}(),
		},
		{
			name: "set headers from map[string]string",
			endpoint: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
			},
			template: map[string]any{
				"headers": map[string]string{
					"Host":          "example.com",
					"Authorization": "Bearer token",
				},
			},
			want: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
				Headers: map[string]string{
					"Host":          "example.com",
					"Authorization": "Bearer token",
				},
			},
		},
		{
			name: "set headers from map[string]any",
			endpoint: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
			},
			template: map[string]any{
				"headers": map[string]any{
					"Host": "example.com",
				},
			},
			want: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
				Headers: map[string]string{
					"Host": "example.com",
				},
			},
		},
		{
			name: "set alerts as typed slice",
			endpoint: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
			},
			template: map[string]any{
				"alerts": []any{
					map[string]any{
						"type":        "slack",
						"webhook-url": "https://hooks.slack.com/...",
					},
				},
			},
			want: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
				Alerts: []map[string]any{
					{
						"type":        "slack",
						"webhook-url": "https://hooks.slack.com/...",
					},
				},
			},
		},
		{
			name: "set maintenance windows",
			endpoint: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
			},
			template: map[string]any{
				"maintenance": []any{
					map[string]any{
						"start":    "02:00",
						"duration": "1h",
					},
				},
			},
			want: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
				Maintenance: []map[string]any{
					{
						"start":    "02:00",
						"duration": "1h",
					},
				},
			},
		},
		{
			name: "set ssh config",
			endpoint: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
			},
			template: map[string]any{
				"ssh": map[string]any{
					"username": "admin",
					"password": "secret",
				},
			},
			want: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
				SSH: map[string]any{
					"username": "admin",
					"password": "secret",
				},
			},
		},
		{
			name: "set guarded flag",
			endpoint: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
				Guarded:  false,
			},
			template: map[string]any{
				"guarded": true,
			},
			want: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
				Guarded:  true,
			},
		},
		{
			name: "unknown key is ignored and not stored",
			endpoint: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
			},
			template: map[string]any{
				"unknown-key": "some-value",
				"store":       "redis",
			},
			want: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
			},
		},
		{
			name: "ignore invalid string type",
			endpoint: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
			},
			template: map[string]any{
				"name": 123,
			},
			want: &Endpoint{
				Name:     "test",
				URL:      "https://example.com",
				Interval: "1m",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.endpoint.ApplyTemplate(tt.template)
			if tt.endpoint.Name != tt.want.Name {
				t.Errorf("Name = %v, want %v", tt.endpoint.Name, tt.want.Name)
			}
			if tt.endpoint.URL != tt.want.URL {
				t.Errorf("URL = %v, want %v", tt.endpoint.URL, tt.want.URL)
			}
			if tt.endpoint.Interval != tt.want.Interval {
				t.Errorf("Interval = %v, want %v", tt.endpoint.Interval, tt.want.Interval)
			}
			if tt.endpoint.Group != tt.want.Group {
				t.Errorf("Group = %v, want %v", tt.endpoint.Group, tt.want.Group)
			}
			if tt.endpoint.Guarded != tt.want.Guarded {
				t.Errorf("Guarded = %v, want %v", tt.endpoint.Guarded, tt.want.Guarded)
			}
			if !equalStringSlices(tt.endpoint.Conditions, tt.want.Conditions) {
				t.Errorf("Conditions = %v, want %v", tt.endpoint.Conditions, tt.want.Conditions)
			}
			if !equalMaps(tt.endpoint.DNS, tt.want.DNS) {
				t.Errorf("DNS = %v, want %v", tt.endpoint.DNS, tt.want.DNS)
			}
			if !equalMaps(tt.endpoint.SSH, tt.want.SSH) {
				t.Errorf("SSH = %v, want %v", tt.endpoint.SSH, tt.want.SSH)
			}
			if !equalStringMaps(tt.endpoint.Headers, tt.want.Headers) {
				t.Errorf("Headers = %v, want %v", tt.endpoint.Headers, tt.want.Headers)
			}
			if tt.endpoint.Method != tt.want.Method {
				t.Errorf("Method = %v, want %v", tt.endpoint.Method, tt.want.Method)
			}
			if tt.endpoint.Body != tt.want.Body {
				t.Errorf("Body = %v, want %v", tt.endpoint.Body, tt.want.Body)
			}
			if tt.endpoint.GraphQL != tt.want.GraphQL {
				t.Errorf("GraphQL = %v, want %v", tt.endpoint.GraphQL, tt.want.GraphQL)
			}
			if !equalSliceOfMaps(tt.endpoint.Alerts, tt.want.Alerts) {
				t.Errorf("Alerts = %v, want %v", tt.endpoint.Alerts, tt.want.Alerts)
			}
			if !equalSliceOfMaps(tt.endpoint.Maintenance, tt.want.Maintenance) {
				t.Errorf("Maintenance = %v, want %v", tt.endpoint.Maintenance, tt.want.Maintenance)
			}
			if tt.want.Enabled == nil && tt.endpoint.Enabled != nil {
				t.Errorf("Enabled = %v, want nil", *tt.endpoint.Enabled)
			}
			if tt.want.Enabled != nil {
				if tt.endpoint.Enabled == nil {
					t.Errorf("Enabled = nil, want %v", *tt.want.Enabled)
				} else if *tt.endpoint.Enabled != *tt.want.Enabled {
					t.Errorf("Enabled = %v, want %v", *tt.endpoint.Enabled, *tt.want.Enabled)
				}
			}
		})
	}
}

func TestEndpoint_setEnabledField(t *testing.T) {
	e := &Endpoint{}

	e.setEnabledField(true)
	if e.Enabled == nil || !*e.Enabled {
		t.Errorf("Enabled should be true")
	}

	e.setEnabledField(false)
	if e.Enabled == nil || *e.Enabled {
		t.Errorf("Enabled should be false")
	}

	e.setEnabledField("not-a-bool")
	// should remain false from previous call, not crash
}

func TestEndpoint_setStringField(t *testing.T) {
	e := &Endpoint{}
	var field string

	e.setStringField(&field, "test")
	if field != "test" {
		t.Errorf("field = %v, want test", field)
	}

	field = ""
	e.setStringField(&field, 123)
	if field != "" {
		t.Errorf("field should remain empty for invalid type, got %v", field)
	}
}

func TestEndpoint_setConditionsField(t *testing.T) {
	e := &Endpoint{}

	e.setConditionsField([]string{"cond1", "cond2"})
	if !equalStringSlices(e.Conditions, []string{"cond1", "cond2"}) {
		t.Errorf("Conditions = %v, want [cond1, cond2]", e.Conditions)
	}

	e.setConditionsField([]any{"cond3", "cond4"})
	if !equalStringSlices(e.Conditions, []string{"cond3", "cond4"}) {
		t.Errorf("Conditions = %v, want [cond3, cond4]", e.Conditions)
	}

	e.setConditionsField("single-condition")
	if !equalStringSlices(e.Conditions, []string{"single-condition"}) {
		t.Errorf("Conditions = %v, want [single-condition]", e.Conditions)
	}
}

func TestEndpoint_setStringMapField(t *testing.T) {
	e := &Endpoint{}

	// From map[string]string
	e.setStringMapField(&e.Headers, map[string]string{"Host": "example.com"})
	if e.Headers["Host"] != "example.com" {
		t.Errorf("Headers[Host] = %v, want example.com", e.Headers["Host"])
	}

	// From map[string]any
	e.setStringMapField(&e.Headers, map[string]any{"Authorization": "Bearer token"})
	if e.Headers["Authorization"] != "Bearer token" {
		t.Errorf("Headers[Authorization] = %v, want Bearer token", e.Headers["Authorization"])
	}

	// Original key still present (merge behavior)
	if e.Headers["Host"] != "example.com" {
		t.Errorf("Headers[Host] should still be example.com after merge, got %v", e.Headers["Host"])
	}

	// Invalid type is ignored
	e.setStringMapField(&e.Headers, "invalid")
	if len(e.Headers) != 2 {
		t.Errorf("Headers should remain unchanged for invalid type, got %v", e.Headers)
	}
}

func TestEndpoint_setSliceOfMapsField(t *testing.T) {
	e := &Endpoint{}

	// From []any
	e.setSliceOfMapsField(&e.Alerts, []any{
		map[string]any{"type": "slack"},
	})
	if len(e.Alerts) != 1 || e.Alerts[0]["type"] != "slack" {
		t.Errorf("Alerts = %v, want [{type: slack}]", e.Alerts)
	}

	// From []map[string]any
	e.setSliceOfMapsField(&e.Alerts, []map[string]any{
		{"type": "discord"},
	})
	if len(e.Alerts) != 1 || e.Alerts[0]["type"] != "discord" {
		t.Errorf("Alerts = %v, want [{type: discord}]", e.Alerts)
	}

	// Invalid type is ignored
	e.setSliceOfMapsField(&e.Alerts, "invalid")
	if len(e.Alerts) != 1 {
		t.Errorf("Alerts should remain unchanged for invalid type, got %v", e.Alerts)
	}
}

func TestEndpoint_setMapField(t *testing.T) {
	e := &Endpoint{}
	var field map[string]any

	e.setMapField(&field, map[string]any{"key1": "value1"})
	if field == nil || field["key1"] != "value1" {
		t.Errorf("field = %v, want {key1: value1}", field)
	}

	e.setMapField(&field, map[string]any{"key2": "value2"})
	if field["key1"] != "value1" || field["key2"] != "value2" {
		t.Errorf("field should merge, got %v", field)
	}

	e.setMapField(&field, "invalid")
	if len(field) != 2 {
		t.Errorf("field should remain unchanged for invalid type, got %v", field)
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalMaps(a, b map[string]any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

func equalStringMaps(a, b map[string]string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

func equalSliceOfMaps(a, b []map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !equalMaps(a[i], b[i]) {
			return false
		}
	}
	return true
}
