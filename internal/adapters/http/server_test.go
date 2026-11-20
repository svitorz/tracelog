package http

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewServerConfiguresTimeouts(t *testing.T) {
	s := NewServer(ServerConfig{Addr: ":0", MaxBodyBytes: 1024}, nil)

	got := map[string]time.Duration{
		"ReadHeaderTimeout": s.httpServer.ReadHeaderTimeout,
		"ReadTimeout":       s.httpServer.ReadTimeout,
		"WriteTimeout":      s.httpServer.WriteTimeout,
		"IdleTimeout":       s.httpServer.IdleTimeout,
	}
	for name, value := range got {
		if value <= 0 {
			t.Errorf("%s = %s, want a positive timeout", name, value)
		}
	}
}

func TestServeFinishesInFlightRequestOnShutdown(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})

	s := NewServer(ServerConfig{MaxBodyBytes: 1024}, nil)
	s.httpServer.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		_, _ = io.WriteString(w, "done")
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	addr := ln.Addr().String()

	ctx, cancel := context.WithCancel(context.Background())
	serveErr := make(chan error, 1)
	go func() { serveErr <- s.Serve(ctx, ln) }()

	type result struct {
		body string
		err  error
	}
	responses := make(chan result, 1)
	go func() {
		resp, err := http.Get("http://" + addr + "/slow")
		if err != nil {
			responses <- result{err: err}
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		responses <- result{body: string(body), err: err}
	}()

	<-started
	cancel()

	waitUntilRefused(t, addr)
	close(release)

	res := <-responses
	if res.err != nil {
		t.Fatalf("in-flight request error = %v, want it to complete", res.err)
	}
	if res.body != "done" {
		t.Fatalf("in-flight response body = %q, want %q", res.body, "done")
	}

	if err := <-serveErr; err != nil {
		t.Fatalf("Serve() error = %v, want nil after graceful shutdown", err)
	}
}

func TestServeReturnsErrorWhenShutdownTimesOut(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)

	s := NewServer(ServerConfig{MaxBodyBytes: 1024}, nil)
	s.shutdownTimeout = 50 * time.Millisecond
	s.httpServer.Handler = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(started)
		<-release
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	serveErr := make(chan error, 1)
	go func() { serveErr <- s.Serve(ctx, ln) }()
	go func() { _, _ = http.Get("http://" + ln.Addr().String() + "/stuck") }()

	<-started
	cancel()

	err = <-serveErr
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Serve() error = %v, want context.DeadlineExceeded", err)
	}
}

// waitUntilRefused confirms the listener was closed, i.e. shutdown started.
func waitUntilRefused(t *testing.T, addr string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err != nil {
			return
		}
		_ = conn.Close()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("listener at %s still accepting connections after shutdown started", addr)
}

func TestHealthAndReadiness(t *testing.T) {
	dbDown := func(context.Context) error { return errors.New("dial tcp 127.0.0.1:5432: connection refused") }
	dbUp := func(context.Context) error { return nil }

	tests := []struct {
		name       string
		ready      ReadinessCheck
		method     string
		path       string
		wantStatus int
	}{
		{"health ignores the database", dbDown, http.MethodGet, "/health", http.StatusOK},
		{"health accepts HEAD", dbDown, http.MethodHead, "/health", http.StatusOK},
		{"health rejects DELETE", dbUp, http.MethodDelete, "/health", http.StatusMethodNotAllowed},
		{"ready with database up", dbUp, http.MethodGet, "/ready", http.StatusOK},
		{"ready with database down", dbDown, http.MethodGet, "/ready", http.StatusServiceUnavailable},
		{"ready without a check", nil, http.MethodGet, "/ready", http.StatusServiceUnavailable},
		{"ready rejects POST", dbUp, http.MethodPost, "/ready", http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewServer(ServerConfig{MaxBodyBytes: 1024, Ready: tt.ready, Logger: discardLogger()}, nil)
			rec := httptest.NewRecorder()
			s.httpServer.Handler.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))

			if rec.Code != tt.wantStatus {
				t.Fatalf("%s %s = %d, want %d", tt.method, tt.path, rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestReadyHidesDatabaseErrorFromResponse(t *testing.T) {
	s := NewServer(ServerConfig{
		MaxBodyBytes: 1024,
		Logger:       discardLogger(),
		Ready:        func(context.Context) error { return errors.New("password authentication failed") },
	}, nil)

	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))

	if body := rec.Body.String(); body != "{\"status\":\"unavailable\"}\n" {
		t.Fatalf("body = %q, want only the generic status", body)
	}
}

func TestReadyAppliesTimeout(t *testing.T) {
	s := NewServer(ServerConfig{
		MaxBodyBytes: 1024,
		Logger:       discardLogger(),
		Ready: func(ctx context.Context) error {
			if _, ok := ctx.Deadline(); !ok {
				return errors.New("no deadline")
			}
			return nil
		},
	}, nil)

	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: readiness check must receive a context with deadline", rec.Code)
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
