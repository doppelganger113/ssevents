package ssevents

import (
	"log/slog"
	"net/http"
	"os"
	"time"
)

const heartbeatIntervalDefault = 20 * time.Second

type Options struct {
	// Port defines the port on which to run the server
	Port int
	// Handlers are used for adding new endpoints
	Handlers map[string]http.HandlerFunc
	// HeartbeatInterval defines on which interval a heartbeat is sent to connected clients
	HeartbeatInterval time.Duration
	// Logger to be used, default is stdout text
	Logger *slog.Logger
	// Overrides the default SSE url /sse
	SseUrl string
	// EmitStrategy option defines what to do on slow consumers as they can block/slow emission to others,
	// default is EmitStrategyBlock.
	EmitStrategy EmitStrategy
	// BufferSize defines how big the channel for each connection is as slow consumers will get their messages dropped.
	// Default value is 1 and is used in conjunction with EmitStrategy when buffering is set.
	BufferSize int
}

func newUpdatedOptions(options *Options) *Options {
	updatedOptions := &Options{
		Port:              3000,
		Handlers:          nil,
		HeartbeatInterval: heartbeatIntervalDefault,
		Logger:            slog.New(slog.NewTextHandler(os.Stdout, nil)),
		BufferSize:        1,
		EmitStrategy:      EmitStrategyBlock,
	}

	if options != nil {
		if options.HeartbeatInterval > 0 {
			updatedOptions.HeartbeatInterval = options.HeartbeatInterval
		}

		if options.Logger != nil {
			updatedOptions.Logger = options.Logger
		}

		if options.Port > 0 {
			updatedOptions.Port = options.Port
		}

		if options.BufferSize > 0 {
			updatedOptions.BufferSize = options.BufferSize
		}

		updatedOptions.Handlers = options.Handlers
		updatedOptions.SseUrl = options.SseUrl
		updatedOptions.EmitStrategy = options.EmitStrategy
	}

	return updatedOptions
}
