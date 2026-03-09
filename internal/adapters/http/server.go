package http

import (
	"encoding/json"
	"net/http"

	"svitorz/tracelog/internal/core/services"
)

// Server is the main HTTP server for the MVP.
type Server struct {
	addr         string
	mux          *http.ServeMux
	eventHandler *EventHandler
}

func NewServer(addr string, registerUseCase *services.RegisterTechnicalEventUseCase) *Server {
	mux := http.NewServeMux()
	eventHandler := NewEventHandler(registerUseCase)

	s := &Server{
		addr:         addr,
		mux:          mux,
		eventHandler: eventHandler,
	}

	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/events", eventHandler.HandleRegisterEvent)

	return s
}

func (s *Server) Start() error {
	return http.ListenAndServe(s.addr, s.mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
