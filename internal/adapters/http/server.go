package http

import (
	"encoding/json"
	"net/http"
)

// Server representa o servidor HTTP da aplicação.
type Server struct {
	addr string
	mux  *http.ServeMux
}

func NewServer(addr string) *Server {
	mux := http.NewServeMux()
	s := &Server{addr: addr, mux: mux}

	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/events", s.handleEvents)

	return s
}

func (s *Server) Start() error {
	return http.ListenAndServe(s.addr, s.mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// TODO: validar e persistir evento no PostgreSQL.
	w.WriteHeader(http.StatusAccepted)
}
