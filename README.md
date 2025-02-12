# SSE server and client

Example of implementing a **Server Sent Events (SSE)** server and client in Go. You can send messages to the server
that will get omitted to the connected clients where the clients are either a Go app or the browser.

## Getting started

**Note!**
Safari users might experience basic html output via stream to not show properly due to internal buffering that is done
so in those cases please use another browser.

**Requirements:**
- Go

When running the applications, especially the server, you will get debug insights that will help you see what's 
happening.

Run server
```bash
go run cmd/server/main.go
```

Run client
```bash
go run cmd/client/main.go
```
Or open browser on `localhost:3000`

## Sending events

There is a timer functionality endpoint on `localhost:3000/sse/timer` that just emits a message every 1 second to that
webpage.

For sending events to all connected subscribers you can either execute in your browser the following endpoint:
`localhost:3000/emit?data=123` or execute a curl request:
```bash
curl "localhost:3000/emit/?data=123"
```
You can also use the POST version of that endpoint to send 

- Plain text
    ```bash
    curl -d 'plain text' -X POST localhost:3000/emit
    ```
- JSON of the whole Event object
    ```bash
    curl -H 'Content-Type: application/json' \
      -d '{ "id":"foo","data":"{\"name\":\"John\", \"age\": 20}", "event": "priority"}' \
      -X POST \
      localhost:3000/emit
    ```
The **event** object has the following structure where all fields are optional except the data field
```json
{
  "id": "123ab",
  "event": "priority",
  "data": "123",
  "retry": 10
}
```

## Custom SSE handler

You can utilize the `sse.HttpController` `Middleware` function that wraps the handling of establishing and closing
the stream properly. Only important thing is that you need to watch for the context closing so that everything is 
shut down properly.

Example implementation of the timer using the wrapper
```go
mux.HandleFunc("GET /sse/timer", sseCtrl.Middleware(func(ctx context.Context, req *http.Request, res chan<- sse.Event) {
    t := time.NewTicker(time.Second)
    defer t.Stop()

    for {
        select {
        case <-t.C:
            res <- sse.Event{Data: "Hello!"}
        case <-ctx.Done():
            return
        }
    }
}))
```
Don't forget to clean up the resources in the handler like the _ticker_ above, channel closing will happen in the 
wrapper, so don't worry about that.