package tests

import (
	"context"
	"fmt"
	"github.com/doppelganger113/ssevents"
	"log/slog"
	"net/http"
	"os"
)

type TestBootstrapOptions struct {
	logger *slog.Logger
}

// BootstrapClientAndServer handles boilerplate set up of server and client for testing environment, by default logs
// only on errors, override logger for debug and info logs.
func BootstrapClientAndServer(options *TestBootstrapOptions) (
	*ssevents.Client, *ssevents.Server, func(ctx context.Context) error, error,
) {
	// Errors only logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	if options != nil {
		if options.logger != nil {
			logger = options.logger
		}
	}

	// Start server
	server, err := ssevents.NewServer(&ssevents.Options{
		Handlers: map[string]http.HandlerFunc{},
		Logger:   logger,
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed starting server: %w", err)
	}

	url, _, err := server.ListenAndServeOnRandomPort()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed establishing server on a random port: %w", err)
	}

	// Start client
	client, err := ssevents.NewSSEClient(url+"/sse", &ssevents.ClientOptions{Logger: logger})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed starting client: %w", err)
	}

	shutdownFn := func(ctx context.Context) error {
		client.Shutdown()
		return server.Shutdown(ctx)
	}

	return client, server, shutdownFn, nil
}
