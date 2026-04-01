package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/docker/docker/client"
	"github.com/home-operations/gatus-sidecar/internal/config"
	"github.com/home-operations/gatus-sidecar/internal/controller"
	"github.com/home-operations/gatus-sidecar/internal/docker"
	"github.com/home-operations/gatus-sidecar/internal/resources/httproute"
	"github.com/home-operations/gatus-sidecar/internal/resources/ingress"
	"github.com/home-operations/gatus-sidecar/internal/resources/ingressroute"
	"github.com/home-operations/gatus-sidecar/internal/resources/service"
	"github.com/home-operations/gatus-sidecar/internal/state"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	cfg := config.Load()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	switch cfg.Mode {
	case "docker":
		if err := runDocker(ctx, cfg); err != nil {
			slog.Error("Docker mode failed", "error", err)
			os.Exit(1)
		}
	default:
		if err := runKubernetes(ctx, cfg); err != nil {
			slog.Error("Kubernetes mode failed", "error", err)
			os.Exit(1)
		}
	}

	slog.Info("All controllers have finished successfully")
}

func runDocker(ctx context.Context, cfg *config.Config) error {
	opts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}
	if cfg.DockerHost != "" {
		opts = append(opts, client.WithHost(cfg.DockerHost))
	}

	dockerClient, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return fmt.Errorf("create docker client: %w", err)
	}
	defer dockerClient.Close()

	stateManager := state.NewManager(cfg.Output)
	ctrl := docker.New(stateManager, dockerClient)

	slog.Info("starting in docker mode")
	return ctrl.Run(ctx, cfg)
}

func runKubernetes(ctx context.Context, cfg *config.Config) error {
	restCfg, err := getKubeConfig()
	if err != nil {
		return fmt.Errorf("get kubernetes config: %w", err)
	}

	dc, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return fmt.Errorf("create dynamic client: %w", err)
	}

	stateManager := state.NewManager(cfg.Output)

	// Initialize controllers slice
	controllers := []*controller.Controller{}

	// Determine if default controllers should be enabled
	defaultControllers := !cfg.EnableHTTPRoute && !cfg.EnableIngress && !cfg.EnableService && !cfg.EnableIngressRoute

	// Conditionally register controllers based on config
	if cfg.EnableHTTPRoute || cfg.AutoHTTPRoute || defaultControllers {
		controllers = append(controllers, controller.New(httproute.Definition(), stateManager, dc))
	}
	if cfg.EnableIngress || cfg.AutoIngress || defaultControllers {
		controllers = append(controllers, controller.New(ingress.Definition(), stateManager, dc))
	}
	if cfg.EnableService || cfg.AutoService || defaultControllers {
		controllers = append(controllers, controller.New(service.Definition(), stateManager, dc))
	}
	if cfg.EnableIngressRoute || cfg.AutoIngressRoute || defaultControllers {
		controllers = append(controllers, controller.New(ingressroute.Definition(), stateManager, dc))
	}

	// If no controllers are enabled, log a warning and exit
	if len(controllers) == 0 {
		slog.Warn("No controllers enabled. Exiting.")
		return nil
	}

	// Run all controllers concurrently
	return runControllers(ctx, cfg, controllers)
}

func runControllers(ctx context.Context, cfg *config.Config, controllers []*controller.Controller) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(controllers))

	for _, c := range controllers {
		wg.Go(func() {
			slog.Info("Starting controller", "controller", c.GetResource())

			if err := c.Run(ctx, cfg); err != nil {
				slog.Error("Controller error", "controller", c.GetResource(), "error", err)
				errChan <- err
			}
		})
	}

	// Wait for either all controllers to finish or an error to occur
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Return the first error encountered
	select {
	case err := <-errChan:
		if err != nil {
			return err
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

func getKubeConfig() (*rest.Config, error) {
	// Check if we're running in a cluster by looking for the service host env var
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("in-cluster config: %w", err)
		}
		slog.Info("using in-cluster kubernetes config")
		return cfg, nil
	}

	// Fall back to kubeconfig for local development
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	cfg, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("kubeconfig: %w", err)
	}

	slog.Info("using kubeconfig")
	return cfg, nil
}
