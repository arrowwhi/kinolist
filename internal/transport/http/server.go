package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/arrowwhi/kinolist/internal/config"
)

type Server struct {
	srv *http.Server
}

func NewServer(cfg config.HTTP, h http.Handler) *Server {
	return &Server{
		srv: &http.Server{
			Addr:              cfg.Addr,
			Handler:           h,
			ReadTimeout:       cfg.ReadTimeout,
			ReadHeaderTimeout: cfg.ReadTimeout,
			WriteTimeout:      cfg.WriteTimeout,
		},
	}
}

// Run блокируется до остановки сервера. Штатное закрытие ошибкой не считается.
func (s *Server) Run() error {
	if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}
	return nil
}
