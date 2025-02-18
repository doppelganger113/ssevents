package tests

import (
	"context"
	"fmt"
	sseserver "github.com/doppelganger113/sse-server"
	"log/slog"
	"net/http"
	"time"
)

var heartbeatIntervalDefault = 20 * time.Second

type Bootstrap struct {
	Heartbeat time.Duration
}

type BootstrapOptions func(bootstrap *Bootstrap)

func WithHeartbeatInterval(intervalMillis time.Duration) BootstrapOptions {
	return func(bootstrap *Bootstrap) {
		bootstrap.Heartbeat = intervalMillis
	}
}

func BootstrapClientAndServer(options ...BootstrapOptions) (
	*sseserver.Client, *sseserver.Server, func(ctx context.Context) error, error,
) {
	// Start server
	server, err := sseserver.New(&sseserver.Options{
		Handlers: map[string]http.HandlerFunc{},
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed starting server: %w", err)
	}

	url, _, err := server.ListenAndServeOnRandomPort()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed establishing server on a random port: %w", err)
	}

	// Start client
	client, err := sseserver.NewSSEClient(url+"/sse", nil)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed starting client: %w", err)
	}

	shutdownFn := func(ctx context.Context) error {
		slog.Info("Shutting down")
		client.Shutdown()
		return server.Shutdown(ctx)
	}

	return client, server, shutdownFn, nil
}
