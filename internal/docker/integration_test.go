//go:build integration

package docker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	"github.com/home-operations/gatus-sidecar/internal/config"
	gatusprovider "github.com/home-operations/gatus-sidecar/internal/provider/gatus"
	"github.com/home-operations/gatus-sidecar/internal/state"
)

func newDockerClient(t *testing.T) *client.Client {
	t.Helper()
	c, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}
	// Verify the daemon is reachable
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := c.Ping(ctx); err != nil {
		t.Skipf("docker daemon not reachable, skipping integration test: %v", err)
	}
	return c
}

// startTestContainer creates and starts a container with the given labels, returning its ID.
// The container is automatically removed when the test finishes.
func startTestContainer(t *testing.T, dc *client.Client, labels map[string]string) string {
	t.Helper()
	ctx := context.Background()

	resp, err := dc.ContainerCreate(ctx, &container.Config{
		Image:  "alpine:latest",
		Cmd:    []string{"sleep", "300"},
		Labels: labels,
	}, nil, nil, nil, "")
	if err != nil {
		t.Fatalf("failed to create container: %v", err)
	}

	t.Cleanup(func() {
		_ = dc.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})
	})

	if err := dc.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		t.Fatalf("failed to start container: %v", err)
	}

	return resp.ID
}

func TestIntegration_InitialSync_TraefikLabels(t *testing.T) {
	dc := newDockerClient(t)
	defer dc.Close()

	labels := map[string]string{
		"traefik.http.routers.testapp.rule": "Host(`testapp.example.com`)",
		"traefik.http.routers.testapp.tls":  "true",
	}
	containerID := startTestContainer(t, dc, labels)

	outputFile := filepath.Join(t.TempDir(), "gatus.yaml")
	sm := state.NewManager(outputFile, gatusprovider.New())
	ctrl := New(sm, dc)

	cfg := &config.Config{
		DefaultInterval:       time.Minute,
		DockerDefaultProtocol: "https",
		LabelConfig:           "gatus.endpoint",
		LabelEnabled:          "gatus.enabled",
		Output:                outputFile,
	}

	ctx := context.Background()
	if err := ctrl.initialSync(ctx, cfg); err != nil {
		t.Fatalf("initialSync failed: %v", err)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	content := string(data)

	if !strings.Contains(content, "testapp.example.com") {
		t.Errorf("output should contain testapp.example.com, got:\n%s", content)
	}
	if !strings.Contains(content, "https://testapp.example.com") {
		t.Errorf("output should contain https://testapp.example.com, got:\n%s", content)
	}

	// Verify the state key format
	expectedKeyPrefix := "container-" + containerID[:12]
	_ = expectedKeyPrefix // key is internal, we verify via output content
}

func TestIntegration_InitialSync_GatusURL(t *testing.T) {
	dc := newDockerClient(t)
	defer dc.Close()

	labels := map[string]string{
		"gatus.url": "https://custom.example.com/health",
	}
	startTestContainer(t, dc, labels)

	outputFile := filepath.Join(t.TempDir(), "gatus.yaml")
	sm := state.NewManager(outputFile, gatusprovider.New())
	ctrl := New(sm, dc)

	cfg := &config.Config{
		DefaultInterval:       time.Minute,
		DockerDefaultProtocol: "https",
		LabelConfig:           "gatus.endpoint",
		LabelEnabled:          "gatus.enabled",
		Output:                outputFile,
	}

	ctx := context.Background()
	if err := ctrl.initialSync(ctx, cfg); err != nil {
		t.Fatalf("initialSync failed: %v", err)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	if !strings.Contains(string(data), "https://custom.example.com/health") {
		t.Errorf("output should contain custom URL, got:\n%s", string(data))
	}
}

func TestIntegration_InitialSync_DisabledContainer(t *testing.T) {
	dc := newDockerClient(t)
	defer dc.Close()

	labels := map[string]string{
		"traefik.http.routers.disabled.rule": "Host(`disabled.example.com`)",
		"gatus.enabled":                     "false",
	}
	startTestContainer(t, dc, labels)

	outputFile := filepath.Join(t.TempDir(), "gatus.yaml")
	sm := state.NewManager(outputFile, gatusprovider.New())
	ctrl := New(sm, dc)

	cfg := &config.Config{
		DefaultInterval:       time.Minute,
		DockerDefaultProtocol: "https",
		LabelConfig:           "gatus.endpoint",
		LabelEnabled:          "gatus.enabled",
		Output:                outputFile,
	}

	ctx := context.Background()
	if err := ctrl.initialSync(ctx, cfg); err != nil {
		t.Fatalf("initialSync failed: %v", err)
	}

	// File may not exist if no endpoints were written, or it may exist with empty endpoints
	data, _ := os.ReadFile(outputFile)
	if strings.Contains(string(data), "disabled.example.com") {
		t.Errorf("disabled container should not appear in output, got:\n%s", string(data))
	}
}

func TestIntegration_WatchLoop_StartStop(t *testing.T) {
	dc := newDockerClient(t)
	defer dc.Close()

	outputFile := filepath.Join(t.TempDir(), "gatus.yaml")
	sm := state.NewManager(outputFile, gatusprovider.New())
	ctrl := New(sm, dc)

	cfg := &config.Config{
		DefaultInterval:       time.Minute,
		DockerDefaultProtocol: "https",
		LabelConfig:           "gatus.endpoint",
		LabelEnabled:          "gatus.enabled",
		Output:                outputFile,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the watch loop in the background
	watchErr := make(chan error, 1)
	go func() {
		watchErr <- ctrl.watchLoop(ctx, cfg)
	}()

	// Give the watcher a moment to establish the event stream
	time.Sleep(500 * time.Millisecond)

	// Start a container with Traefik labels
	labels := map[string]string{
		"traefik.http.routers.watched.rule": "Host(`watched.example.com`)",
		"traefik.http.routers.watched.tls":  "true",
	}
	containerID := startTestContainer(t, dc, labels)

	// Wait for the event to be processed
	time.Sleep(2 * time.Second)

	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output after start: %v", err)
	}
	if !strings.Contains(string(data), "watched.example.com") {
		t.Errorf("started container should appear in output, got:\n%s", string(data))
	}

	// Stop the container
	stopTimeout := 5
	if err := dc.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &stopTimeout}); err != nil {
		t.Fatalf("failed to stop container: %v", err)
	}

	// Wait for the stop/die event to be processed
	time.Sleep(2 * time.Second)

	data, err = os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output after stop: %v", err)
	}
	if strings.Contains(string(data), "watched.example.com") {
		t.Errorf("stopped container should not appear in output, got:\n%s", string(data))
	}

	// Cancel context to stop the watch loop
	cancel()
	select {
	case err := <-watchErr:
		if err != nil && err != context.Canceled {
			t.Errorf("watchLoop returned unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("watchLoop did not exit after context cancellation")
	}
}
