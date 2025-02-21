# SSE server and client

Example implementation of **Server Sent Events (SSE)** server and client in **Go**. Useful for usage in tests where you
want to spin up a real SSE server to emit events to.

![Tests](https://github.com/doppelganger113/ssevents/actions/workflows/check.yaml/badge.svg)
![Lint](https://github.com/doppelganger113/ssevents/actions/workflows/golangci-lint.yaml/badge.svg)

## Quick start

This library has 2 example implementation of a server and a client, server will have a webpage that will append messages
as it receives them and the client will output them to a console.

1. Run server (accessed on `localhost:3000`)
    ```bash
    make run
    ```
2. Run client
    ```bash
    make run-client
    ```
3. Emit an event (sends a curl request)
    ```bash
    make emit
    #    Sending hello to the server API...
    #    curl -X POST -H "Content-Type: application/json" -d '{"data": "{\"message\": \"Hello\"}"}' localhost:3000/emit
    ```

> Note: you can pass args like so: `make run ARGS="--log-level debug"`

And on the client for example you will see the received event:
```bash
time=2025-02-19T14:39:46.364+01:00 level=INFO msg="received an event" event="data: {\"message\": \"Hello\"}"
```

You can see both [server example](examples/server/main.go) and [client example](examples/client/main.go) on how to
run both in Go programmatically.

Options for configuring the server are:
```go
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
```

Table of contents
=================

<!--ts-->
* [Event structure](#event-structure)
* [Test usage](#test-usage)
* [FAQ](#faq)
<!--te-->

## Event structure

The **event** object (that was sent above) has the following structure where _all fields are optional_ except the 
**data** field
```json
{
  "id": "123ab",
  "event": "priority",
  "data": "123",
  "retry": 10
}
```
but don't forget that this json is converted to a string when being assigned to field **data**.

## Test usage

A utility function that you can use in tests to easily start and server and client that are connected is through the
use of `BootstrapClientAndServer` and the `Observers` that give more control when subscribing to events.

```go
client, server, shutdown, err := tests.BootstrapClientAndServer(nil)
if err != nil {
    t.Error(err)
}
defer func() {
    if shutdownErr := shutdown(ctx); shutdownErr != nil {
        t.Error(shutdownErr)
    }
}()

client.Start()
```
Once the `client.Start()` is invoked, it will connect and start consuming messages emitted from the server.

```go
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

	client, server, shutdown, err := tests.BootstrapClientAndServer(nil)
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
```


## FAQ

- **Safari users** might experience basic html output via stream to not show properly due to internal buffering that is done
so in those cases please use another browser.