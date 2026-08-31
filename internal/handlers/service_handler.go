package handlers

import (
	"io"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

// ServiceHandler serves systemd service endpoints via systemctl.
type ServiceHandler struct{}

// NewServiceHandler creates a service handler.
func NewServiceHandler() *ServiceHandler {
	return &ServiceHandler{}
}

// Service describes a systemd unit.
type Service struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Load        string `json:"load"`
	Active      string `json:"active"`
	Sub         string `json:"sub"`
	Status      string `json:"status"`
}

// List handles GET /api/services.
func (h *ServiceHandler) List(w http.ResponseWriter, r *http.Request) {
	out, _, err := runCommand("systemctl", "list-units", "--type=service", "--all", "--no-pager", "--no-legend", "--plain")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "systemctl command failed: "+err.Error())
		return
	}

	var services []Service
	for _, line := range splitLines(out) {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		s := Service{
			Name:   strings.TrimSuffix(fields[0], ".service"),
			Load:   fields[1],
			Active: fields[2],
			Sub:    fields[3],
			Status: fields[2],
		}
		services = append(services, s)
	}
	writeJSON(w, http.StatusOK, services)
}

// Get handles GET /api/services/:name.
func (h *ServiceHandler) Get(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := sanitizeUnit(vars["name"])
	if name == "" {
		writeError(w, http.StatusBadRequest, "service name required")
		return
	}

	status, _, _ := runCommand("systemctl", "show", name, "--no-pager", "--property=Names,LoadState,ActiveState,SubState,Description")
	lines := parseShow(status)

	logs, _, _ := runCommand("journalctl", "-u", name, "-n", "20", "--no-pager")

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":        name,
		"show":        lines,
		"recent_logs": logs,
	})
}

// Action handles start/stop/restart for services.
func (h *ServiceHandler) Action(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := sanitizeUnit(vars["name"])
	action := vars["action"]
	if name == "" {
		writeError(w, http.StatusBadRequest, "service name required")
		return
	}

	mapping := map[string]string{
		"start":   "start",
		"stop":    "stop",
		"restart": "restart",
	}
	verb, ok := mapping[action]
	if !ok {
		writeError(w, http.StatusBadRequest, "unsupported action: "+action)
		return
	}

	out, errb, err := runCommand("systemctl", verb, name)
	if err != nil {
		msg := errb
		if msg == "" {
			msg = out
		}
		if msg == "" {
			msg = err.Error()
		}
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(msg))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "result": strings.TrimSpace(out)})
}

// Logs handles GET /api/services/:name/logs?lines=50.
func (h *ServiceHandler) Logs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := sanitizeUnit(vars["name"])
	if name == "" {
		writeError(w, http.StatusBadRequest, "service name required")
		return
	}
	lines := r.URL.Query().Get("lines")
	if lines == "" {
		lines = "50"
	}
	out, errb, err := runCommand("journalctl", "-u", name, "-n", lines, "--no-pager")
	if err != nil {
		msg := errb
		if msg == "" {
			msg = err.Error()
		}
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(msg))
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	io.WriteString(w, out)
}

// sanitizeUnit removes path separators and ensures a .service suffix is not
// duplicative. It prevents passing arbitrary arguments to systemctl.
func sanitizeUnit(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimPrefix(name, "\"")
	name = strings.TrimSuffix(name, "\"")
	if name == "" || strings.ContainsAny(name, "/\\") {
		return ""
	}
	return name
}

// parseShow converts `systemctl show --property=` output into a map.
func parseShow(output string) map[string]string {
	result := map[string]string{}
	for _, line := range splitLines(output) {
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := line[:idx]
		value := line[idx+1:]
		result[key] = value
	}
	return result
}
