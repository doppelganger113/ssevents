package server

import (
	"context"
	"errors"
	"github.com/doppelganger113/sse-server/sse"
	"net/http"
	"strconv"
)

type Options struct {
	Port int
}

type Server struct {
	httpServer *http.Server
	sseCtrl    *sse.HttpController
}

func (s *Server) ListenAndServe() error {
	if err := s.httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return errors.Join(
		s.sseCtrl.Shutdown(),
		s.httpServer.Shutdown(ctx),
	)
}

func New(options Options) (*Server, error) {
	sseCtrl := sse.NewController(nil)
	httpServer := &http.Server{Addr: ":" + strconv.Itoa(options.Port), Handler: createMux(sseCtrl)}

	return &Server{httpServer: httpServer, sseCtrl: sseCtrl}, nil
}
