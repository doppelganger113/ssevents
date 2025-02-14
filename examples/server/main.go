package main

import (
	"context"
	"errors"
	"flag"
	"github.com/doppelganger113/sse-server"
	"github.com/doppelganger113/sse-server/internal/fe"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"
)

var (
	port = flag.Int("port", 3000, "port of the server")
)

func init() {
	flag.Parse()
}

func logErrorAndExit(err error) {
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func main() {
	handlers := make(map[string]http.HandlerFunc)
	handlers["GET /"] = func(w http.ResponseWriter, req *http.Request) {
		// Only main path (home)
		if req.URL.String() == "/" {
			_, _ = w.Write(fe.IndexFile)
		}
	}

	srvr, err := sse_server.New(sse_server.Options{Port: *port, Handlers: handlers})
	if err != nil {
		logErrorAndExit(err)
	}

	serverErr := make(chan error)
	go func() {
		slog.Info("Started server on port :" + strconv.Itoa(*port))
		serverErr <- srvr.ListenAndServe()
	}()

	select {
	case err = <-serverErr:
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		logErrorAndExit(errors.Join(err, srvr.Shutdown(ctx)))
	case _ = <-sse_server.WatchSigTerm():
		slog.Info("shut down signal received")
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		logErrorAndExit(srvr.Shutdown(ctx))
	}
}
