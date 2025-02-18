package sse_server

import (
	"log/slog"
	"net/http"
	"os"
	"time"
)

const heartbeatIntervalDefault = 20 * time.Second

type Options struct {
	Port              int
	Handlers          map[string]http.HandlerFunc
	HeartbeatInterval time.Duration
	Logger            *slog.Logger
	// BufferSize defines how big the channel for each connection is as slow consumers will get their messages dropped.
	// Default value is 1.
	BufferSize int
}

type OptionsFn func(o *Options)

func WithPort(port int) OptionsFn {
	return func(o *Options) {
		o.Port = port
	}
}

func newUpdatedOptions(options *Options) *Options {
	updatedOptions := &Options{
		Port:              0,
		Handlers:          nil,
		HeartbeatInterval: 0,
		Logger:            nil,
		BufferSize:        0,
	}

	if options != nil {
		if options.HeartbeatInterval > 0 {
			updatedOptions.HeartbeatInterval = options.HeartbeatInterval
		} else {
			updatedOptions.HeartbeatInterval = heartbeatIntervalDefault
		}

		if options.Logger != nil {
			updatedOptions.Logger = options.Logger
		} else {
			updatedOptions.Logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
		}

		if options.Port > 0 {
			updatedOptions.Port = options.Port
		} else {
			updatedOptions.Port = 3000
		}

		if options.BufferSize > 0 {
			updatedOptions.BufferSize = options.BufferSize
		} else {
			updatedOptions.BufferSize = 1
		}

		updatedOptions.Handlers = options.Handlers
	}

	return updatedOptions
}
