// Package tests verifies that the library is usable for testing with the usage of SSE server through the client
package tests

import (
	"context"
	"fmt"
	sseserver "github.com/doppelganger113/sse-server"
	"log/slog"
	"testing"
	"time"
)

func TestSendingAnEvent(t *testing.T) {
	const numberOfSentMessages = 5

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, server, shutdown, err := BootstrapClientAndServer()
	if err != nil {
		t.Error(err)
	}
	defer func() {
		if shutdownErr := shutdown(ctx); shutdownErr != nil {
			t.Error(shutdownErr)
		}
	}()

	// Reads all non heartbeat events
	consumerAllObserver := client.Subscribe(
		sseserver.NewObserverBuilder().
			NoHeartbeat().
			Buffer(10).
			Limit(5).
			Build(),
	)

	onlyFirstObserver := client.Subscribe(
		sseserver.NewObserverBuilder().
			NoHeartbeat().
			First().
			Build(),
	)

	//
	//_ = client.Subscribe(
	//	sseserver.NewObserverBuilder().
	//		NoHeartbeat().
	//		Buffer(10).
	//		Build(),
	//)
	//go func() {
	//	for event := range eventsCh3 {
	//		log.Println("third", event)
	//	}
	//}()

	//eventCh4 := client.Subscribe(
	//	sseserver.NewObserverBuilder().
	//		NoHeartbeat().
	//		Limit(3).
	//		Buffer(10).
	//		Build(),
	//)
	//go func() {
	//	for event := range eventCh4 {
	//		log.Println("fourth", event)
	//	}
	//}()

	slog.Info("Starting...")
	client.Start(ctx)
	slog.Info("Started")

	for i := 0; i < numberOfSentMessages; i++ {
		slog.Info("Emitting...")
		server.Emit(sseserver.Event{Data: fmt.Sprintf("Message {%d}", i)})
	}

	var counter int
	for evt := range consumerAllObserver {
		counter++
		fmt.Println("Basic operator " + evt.String())
	}
	if counter != 5 {
		t.Errorf("failed basic observer, expected 5 got %d events", counter)
	}

	var counter2 int
	for evt := range onlyFirstObserver {
		counter2++
		fmt.Println("Only first operator", evt)
	}
	if counter2 != 1 {
		t.Errorf("failed only first observer, was expect 1, got %d events", counter2)
	}
	fmt.Printf("onlyFirstObserver: %d\n", counter2)

	//
	//time.Sleep(5 * time.Second)
	//for {
	//	select {
	//	case evt := <-eventsCh:
	//		t.Log(evt)
	//	case <-ctx.Done():
	//		return
	//	case serverErr := <-errCh:
	//		if serverErr != nil {
	//			t.Error(serverErr)
	//		}
	//		return
	//	}
	//}
}
