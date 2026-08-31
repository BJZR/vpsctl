package handlers

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"
)

// writeJSON writes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// parseJSON decodes the request body into v.
func parseJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// runCommand executes a command and captures stdout/stderr.
func runCommand(name string, args ...string) (string, string, error) {
	cmd := exec.Command(name, args...)
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	return out.String(), errb.String(), err
}
