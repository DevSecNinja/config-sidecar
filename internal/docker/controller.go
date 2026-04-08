package docker

import (
	"context"
	"log/slog"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"

	"github.com/home-operations/gatus-sidecar/internal/config"
	"github.com/home-operations/gatus-sidecar/internal/state"
)

// dockerClient is the subset of the Docker API used by the controller.
type dockerClient interface {
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
	Events(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error)
}

// Controller watches Docker containers and generates Gatus endpoints.
type Controller struct {
	client       dockerClient
	stateManager *state.Manager
}

// New creates a new Docker controller.
func New(stateManager *state.Manager, dockerClient *client.Client) *Controller {
	return &Controller{
		client:       dockerClient,
		stateManager: stateManager,
	}
}

// Run starts the Docker controller: performs an initial sync, then watches for events.
func (c *Controller) Run(ctx context.Context, cfg *config.Config) error {
	if err := c.initialSync(ctx, cfg); err != nil {
		return err
	}
	return c.watchLoop(ctx, cfg)
}

func (c *Controller) initialSync(ctx context.Context, cfg *config.Config) error {
	containers, err := c.client.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return err
	}

	changed := false
	for _, ctr := range containers {
		labels := ctr.Labels
		if !hasTraefikRouter(labels) && labels["gatus.url"] == "" {
			continue
		}
		if !isEnabled(labels, cfg.LabelEnabled) {
			continue
		}

		name := containerName(ctr.Names)
		key := stateKey(ctr.ID)
		e := buildEndpoint(name, labels, cfg)
		if c.stateManager.AddOrUpdate(key, e, false) {
			slog.Info("added docker container endpoint", "name", name, "id", ctr.ID[:12])
			changed = true
		}
	}

	if changed {
		c.stateManager.ForceWrite()
	}

	slog.Info("docker initial sync complete", "containers", len(containers))
	return nil
}

func (c *Controller) watchLoop(ctx context.Context, cfg *config.Config) error {
	f := filters.NewArgs()
	f.Add("type", string(events.ContainerEventType))
	f.Add("event", "start")
	f.Add("event", "stop")
	f.Add("event", "die")
	f.Add("event", "destroy")

	for {
		if err := c.watchEvents(ctx, cfg, f); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Warn("docker event stream error, reconnecting in 5s", "error", err)

			if syncErr := c.initialSync(ctx, cfg); syncErr != nil {
				slog.Error("re-sync after event stream error failed", "error", syncErr)
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
			continue
		}
		return nil
	}
}

func (c *Controller) watchEvents(ctx context.Context, cfg *config.Config, f filters.Args) error {
	eventCh, errCh := c.client.Events(ctx, events.ListOptions{Filters: f})
	slog.Info("watching docker events")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			return err
		case event := <-eventCh:
			c.handleEvent(ctx, cfg, event)
		}
	}
}

func (c *Controller) handleEvent(ctx context.Context, cfg *config.Config, event events.Message) {
	containerID := event.Actor.ID
	action := string(event.Action)
	key := stateKey(containerID)

	switch action {
	case "start":
		info, err := c.client.ContainerInspect(ctx, containerID)
		if err != nil {
			slog.Error("failed to inspect container", "id", containerID[:12], "error", err)
			return
		}

		labels := info.Config.Labels
		if !hasTraefikRouter(labels) && labels["gatus.url"] == "" {
			return
		}
		if !isEnabled(labels, cfg.LabelEnabled) {
			return
		}

		name := info.Name
		e := buildEndpoint(name, labels, cfg)
		if c.stateManager.AddOrUpdate(key, e, true) {
			slog.Info("docker container started", "name", name, "id", containerID[:12])
		}

	case "stop", "die", "destroy":
		if c.stateManager.Remove(key) {
			slog.Info("docker container removed", "action", action, "id", containerID[:12])
		}
	}
}

func stateKey(containerID string) string {
	if len(containerID) > 12 {
		return "container-" + containerID[:12]
	}
	return "container-" + containerID
}

func containerName(names []string) string {
	if len(names) > 0 {
		return names[0]
	}
	return "unknown"
}
