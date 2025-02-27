// Package tests verifies that the library is usable for testing with the usage of SSE server through the client
package tests

import (
	"context"
	"errors"
	"fmt"
	"github.com/doppelganger113/ssevents"
	"sync"
	"testing"
	"time"
)

func Test_givenMultipleObserver_withLimit_thenConsumeLimitAndComplete(t *testing.T) {
	const numberOfSentMessages = 5

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, server, shutdown, err := BootstrapClientAndServer(nil)
	if err != nil {
		t.Error(err)
	}
	defer func() {
		if shutdownErr := shutdown(ctx); shutdownErr != nil {
			t.Error(shutdownErr)
		}
	}()

	const numOfObservers = 3
	var observers []*ssevents.Observer

	for i := 0; i < numOfObservers; i++ {
		obs := client.Subscribe(
			ssevents.NewObserverBuilder().
				Limit(4).
				Build(),
		)
		observers = append(observers, obs)
	}

	client.Start()

	consumerAllResult := make(chan []ssevents.Event, numOfObservers)

	var wg sync.WaitGroup

	for i := 0; i < numOfObservers; i++ {
		wg.Add(1)
		go func(o *ssevents.Observer) {
			defer wg.Done()
			consumerAllResult <- o.WaitForAll()
		}(observers[i])
	}

	for i := 0; i < numberOfSentMessages; i++ {
		server.Emit(ssevents.Event{Data: fmt.Sprintf("Message {%d}", i)})
	}

	wg.Wait()
	close(consumerAllResult)

	var results [][]ssevents.Event
	for events := range consumerAllResult {
		results = append(results, events)
	}

	for _, result := range results {
		if 4 != len(result) {
			t.Errorf("failed basic observer, expected %d got %d events", numberOfSentMessages, len(result))
		}
	}
}

func Test_givenObserver_whenWaitingForFirstOnly_thenConsumeOneAndComplete(t *testing.T) {
	const numberOfSentMessages = 5

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, server, shutdown, err := BootstrapClientAndServer(nil)
	if err != nil {
		t.Error(err)
	}
	defer func() {
		if shutdownErr := shutdown(ctx); shutdownErr != nil {
			t.Error(shutdownErr)
		}
	}()

	// Reads all non heartbeat events
	observer := client.Subscribe(
		ssevents.NewObserverBuilder().
			First().
			Build(),
	)
	client.Start()

	consumerAllResult := make(chan int)
	go func() {
		var counter int
		for range observer.EventCh {
			counter++
		}
		consumerAllResult <- counter
	}()

	for i := 0; i < numberOfSentMessages; i++ {
		server.Emit(ssevents.Event{Data: fmt.Sprintf("Message {%d}", i)})
	}

	result := <-consumerAllResult
	if 1 != result {
		t.Errorf("failed basic observer, expected 1 got %d events", result)
	}
}

func Test_givenObserver_whenBufferAndLimit_thenHandleInSameThreadAndComplete(t *testing.T) {
	const numberOfSentMessages = 5

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, server, shutdown, err := BootstrapClientAndServer(nil)
	if err != nil {
		t.Error(err)
	}
	defer func() {
		if shutdownErr := shutdown(ctx); shutdownErr != nil {
			t.Error(shutdownErr)
		}
	}()

	// Reads all non heartbeat events
	observer := client.Subscribe(
		ssevents.NewObserverBuilder().
			Buffer(5).
			Limit(5).
			Build(),
	)

	client.Start()

	for i := 0; i < numberOfSentMessages; i++ {
		server.Emit(ssevents.Event{Data: fmt.Sprintf("Message {%d}", i)})
	}

	result := observer.WaitForAll()

	if numberOfSentMessages != len(result) {
		t.Errorf("failed basic observer, expected %d got %d events", numberOfSentMessages, len(result))
	}
}

