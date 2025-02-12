package main

import (
	"context"
	"fmt"
	"github.com/doppelganger113/sse-server/internal/util"
	"github.com/doppelganger113/sse-server/sse"
	"log"
	"log/slog"
)

func main() {
	sseURL := "http://localhost:3000/sse"
	c, err := sse.NewSSEClient(sseURL)
	defer c.Shutdown()
	if err != nil {
		log.Fatalln(err)
	}
	c.Start(context.Background())

	sigTerm := util.WatchSigTerm()

	slog.Info("client started")
	// Read from channels
	for {
		select {
		case <-sigTerm:
			slog.Info("Received shutdown signal")
			return
		case errCh, ok := <-c.Error:
			if !ok {
				fmt.Println("SSE connection closed.")
				return
			}
			slog.Error("received error: " + errCh.Error())
		case event, ok := <-c.Event:
			if !ok {
				fmt.Println("SSE connection closed.")
				return
			}
			fmt.Printf("Received Event: %+v\n", event)
		}
	}
}
