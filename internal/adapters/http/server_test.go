package http

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
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
