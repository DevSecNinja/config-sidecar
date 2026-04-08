package docker

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"

	"github.com/home-operations/gatus-sidecar/internal/config"
	"github.com/home-operations/gatus-sidecar/internal/state"
)

// fakeDockerClient implements dockerClient for unit tests.
type fakeDockerClient struct {
	// eventsFunc is called each time Events() is invoked. The test controls
	// what channels are returned on each call.
	eventsFunc func(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error)

	// containerListFunc is called for ContainerList (used by initialSync).
	containerListFunc func(ctx context.Context, options container.ListOptions) ([]container.Summary, error)

	// containerInspectFunc is called for ContainerInspect.
	containerInspectFunc func(ctx context.Context, containerID string) (container.InspectResponse, error)
}

func (f *fakeDockerClient) ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
	if f.containerListFunc != nil {
		return f.containerListFunc(ctx, options)
	}
	return nil, nil
}

func (f *fakeDockerClient) ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	if f.containerInspectFunc != nil {
		return f.containerInspectFunc(ctx, containerID)
	}
	return container.InspectResponse{}, nil
}

func (f *fakeDockerClient) Events(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error) {
	if f.eventsFunc != nil {
		return f.eventsFunc(ctx, options)
	}
	ch := make(chan events.Message)
	errCh := make(chan error)
	return ch, errCh
}

func newTestController(fc *fakeDockerClient) *Controller {
	sm := state.NewManager(filepath.Join("/tmp", "gatus-test-watchloop.yaml"))
	return &Controller{
		client:       fc,
		stateManager: sm,
	}
}

func testConfig() *config.Config {
	return &config.Config{
		DefaultInterval:       time.Minute,
		DockerDefaultProtocol: "https",
		LabelConfig:           "gatus.endpoint",
		LabelEnabled:          "gatus.enabled",
	}
}

func TestWatchLoop_ReconnectsOnTransientError(t *testing.T) {
	var callCount atomic.Int32

	fc := &fakeDockerClient{
		eventsFunc: func(ctx context.Context, _ events.ListOptions) (<-chan events.Message, <-chan error) {
			n := callCount.Add(1)
			msgCh := make(chan events.Message)
			errCh := make(chan error, 1)

			if n <= 2 {
				// First two calls fail immediately with a transient error.
				errCh <- errors.New("unexpected EOF")
			} else {
				// Third call blocks until context is cancelled.
				go func() {
					<-ctx.Done()
					errCh <- ctx.Err()
				}()
			}
			return msgCh, errCh
		},
	}

	ctrl := newTestController(fc)
	cfg := testConfig()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- ctrl.watchLoop(ctx, cfg)
	}()

	// Wait long enough for 2 reconnects (5s backoff each) plus some margin.
	// Use a polling approach so the test doesn't sleep unnecessarily long.
	deadline := time.After(15 * time.Second)
	for {
		if callCount.Load() >= 3 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for reconnects; Events called %d times", callCount.Load())
		case <-time.After(100 * time.Millisecond):
		}
	}

	cancel()

	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("watchLoop returned unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("watchLoop did not exit after context cancellation")
	}

	if got := callCount.Load(); got < 3 {
		t.Errorf("expected Events to be called at least 3 times, got %d", got)
	}
}

func TestWatchLoop_ReSyncsAfterError(t *testing.T) {
	var syncCount atomic.Int32

	fc := &fakeDockerClient{
		containerListFunc: func(ctx context.Context, _ container.ListOptions) ([]container.Summary, error) {
			syncCount.Add(1)
			return nil, nil
		},
		eventsFunc: func(ctx context.Context, _ events.ListOptions) (<-chan events.Message, <-chan error) {
			msgCh := make(chan events.Message)
			errCh := make(chan error, 1)
			// Always fail so we can observe re-syncs.
			errCh <- errors.New("connection reset")
			return msgCh, errCh
		},
	}

	ctrl := newTestController(fc)
	cfg := testConfig()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- ctrl.watchLoop(ctx, cfg)
	}()

	// Wait for at least 1 re-sync (initialSync is called inside watchLoop on error).
	deadline := time.After(10 * time.Second)
	for {
		if syncCount.Load() >= 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for re-sync; ContainerList called %d times", syncCount.Load())
		case <-time.After(100 * time.Millisecond):
		}
	}

	cancel()
	<-done

	if got := syncCount.Load(); got < 1 {
		t.Errorf("expected initialSync (ContainerList) to be called at least once after error, got %d", got)
	}
}

func TestWatchLoop_ReturnsOnContextCancel(t *testing.T) {
	fc := &fakeDockerClient{
		eventsFunc: func(ctx context.Context, _ events.ListOptions) (<-chan events.Message, <-chan error) {
			msgCh := make(chan events.Message)
			errCh := make(chan error, 1)
			go func() {
				<-ctx.Done()
				errCh <- ctx.Err()
			}()
			return msgCh, errCh
		},
	}

	ctrl := newTestController(fc)
	cfg := testConfig()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- ctrl.watchLoop(ctx, cfg)
	}()

	// Cancel immediately.
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("watchLoop did not exit after context cancellation")
	}
}

func TestWatchLoop_ReturnsNilOnCleanClose(t *testing.T) {
	fc := &fakeDockerClient{
		eventsFunc: func(ctx context.Context, _ events.ListOptions) (<-chan events.Message, <-chan error) {
			msgCh := make(chan events.Message)
			errCh := make(chan error)
			// Close both channels to simulate a clean shutdown.
			close(msgCh)
			close(errCh)
			return msgCh, errCh
		},
	}

	ctrl := newTestController(fc)
	cfg := testConfig()

	ctx := context.Background()

	done := make(chan error, 1)
	go func() {
		done <- ctrl.watchLoop(ctx, cfg)
	}()

	select {
	case err := <-done:
		// A closed errCh yields the zero value (nil), so watchEvents returns nil,
		// and watchLoop returns nil.
		if err != nil {
			t.Errorf("expected nil error on clean close, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("watchLoop did not return on clean channel close")
	}
}
