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

type SSEHandler func(ctx context.Context, req *http.Request, res chan<- Event)

type HttpController struct {
	log         slog.Logger
	shutdownCtx context.Context
	cancel      context.CancelFunc
	subscribers *sync.Map
	options     *Options
}

func NewController(options *Options) *HttpController {
	ctx, cancel := context.WithCancel(context.Background())

	ctrl := &HttpController{
		shutdownCtx: ctx,
		cancel:      cancel,
		log:         *slog.New(slog.NewTextHandler(os.Stdout, nil)),
		subscribers: &sync.Map{},
		options:     options,
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
		rc := http.NewResponseController(w)

		// On-connect heartbeat
		stringEvent, err := NewSSE("heartbeat", time.Now().String()).ToResponseString()
		if err != nil {
			c.log.Error("failed formatting heartbeat event")
			return
		}
		c.writeAndFlush(rc, w, stringEvent)

		heartbeatTicker := time.NewTicker(c.options.HeartbeatInterval)
		defer heartbeatTicker.Stop()

		data := make(chan Event, 1)
		defer close(data)

		handlerCtx, handlerCleanup := context.WithCancel(c.shutdownCtx)
		defer handlerCleanup()
		go handler(handlerCtx, req, data)

		clientGone := req.Context().Done()
		for {
			select {
			case <-clientGone:
				c.log.Info("Client disconnected")
				return
			case <-c.shutdownCtx.Done():
				c.log.Info("Received shutdown for SSE")
				return
			case <-heartbeatTicker.C:
				stringData, transformErr := NewSSE("heartbeat", time.Now().String()).ToResponseString()
				if transformErr != nil {
					c.log.Error("failed formatting heartbeat event", "err", transformErr)
					return
				}
				c.writeAndFlush(rc, w, stringData)
			case d := <-data:
				stringData, transformErr := d.ToResponseString()
				if transformErr != nil {
					c.log.Error("failed formatting event", "err", transformErr, "event", d)
					return
				}
				c.writeAndFlush(rc, w, stringData)
			}
		}
	}
}

func (c *HttpController) Emit(e Event) {
	c.subscribers.Range(func(_, subChannel any) bool {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		select {
		case subChannel.(chan Event) <- e:
		case <-ctx.Done():
		}
		return true
	})
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
