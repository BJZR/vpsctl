package handlers

import (
	"encoding/json"
	"net/http"
)

// Health reports process liveness/readiness for orchestrators and health checks.
func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	status := map[string]string{
		"status": "ok",
		"name":   "vpsctl",
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}
