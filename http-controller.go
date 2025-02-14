package sse_server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"
)

const heartbeatIntervalDefault = 20 * time.Second

type SSEHandler func(ctx context.Context, req *http.Request, res chan<- Event)

type HttpController struct {
	log               slog.Logger
	shutdownCtx       context.Context
	cancel            context.CancelFunc
	heartbeatInterval time.Duration
	subscribers       *sync.Map
}

type ControllerOptions struct {
	// HeartbeatInterval represented in millis or default [heartbeatIntervalDefault]
	HeartbeatInterval *time.Duration
	Logger            *slog.Logger
}

func NewController(options *ControllerOptions) *HttpController {
	ctx, cancel := context.WithCancel(context.Background())

	ctrl := &HttpController{
		shutdownCtx: ctx,
		cancel:      cancel,
		log:         *slog.New(slog.NewTextHandler(os.Stdout, nil)),
		subscribers: &sync.Map{},
	}

	if options != nil {
		if options.HeartbeatInterval != nil && *options.HeartbeatInterval > 0 {
			ctrl.heartbeatInterval = *options.HeartbeatInterval
		}
		if options.Logger != nil {
			ctrl.log = *options.Logger
		}
	}
	if ctrl.heartbeatInterval <= 0 {
		ctrl.heartbeatInterval = heartbeatIntervalDefault
	}
	return ctrl
}

func (c *HttpController) Shutdown() error {
	c.cancel()
	return nil
}

func (c *HttpController) writeAndFlush(rc *http.ResponseController, w http.ResponseWriter, data string) {
	_, err := fmt.Fprint(w, data)
	if err != nil {
		c.log.Error("sending data to client on SSE failed", "err", err)
		return
	}

	err = rc.Flush()
	if err != nil {
		c.log.Error("failed flushing the SSE", "err", err)
		return
	}
}

// Middleware - creates a wrapper for sending SSE to the client with proper cancellation, heartbeat
// and cleanup functionality already implemented.
//
// The main ctx is used for graceful shutdown of all connected subscribers to SSE and is used when
// shutting down the server.
//
// handler function will be executed in a separate goroutine so you just need to send data to it and ensure
// that you clean up everything in it. Take for an example the following handler function that receives
// data from a channel then forwards it as a response (note that subscribers is a global channel map).
//
//		 subscribers[req.Context()] = subscribeCh
//		 defer func() {
//		   close(subscribeCh)
//		   delete(subscribers, req.Context())
//		   slog.Info("cleaning up subscribers")
//		 }()
//
//	 for {
//	   select {
//	     case <-handlerCtx.Done():
//	       return
//	     case data := <-subscribeCh:
//	       select {
//	         case res <- data:
//	         case <-handlerCtx.Done():
//	           return
//	       }
//	       slog.Info("Sent data to client")
//	   }
//	 }
func (c *HttpController) Middleware(handler SSEHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")             // Adjust if needed
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS") // not needed

		// You may need this locally for CORS requests
		w.Header().Set("Access-Control-Allow-Origin", "*")

		c.log.Info("Client connected")

		clientGone := req.Context().Done()
		rc := http.NewResponseController(w)

		heartbeatTicker := time.NewTicker(c.heartbeatInterval)
		defer heartbeatTicker.Stop()

		data := make(chan Event)
		defer close(data)

		handlerCtx, handlerCleanup := context.WithCancel(c.shutdownCtx)
		defer handlerCleanup()
		go handler(handlerCtx, req, data)

		for {
			select {
			case <-clientGone:
				c.log.Info("Client disconnected")
				return
			case <-c.shutdownCtx.Done():
				c.log.Info("Received shutdown for SSE")
				return
			case <-heartbeatTicker.C:
				stringEvent, err := NewSSE("heartbeat", time.Now().String()).ToResponseString()
				if err != nil {
					c.log.Error("failed formatting heartbeat event")
					return
				}
				c.writeAndFlush(rc, w, stringEvent)
			case d := <-data:
				stringEvent, err := d.ToResponseString()
				if err != nil {
					c.log.Error("failed formatting event", "event", d)
					return
				}
				c.writeAndFlush(rc, w, stringEvent)
			}
		}
	}
}

func (c *HttpController) Emit(e Event) {
	// TODO: think about avoiding blocking when consumer is slow
	var counter int
	c.subscribers.Range(func(_, subChannel any) bool {
		counter++
		subChannel.(chan Event) <- e
		return true
	})
	slog.Info(fmt.Sprintf("Emitted: '%+v' to %d subscribers", e, counter))
}

func (c *HttpController) HasSubscriber(key any) bool {
	_, ok := c.subscribers.Load(key)
	return ok
}

func (c *HttpController) Store(key any, subCh chan Event) {
	c.subscribers.Store(key, subCh)
}

func (c *HttpController) Delete(key any) {
	c.subscribers.Delete(key)
}
