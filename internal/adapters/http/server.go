package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strings"
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
	readinessTimeout  = 2 * time.Second
)

// ReadinessCheck reports whether the server's dependencies (the database) can serve requests.
type ReadinessCheck func(ctx context.Context) error

// ServerConfig groups the settings the HTTP adapter needs.
type ServerConfig struct {
	Addr         string
	MaxBodyBytes int64
	// Logger receives internal error details. Nil means slog.Default().
	Logger *slog.Logger
	// Ready backs GET /ready. Nil makes /ready always report unavailable.
	Ready ReadinessCheck
}

// Server is the main HTTP server for the MVP.
type Server struct {
	httpServer      *http.Server
	shutdownTimeout time.Duration
}

func NewServer(cfg ServerConfig, registerUseCase *services.RegisterTechnicalEventUseCase) *Server {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	mux := http.NewServeMux()
	eventHandler := NewEventHandler(registerUseCase, cfg.MaxBodyBytes, logger)

	mux.HandleFunc("/health", allowMethods(handleHealth, http.MethodGet, http.MethodHead))
	mux.HandleFunc("/ready", allowMethods(readyHandler(cfg.Ready, logger), http.MethodGet, http.MethodHead))
	mux.HandleFunc("/events", allowMethods(eventHandler.HandleRegisterEvent, http.MethodPost))

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

// handleHealth is the liveness probe: it answers while the process runs and never
// touches the database, so a slow database does not get a healthy process restarted.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// readyHandler is the readiness probe: it pings the database with a short timeout.
func readyHandler(check ReadinessCheck, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := errors.New("readiness check not configured")
		if check != nil {
			ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
			defer cancel()
			err = check(ctx)
		}

		if err != nil {
			logger.WarnContext(r.Context(), "readiness check failed", "error", err)
			respondJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}

// allowMethods answers 405 with an Allow header, keeping the JSON error format.
func allowMethods(next http.HandlerFunc, methods ...string) http.HandlerFunc {
	allowed := strings.Join(methods, ", ")
	return func(w http.ResponseWriter, r *http.Request) {
		if !slices.Contains(methods, r.Method) {
			w.Header().Set("Allow", allowed)
			respondError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		next(w, r)
	}
}
