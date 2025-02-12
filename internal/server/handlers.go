package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/doppelganger113/sse-server/internal/fe"
	"github.com/doppelganger113/sse-server/sse"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

var subscribers = sync.Map{}

func emitToSubscribers(evt sse.Event) {
	var counter int
	subscribers.Range(func(_, subChannel any) bool {
		counter++
		subChannel.(chan sse.Event) <- evt
		return true
	})
	slog.Info(fmt.Sprintf("Emitted: '%+v' to %d subscribers", evt, counter))
}

func respondError(w http.ResponseWriter, err error) {
	if err != nil {
		w.WriteHeader(400)
		_, _ = w.Write([]byte("failed: " + err.Error()))
	}
}

func createMux(sseCtrl *sse.HttpController) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, req *http.Request) {
		// Only main path (home)
		if req.URL.String() == "/" {
			_, err := w.Write(fe.IndexFile)
			if err != nil {
				slog.Error("failed writing response", "err", err)
			}
			return
		}

		// Catch unmapped requests
		slog.Info(fmt.Sprintf("[Unmapped]: %s - %s", req.Method, req.URL.RawQuery))
	})

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

	mux.HandleFunc("GET /sse", sseCtrl.Middleware(func(ctx context.Context, req *http.Request, res chan<- sse.Event) {
		subscribeCh := make(chan sse.Event, 1)
		if _, ok := subscribers.Load(req.Context()); ok {
			slog.Warn("existing context subscriber should not exist, overriding it")
		}
		subscribers.Store(req.Context(), subscribeCh)
		defer func() {
			slog.Info("Subscriber: cleaning up")
			close(subscribeCh)
			subscribers.Delete(req.Context())
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case data := <-subscribeCh:
				select {
				case res <- data:
				case <-ctx.Done():
					return
				}
			}
		}
	}))

	mux.HandleFunc("GET /emit", func(w http.ResponseWriter, req *http.Request) {
		data := req.URL.Query().Get("data")
		emitToSubscribers(sse.Event{Data: data})
	})

	mux.HandleFunc("POST /emit", func(w http.ResponseWriter, req *http.Request) {
		// Handle JSON
		if contentType := req.Header.Get("Content-Type"); contentType == "application/json" {
			var event sse.Event
			if err := json.NewDecoder(req.Body).Decode(&event); err != nil {
				respondError(w, err)
				return
			}
			if event.Data == "" {
				respondError(w, errors.New("data should not be empty"))
				return
			}

			emitToSubscribers(event)
			return
		}

		// Handle text
		data, err := io.ReadAll(req.Body)
		if err != nil {
			respondError(w, err)
			return
		}
		if string(data) == "" {
			respondError(w, errors.New("data should not be empty"))
			return
		}

		emitToSubscribers(sse.Event{Data: string(data)})
	})

	return mux
}
