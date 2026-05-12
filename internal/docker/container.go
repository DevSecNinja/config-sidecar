package docker

import (
	"encoding/json"
	"log/slog"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/home-operations/config-sidecar/internal/config"
	"github.com/home-operations/config-sidecar/internal/endpoint"
)

var hostRegex = regexp.MustCompile("Host\\(`([^`]+)`\\)")

const (
	gatusLabelPrefix        = "gatus."
	gatusHeadersLabelPrefix = "gatus.headers."
	gatusURLLabel           = "gatus.url"
	defaultHTTPCondition    = "[STATUS] == 200"
	protocolHTTPS           = "https"
	stringValueTrue         = "true"
	stringValueFalse        = "false"
	stringValueZero         = "0"
	unknownContainerName    = "unknown"
)

// extractURL determines the URL for a container from its labels.
// If a "gatus.url" label exists, it is used directly.
// Otherwise, it scans for Traefik router rules to extract the hostname and protocol.
func extractURL(labels map[string]string, defaultProtocol string) string {
	if url, ok := labels[gatusURLLabel]; ok && url != "" {
		return url
	}

	var routerName string
	var hostname string

	for key, value := range labels {
		if isTraefikRouterRule(key) {
			matches := hostRegex.FindStringSubmatch(value)
			if len(matches) >= 2 {
				hostname = matches[1]
				// Extract router name: traefik.http.routers.<name>.rule
				parts := strings.Split(key, ".")
				if len(parts) >= 4 {
					routerName = parts[3]
				}
				break
			}
		}
	}

	if hostname == "" {
		return ""
	}

	protocol := defaultProtocol
	if routerName != "" {
		// Check for explicit TLS
		tlsKey := "traefik.http.routers." + routerName + ".tls"
		if tlsVal, ok := labels[tlsKey]; ok && (tlsVal == stringValueTrue || tlsVal == "") {
			protocol = protocolHTTPS
		}

		// Check entrypoints for websecure/https
		epKey := "traefik.http.routers." + routerName + ".entrypoints"
		if epVal, ok := labels[epKey]; ok {
			lower := strings.ToLower(epVal)
			if strings.Contains(lower, "websecure") || strings.Contains(lower, protocolHTTPS) {
				protocol = protocolHTTPS
			}
		}
	}

	return protocol + "://" + hostname
}

// isEnabled checks whether a container should be monitored based on its labels.
func isEnabled(labels map[string]string, enabledLabel string) bool {
	val, ok := labels[enabledLabel]
	if !ok {
		return true
	}
	return val != stringValueFalse && val != stringValueZero
}

// hasTraefikRouter returns true if any label matches a Traefik HTTP router rule pattern.
func hasTraefikRouter(labels map[string]string) bool {
	for key := range labels {
		if isTraefikRouterRule(key) {
			return true
		}
	}
	return false
}

func isTraefikRouterRule(key string) bool {
	return strings.HasPrefix(key, "traefik.http.routers.") && strings.HasSuffix(key, ".rule")
}

// buildEndpoint creates a Gatus endpoint from container labels.
func buildEndpoint(name string, labels map[string]string, cfg *config.Config) *endpoint.Endpoint {
	// Strip leading slash from container name
	name = strings.TrimPrefix(name, "/")

	url := extractURL(labels, cfg.DockerDefaultProtocol)

	e := &endpoint.Endpoint{
		Name:       name,
		Group:      cfg.DockerDefaultGroup,
		URL:        url,
		Interval:   cfg.DefaultInterval.String(),
		Conditions: []string{defaultHTTPCondition},
	}

	// Handle deprecated gatus.endpoint blob label
	if templateYAML, ok := labels[cfg.LabelConfig]; ok && templateYAML != "" {
		slog.Warn("gatus.endpoint label is deprecated; use individual gatus.* labels instead", "container", name)
		var templateData map[string]any
		if err := yaml.Unmarshal([]byte(templateYAML), &templateData); err == nil {
			e.ApplyTemplate(templateData)
		}
	}

	// Apply individual gatus.* labels (override any blob-set values)
	applyGatusLabels(e, labels, cfg)

	// Per-container group label takes highest priority
	if group, ok := labels[cfg.LabelGroup]; ok && group != "" {
		e.Group = group
	}

	return e
}

// applyGatusLabels applies individual gatus.* labels to the endpoint.
// Labels handled elsewhere (gatus.url, gatus.enabled, gatus.group, gatus.endpoint) are skipped.
func applyGatusLabels(e *endpoint.Endpoint, labels map[string]string, cfg *config.Config) {
	// Labels handled elsewhere – skip them in this scan.
	reserved := map[string]bool{
		gatusURLLabel:    true,
		cfg.LabelEnabled: true, // e.g. "gatus.enabled"
		cfg.LabelGroup:   true, // e.g. "gatus.group"
		cfg.LabelConfig:  true, // e.g. "gatus.endpoint" (deprecated blob)
	}

	for key, value := range labels {
		if !strings.HasPrefix(key, gatusLabelPrefix) {
			continue
		}
		if reserved[key] {
			continue
		}

		// Handle gatus.headers.* – assemble the headers map from dot-separated keys.
		if strings.HasPrefix(key, gatusHeadersLabelPrefix) {
			headerKey := strings.TrimPrefix(key, gatusHeadersLabelPrefix)
			if headerKey != "" {
				if e.Headers == nil {
					e.Headers = make(map[string]string)
				}
				e.Headers[headerKey] = value
			}
			continue
		}

		suffix := strings.TrimPrefix(key, gatusLabelPrefix)
		switch suffix {
		case "interval":
			e.Interval = value
		case "method":
			e.Method = value
		case "body":
			e.Body = value
		case "graphql":
			e.GraphQL = value == stringValueTrue
		case "conditions":
			var conditions []string
			if err := json.Unmarshal([]byte(value), &conditions); err != nil {
				slog.Warn("failed to parse gatus label as JSON", "label", key, "error", err)
			} else {
				e.Conditions = conditions
			}
		case "alerts":
			var alerts []map[string]any
			if err := json.Unmarshal([]byte(value), &alerts); err != nil {
				slog.Warn("failed to parse gatus label as JSON", "label", key, "error", err)
			} else {
				e.Alerts = alerts
			}
		case "client":
			var client map[string]any
			if err := json.Unmarshal([]byte(value), &client); err != nil {
				slog.Warn("failed to parse gatus label as JSON", "label", key, "error", err)
			} else {
				e.Client = client
			}
		case "dns":
			var dns map[string]any
			if err := json.Unmarshal([]byte(value), &dns); err != nil {
				slog.Warn("failed to parse gatus label as JSON", "label", key, "error", err)
			} else {
				e.DNS = dns
			}
		case "ui":
			var ui map[string]any
			if err := json.Unmarshal([]byte(value), &ui); err != nil {
				slog.Warn("failed to parse gatus label as JSON", "label", key, "error", err)
			} else {
				e.UI = ui
			}
		case "ssh":
			var ssh map[string]any
			if err := json.Unmarshal([]byte(value), &ssh); err != nil {
				slog.Warn("failed to parse gatus label as JSON", "label", key, "error", err)
			} else {
				e.SSH = ssh
			}
		case "maintenance":
			var maintenance []map[string]any
			if err := json.Unmarshal([]byte(value), &maintenance); err != nil {
				slog.Warn("failed to parse gatus label as JSON", "label", key, "error", err)
			} else {
				e.Maintenance = maintenance
			}
		default:
			slog.Warn("ignoring unknown gatus label", "label", key)
		}
	}
}
