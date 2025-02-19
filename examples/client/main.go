package main

import (
	"flag"
	"fmt"
	"github.com/doppelganger113/ssevents"
	"log/slog"
	"os"
)

// flags
var (
	logLevelFlag = flag.String("log-level", "info", "logging level, types: debug,info,warn,error")
)

var (
	logLevel slog.Level
	log      *slog.Logger
)

func init() {
	flag.Parse()

	switch *logLevelFlag {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		slog.Info(fmt.Sprintf("Unknown log level %s defaulting to info", *logLevelFlag))
		logLevel = slog.LevelInfo
	}
	log = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
}

func main() {
	sseURL := "http://localhost:3000/sse"
	c, err := ssevents.NewSSEClient(sseURL, nil)
	if err != nil {
		log.Error("failed creating sse client", "err", err)
		os.Exit(1)
	}
	defer c.Shutdown()

	c.Start()

	sigTerm := ssevents.WatchSigTerm()

	log.Info("client started")
	// Read from channels
	for {
		select {
		case <-sigTerm:
			log.Info("shut down signal received")
			return
		case errCh, ok := <-c.Errors():
			if !ok {
				log.Info("error channel closed, stopping")
				return
			}
			log.Error("received error", "err", errCh)
		case event, ok := <-c.Events():
			if !ok {
				log.Info("events channel closed")
				return
			}
			log.Info("received an event", "event", event)
		}
	}
}