func Test_givenObserverNoBuffer_whenNotAllEventsArrive_thenBlockChannelReading(t *testing.T) {
	const numberOfSentMessages = 4

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()

	client, server, shutdown, err := BootstrapClientAndServer(nil)
	if err != nil {
		t.Error(err)
	}
	defer func() {
		shutdownErr := shutdown(shutdownCtx)
		if shutdownErr != nil && !errors.Is(shutdownErr, context.DeadlineExceeded) {
			t.Error(shutdownErr.Error())
		}
	}()

	observer := client.Subscribe(
		ssevents.NewObserverBuilder().
			Buffer(0).
			Limit(5).
			Build(),
	)

	client.Start()

	resultCh := make(chan []ssevents.Event)
	go func() {
		resultCh <- observer.WaitForAll()
	}()

	for i := 0; i < numberOfSentMessages; i++ {
		server.Emit(ssevents.Event{Data: fmt.Sprintf("Message {%d}", i)})
	}

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	select {
	case <-resultCh:
		t.Error("should not be able to consume all messages")
	case <-timeoutCtx.Done():
		if !errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
			t.Error(timeoutCtx.Err())
		}
	}
}

func Test_givenObserverNoBuffer_whenNotAllEventsArriveAndHasTimeout_thenTimeout(t *testing.T) {
	const numberOfSentMessages = 4

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()

	client, server, shutdown, err := BootstrapClientAndServer(nil)
	if err != nil {
		t.Error(err)
	}
	defer func() {
		shutdownErr := shutdown(shutdownCtx)
		if shutdownErr != nil && !errors.Is(shutdownErr, context.DeadlineExceeded) {
			t.Error(shutdownErr.Error())
		}
	}()

	observer := client.Subscribe(
		ssevents.NewObserverBuilder().
			Buffer(0).
			Limit(5).
			Build(),
	)

	client.Start()

	resultCh := make(chan error)
	go func() {
		_, timeoutErr := observer.WaitForAllOrTimeout(100 * time.Millisecond)
		resultCh <- timeoutErr
	}()

	for i := 0; i < numberOfSentMessages; i++ {
		server.Emit(ssevents.Event{Data: fmt.Sprintf("Message {%d}", i)})
	}

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	select {
	case resultErr := <-resultCh:
		if resultErr == nil {
			t.Error("should return an error that is timeout")
		}
	case <-timeoutCtx.Done():
		t.Error(timeoutCtx.Err())
	}
}

func Test_givenObserverNoBuffer_whenOnEvents_thenReturnSpecifiedEventTypesOnly(t *testing.T) {
	const numberOfSentMessages = 5

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()

	client, server, shutdown, err := BootstrapClientAndServer(nil)
	if err != nil {
		t.Error(err)
	}
	defer func() {
		shutdownErr := shutdown(shutdownCtx)
		if shutdownErr != nil && !errors.Is(shutdownErr, context.DeadlineExceeded) {
			t.Error(shutdownErr.Error())
		}
	}()

	observer := client.Subscribe(
		ssevents.NewObserverBuilder().
			On("Custom").
			Buffer(0).
			Limit(2).
			Build(),
	)

	client.Start()

	type result struct {
		events []ssevents.Event
		err    error
	}

	resultCh := make(chan result)
	go func() {
		events, timeoutErr := observer.WaitForAllOrTimeout(100 * time.Millisecond)
		resultCh <- result{
			events: events,
			err:    timeoutErr,
		}
	}()

	for i := 0; i < numberOfSentMessages; i++ {
		evt := ssevents.Event{Data: fmt.Sprintf("Message {%d}", i)}
		if i > 2 {
			evt.Event = "Custom"
		}
		server.Emit(evt)
	}

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	select {
	case observerResult := <-resultCh:
		if observerResult.err != nil {
			t.Errorf("failed result, got %v", observerResult.err)
		}
		if len(observerResult.events) != 2 {
			t.Errorf("should return 2 events only, received %d", len(observerResult.events))
		}
		for _, event := range observerResult.events {
			if event.Event != "Custom" {
				t.Errorf("expected to receive events with Event being Custom, got %s", event.Event)
			}
		}
	case <-timeoutCtx.Done():
		t.Error(timeoutCtx.Err())
	}
}
