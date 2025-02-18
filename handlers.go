package sse_server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

func respondError(w http.ResponseWriter, err error) {
	if err != nil {
		w.WriteHeader(400)
		_, _ = w.Write([]byte("failed: " + err.Error()))
	}
}

func createMux(sseCtrl *HttpController, routes map[string]http.HandlerFunc) *http.ServeMux {
	mux := http.NewServeMux()

	for route, handler := range routes {
		mux.HandleFunc(route, handler)
	}

	if routes["GET /"] == nil {
		mux.HandleFunc("GET /", func(w http.ResponseWriter, req *http.Request) {
			// Catch unmapped requests
			slog.Info(fmt.Sprintf("[Unmapped]: %s - %s", req.Method, req.URL.RawQuery))
		})
	}

	mux.HandleFunc("GET /sse", sseCtrl.Middleware(func(ctx context.Context, req *http.Request, res chan<- Event) {
		subscribeCh := make(chan Event, sseCtrl.options.BufferSize)
		if sseCtrl.HasSubscriber(req.Context()) {
			slog.Warn("existing context subscriber should not exist, overriding it")
		}

		sseCtrl.Store(req.Context(), subscribeCh)
		defer func() {
			slog.Info("Subscriber: cleaning up")
			close(subscribeCh)
			sseCtrl.Delete(req.Context())
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

	mux.HandleFunc("POST /emit", func(w http.ResponseWriter, req *http.Request) {
		// Handle JSON
		if contentType := req.Header.Get("Content-Type"); contentType == "application/json" {
			var event Event
			if err := json.NewDecoder(req.Body).Decode(&event); err != nil {
				respondError(w, err)
				return
			}
			if event.Data == "" {
				respondError(w, errors.New("data should not be empty"))
				return
			}

			sseCtrl.Emit(event)
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

		sseCtrl.Emit(Event{Data: string(data)})
	})

	return mux
}
