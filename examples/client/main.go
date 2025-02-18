package main

import (
	"fmt"
	"github.com/doppelganger113/sse-server"
	"log"
	"log/slog"
)

func main() {
	sseURL := "http://localhost:3000/sse"
	c, err := sse_server.NewSSEClient(sseURL, nil)
	defer c.Shutdown()
	if err != nil {
		log.Fatalln(err)
	}
	c.Start()

	sigTerm := sse_server.WatchSigTerm()

	slog.Info("client started")
	// Read from channels
	for {
		select {
		case <-sigTerm:
			slog.Info("Received shutdown signal")
			return
		case errCh, ok := <-c.Errors():
			if !ok {
				fmt.Println("SSE connection closed.")
				return
			}
			slog.Error("received error: " + errCh.Error())
		case event, ok := <-c.Events():
			if !ok {
				fmt.Println("SSE connection closed.")
				return
			}
			fmt.Printf("Received EventCh: %+v\n", event)
		}
	}
}
