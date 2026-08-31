package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gorilla/mux"
)

// DockerHandler serves Docker CLI-backed endpoints.
type DockerHandler struct{}

// NewDockerHandler creates a docker handler.
func NewDockerHandler() *DockerHandler {
	return &DockerHandler{}
}

// Container is a parsed docker container.
type Container struct {
	ID     string   `json:"id"`
	Names  []string `json:"names"`
	Image  string   `json:"image"`
	State  string   `json:"state"`
	Status string   `json:"status"`
}

// Containers handles GET /api/docker/containers.
func (h *DockerHandler) Containers(w http.ResponseWriter, r *http.Request) {
	out, _, err := runCommand("docker", "ps", "-a", "--format", "{{json .}}")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "docker command failed: "+err.Error())
		return
	}
	var containers []Container
	for _, line := range splitLines(out) {
		if line == "" {
			continue
		}
		var c Container
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			continue
		}
		containers = append(containers, c)
	}
	writeJSON(w, http.StatusOK, containers)
}

// ContainerAction handles start/stop/restart actions.
func (h *DockerHandler) ContainerAction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	action := vars["action"]

	if id == "" {
		writeError(w, http.StatusBadRequest, "container id required")
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

	out, errb, err := runCommand("docker", verb, id)
	if err != nil {
		msg := errb
		if msg == "" {
			msg = out
		}
		if msg == "" {
			msg = err.Error()
		}
		writeError(w, http.StatusInternalServerError, msg)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": out})
}

// Remove handles DELETE /api/docker/containers/:id. Supports ?force=true.
func (h *DockerHandler) Remove(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		writeError(w, http.StatusBadRequest, "container id required")
		return
	}
	args := []string{"rm"}
	if r.URL.Query().Get("force") == "true" {
		args = append(args, "-f")
	}
	args = append(args, id)

	out, errb, err := runCommand("docker", args...)
	if err != nil {
		msg := errb
		if msg == "" {
			msg = err.Error()
		}
		writeError(w, http.StatusInternalServerError, msg)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed", "message": out})
}

// Logs handles GET /api/docker/containers/:id/logs?lines=100.
func (h *DockerHandler) Logs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		writeError(w, http.StatusBadRequest, "container id required")
		return
	}
	lines := r.URL.Query().Get("lines")
	if lines == "" {
		lines = "100"
	}
	out, errb, err := runCommand("docker", "logs", "--tail", lines, id)
	if err != nil {
		msg := errb
		if msg == "" {
			msg = err.Error()
		}
		writeError(w, http.StatusInternalServerError, msg)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	io.WriteString(w, out)
}

// Image is a parsed docker image.
type Image struct {
	ID       string   `json:"id"`
	RepoTags []string `json:"repo_tags"`
	Size     string   `json:"size"`
}

// Images handles GET /api/docker/images.
func (h *DockerHandler) Images(w http.ResponseWriter, r *http.Request) {
	out, _, err := runCommand("docker", "images", "--format", "{{json .}}")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "docker command failed: "+err.Error())
		return
	}
	var images []Image
	for _, line := range splitLines(out) {
		if line == "" {
			continue
		}
		var img Image
		if err := json.Unmarshal([]byte(line), &img); err != nil {
			continue
		}
		images = append(images, img)
	}
	writeJSON(w, http.StatusOK, images)
}

// Stats handles GET /api/docker/stats (docker stats --no-stream).
func (h *DockerHandler) Stats(w http.ResponseWriter, r *http.Request) {
	format := "{\"id\":\"{{.ID}}\",\"name\":\"{{.Name}}\",\"cpu\":\"{{.CPUPerc}}\",\"mem\":\"{{.MemPerc}}\",\"mem_usage\":\"{{.MemUsage}}\",\"net_io\":\"{{.NetIO}}\",\"block_io\":\"{{.BlockIO}}\",\"pids\":\"{{.PIDs}}\"}"
	out, _, err := runCommand("docker", "stats", "--no-stream", "--format", format)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "docker command failed: "+err.Error())
		return
	}
	var stats []map[string]string
	for _, line := range splitLines(out) {
		if line == "" {
			continue
		}
		var m map[string]string
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue
		}
		stats = append(stats, m)
	}
	writeJSON(w, http.StatusOK, stats)
}

// splitLines splits a string on newlines, trimming carriage returns.
func splitLines(s string) []string {
	out := []string{}
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			out = append(out, line)
			start = i + 1
		}
	}
	return out
}
