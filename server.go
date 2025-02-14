package sse_server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Options struct {
	Port              int
	Handlers          map[string]http.HandlerFunc
	HeartbeatInterval *time.Duration
}

type Server struct {
	url        string
	httpServer *http.Server
	sseCtrl    *HttpController
}

func New(options Options) (*Server, error) {
	sseCtrl := NewController(&ControllerOptions{HeartbeatInterval: options.HeartbeatInterval})
	httpServer := &http.Server{
		Addr:    ":" + strconv.Itoa(options.Port),
		Handler: createMux(sseCtrl, options.Handlers),
	}

	return &Server{httpServer: httpServer, sseCtrl: sseCtrl}, nil
}

func (s *Server) ListenAndServe() error {
	if err := s.httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
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
			slog.Info("Closing random port server")
			close(errCh)
			listener.Close()
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

func (s *Server) Emit(e Event) {
	s.sseCtrl.Emit(e)
}
