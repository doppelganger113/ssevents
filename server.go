package ssevents

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
)

type Server struct {
	httpServer *http.Server
	sseCtrl    *HttpController
	logger     *slog.Logger
}

func NewServer(options *Options) (*Server, error) {
	updatedOptions := newUpdatedOptions(options)

	sseCtrl := NewController(updatedOptions)
	httpServer := &http.Server{
		Addr:    ":" + strconv.Itoa(updatedOptions.Port),
		Handler: createMux(sseCtrl, options, updatedOptions.Handlers),
	}

	return &Server{
		httpServer: httpServer,
		sseCtrl:    sseCtrl,
		logger:     options.Logger,
	}, nil
}

// ListenAndServe starts serving HTTP requests and returns an error on unknown failure. Returns nil error when server
// is closed or shut down.
func (s *Server) ListenAndServe() error {
	if err := s.httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

// ListenAndServeOnRandomPort starts a server on a random available port, but does not block so you can use
// the url address of the server for connecting your client to. The returned channel is used when the server closes.
func (s *Server) ListenAndServeOnRandomPort() (string, chan error, error) {
	errCh := make(chan error)

	listener, err := net.Listen("tcp", ":0") // ":0" picks a random available port
	if err != nil {
		return "", nil, fmt.Errorf("failed listening on random tcp port: %w", err)
	}

	// Get the actual bound address
	addr := listener.Addr().String()

	go func() {
		defer func() {
			close(errCh)
			if closerErr := listener.Close(); closerErr != nil {
				s.logger.Error("failed closing listener", "err", errCh)
			}
		}()
		if err = s.httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	return normalizeAddress(addr), errCh, nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return errors.Join(
		s.sseCtrl.Shutdown(),
		s.httpServer.Shutdown(ctx),
	)
}

// Emit sends an event to all TCP connections listening on the sse endpoint
func (s *Server) Emit(e Event) {
	s.sseCtrl.Emit(e)
}

// normalizeAddress converts a net.Listener address into a client-accessible URL
func normalizeAddress(addr string) string {
	// Check if the address is in the format [::]:port
	if strings.HasPrefix(addr, "[::]:") {
		// Replace [::] with localhost (IPv4 and IPv6 compatible)
		return "http://localhost" + addr[4:]
	} else if strings.HasPrefix(addr, "0.0.0.0:") {
		// Replace 0.0.0.0 with localhost
		return "http://localhost" + addr[7:]
	}
	// Assume it's already a valid hostname/IP
	return "http://" + addr
}
