package endpoint

import (
	"log/slog"
	"maps"
)

const (
	fieldName       = "name"
	fieldHeaders    = "headers"
	fieldConditions = "conditions"
	fieldDNS        = "dns"
)

// Endpoint represents the configuration for a single endpoint.
// All Gatus endpoint fields are explicitly declared; unknown keys are rejected.
type Endpoint struct {
	Enabled     *bool             `yaml:"enabled,omitempty"`
	Name        string            `yaml:"name"`
	Group       string            `yaml:"group,omitempty"`
	URL         string            `yaml:"url"`
	Method      string            `yaml:"method,omitempty"`
	Body        string            `yaml:"body,omitempty"`
	GraphQL     bool              `yaml:"graphql,omitempty"`
	Headers     map[string]string `yaml:"headers,omitempty"`
	ExtraLabels map[string]string `yaml:"extra-labels,omitempty"`
	Interval    string            `yaml:"interval"`
	Conditions  []string          `yaml:"conditions,omitempty"`
	Alerts      []map[string]any  `yaml:"alerts,omitempty"`
	Maintenance []map[string]any  `yaml:"maintenance,omitempty"`
	DNS         map[string]any    `yaml:"dns,omitempty"`
	SSH         map[string]any    `yaml:"ssh,omitempty"`
	Client      map[string]any    `yaml:"client,omitempty"`
	UI          map[string]any    `yaml:"ui,omitempty"`
	Guarded     bool              `yaml:"-"`
}

// ApplyTemplate applies template data to the endpoint, allowing overrides of default values.
// Unknown keys are logged and ignored — they are not forwarded to Gatus.
func (e *Endpoint) ApplyTemplate(templateData map[string]any) {
	if templateData == nil {
		return
	}

	for key, value := range templateData {
		switch key {
		case "enabled":
			e.setEnabledField(value)
		case fieldName:
			e.setStringField(&e.Name, value)
		case "group":
			e.setStringField(&e.Group, value)
		case "url":
			e.setStringField(&e.URL, value)
		case "method":
			e.setStringField(&e.Method, value)
		case "body":
			e.setStringField(&e.Body, value)
		case "graphql":
			if v, ok := value.(bool); ok {
				e.GraphQL = v
			}
		case fieldHeaders:
			e.setStringMapField(&e.Headers, value)
		case "extra-labels":
			e.setStringMapField(&e.ExtraLabels, value)
		case "interval":
			e.setStringField(&e.Interval, value)
		case fieldConditions:
			e.setConditionsField(value)
		case "alerts":
			e.setSliceOfMapsField(&e.Alerts, value)
		case "maintenance":
			e.setSliceOfMapsField(&e.Maintenance, value)
		case fieldDNS:
			e.setMapField(&e.DNS, value)
		case "ssh":
			e.setMapField(&e.SSH, value)
		case "client":
			e.setMapField(&e.Client, value)
		case "ui":
			e.setMapField(&e.UI, value)
		case "guarded":
			if guarded, ok := value.(bool); ok {
				e.Guarded = guarded
			}
		default:
			slog.Warn("ignoring unknown endpoint field from template", "key", key)
		}
	}
}

// setEnabledField sets the Enabled pointer field from a bool value
func (e *Endpoint) setEnabledField(value any) {
	if v, ok := value.(bool); ok {
		e.Enabled = &v
	}
}

// setStringField sets a string field if the value is a string
func (e *Endpoint) setStringField(field *string, value any) {
	if str, ok := value.(string); ok {
		*field = str
	}
}

// setStringMapField sets string-to-string map fields, accepting both
// map[string]string and map[string]any (with string values)
func (e *Endpoint) setStringMapField(field *map[string]string, value any) {
	switch v := value.(type) {
	case map[string]string:
		if *field == nil {
			*field = make(map[string]string)
		}
		for k, val := range v {
			(*field)[k] = val
		}
	case map[string]any:
		if *field == nil {
			*field = make(map[string]string)
		}
		for k, val := range v {
			if str, ok := val.(string); ok {
				(*field)[k] = str
			}
		}
	}
}

// setConditionsField handles different condition formats
func (e *Endpoint) setConditionsField(value any) {
	switch v := value.(type) {
	case []string:
		e.Conditions = v
	case []any:
		conditions := make([]string, 0, len(v))
		for _, cond := range v {
			if str, ok := cond.(string); ok {
				conditions = append(conditions, str)
			}
		}
		e.Conditions = conditions
	case string:
		e.Conditions = []string{v}
	}
}

// setSliceOfMapsField sets a []map[string]any field, accepting both
// []map[string]any and []any (with map[string]any elements)
func (e *Endpoint) setSliceOfMapsField(field *[]map[string]any, value any) {
	switch v := value.(type) {
	case []map[string]any:
		*field = v
	case []any:
		result := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				result = append(result, m)
			}
		}
		*field = result
	}
}

// setMapField merges map settings into the specified field
func (e *Endpoint) setMapField(field *map[string]any, value any) {
	if mapValue, ok := value.(map[string]any); ok {
		if *field == nil {
			*field = make(map[string]any)
		}
		maps.Copy(*field, mapValue)
	}
}
