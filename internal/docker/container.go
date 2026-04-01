package docker

import (
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/home-operations/gatus-sidecar/internal/config"
	"github.com/home-operations/gatus-sidecar/internal/endpoint"
)

var hostRegex = regexp.MustCompile("Host\\(`([^`]+)`\\)")

// extractURL determines the URL for a container from its labels.
// If a "gatus.url" label exists, it is used directly.
// Otherwise, it scans for Traefik router rules to extract the hostname and protocol.
func extractURL(labels map[string]string, defaultProtocol string) string {
	if url, ok := labels["gatus.url"]; ok && url != "" {
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
		if tlsVal, ok := labels[tlsKey]; ok && (tlsVal == "true" || tlsVal == "") {
			protocol = "https"
		}

		// Check entrypoints for websecure/https
		epKey := "traefik.http.routers." + routerName + ".entrypoints"
		if epVal, ok := labels[epKey]; ok {
			lower := strings.ToLower(epVal)
			if strings.Contains(lower, "websecure") || strings.Contains(lower, "https") {
				protocol = "https"
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
	return val != "false" && val != "0"
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
		URL:        url,
		Interval:   cfg.DefaultInterval.String(),
		Conditions: []string{"[STATUS] == 200"},
	}

	// Apply template override from label
	if templateYAML, ok := labels[cfg.LabelConfig]; ok && templateYAML != "" {
		var templateData map[string]any
		if err := yaml.Unmarshal([]byte(templateYAML), &templateData); err == nil {
			e.ApplyTemplate(templateData)
		}
	}

	return e
}
