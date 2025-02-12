package main

import (
	"context"
	"errors"
	"flag"
	"github.com/doppelganger113/sse-server/internal/server"
	"github.com/doppelganger113/sse-server/internal/util"
	"log/slog"
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
	srvr, err := server.New(server.Options{Port: *port})
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
	case _ = <-util.WatchSigTerm():
		slog.Info("shut down signal received")
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		logErrorAndExit(srvr.Shutdown(ctx))
	}
}
