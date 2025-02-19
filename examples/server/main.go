package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/doppelganger113/sse-server"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"
)

import _ "embed"

//go:embed index.html
var IndexFile []byte

// flags
var (
	logLevelFlag = flag.String("log-level", "info", "logging level, types: debug,info,warn,error")
	port         = flag.Int("port", 3000, "port of the server")
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

func logErrorAndExit(err error) {
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}

func main() {
	handlers := make(map[string]http.HandlerFunc)

	// Serve our webpage that will connect via JavaScript to listen for SSE
	handlers["GET /"] = func(w http.ResponseWriter, req *http.Request) {
		// Only main path (home)
		if req.URL.String() == "/" {
			_, _ = w.Write(IndexFile)
		}
	}

	srvr, err := sse_server.New(&sse_server.Options{Port: *port, Handlers: handlers})
	if err != nil {
		logErrorAndExit(err)
	}

	serverErr := make(chan error)
	go func() {
		log.Info("Started server on port :" + strconv.Itoa(*port))
		serverErr <- srvr.ListenAndServe()
	}()

	select {
	case err = <-serverErr:
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		logErrorAndExit(errors.Join(err, srvr.Shutdown(ctx)))
	case <-sse_server.WatchSigTerm():
		log.Info("shut down signal received")
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		logErrorAndExit(srvr.Shutdown(ctx))
	}
}
