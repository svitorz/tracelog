package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"svitorz/tracelog/internal/core/services"
)

// Valores justificados na spec 0002, seção 4.1.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownTimeout   = 8 * time.Second
)

// ServerConfig groups the settings the HTTP adapter needs.
type ServerConfig struct {
	Addr         string
	MaxBodyBytes int64
}

// Server is the main HTTP server for the MVP.
type Server struct {
	httpServer      *http.Server
	shutdownTimeout time.Duration
}

func NewServer(cfg ServerConfig, registerUseCase *services.RegisterTechnicalEventUseCase) *Server {
	mux := http.NewServeMux()
	eventHandler := NewEventHandler(registerUseCase, cfg.MaxBodyBytes)

	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/events", eventHandler.HandleRegisterEvent)

	return &Server{
		httpServer: &http.Server{
			Addr:              cfg.Addr,
			Handler:           mux,
			ReadHeaderTimeout: readHeaderTimeout,
			ReadTimeout:       readTimeout,
			WriteTimeout:      writeTimeout,
			IdleTimeout:       idleTimeout,
		},
		shutdownTimeout: shutdownTimeout,
	}
}

// Addr returns the configured listen address.
func (s *Server) Addr() string {
	return s.httpServer.Addr
}

// Serve accepts connections on ln until ctx is cancelled. Then it stops accepting new
// connections and waits up to the shutdown timeout for in-flight requests to finish.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- s.httpServer.Serve(ln)
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("HTTP server stopped: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown did not finish within %s: %w", s.shutdownTimeout, err)
	}

	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("HTTP server stopped: %w", err)
	}

	return nil
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
