package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// FileHandler serves file manager endpoints.
type FileHandler struct {
	BaseDir string
}

// NewFileHandler creates a file handler rooted at baseDir.
func NewFileHandler(baseDir string) *FileHandler {
	if baseDir == "" {
		baseDir = "/"
	}
	return &FileHandler{BaseDir: filepath.Clean(baseDir)}
}

// resolve safely joins a requested path under the base dir, preventing
// traversal outside it.
func (h *FileHandler) resolve(path string) (string, error) {
	if path == "" {
		path = "/"
	}
	// Reject explicit traversal segments outright instead of silently
	// clamping them, so the panel never resolves paths outside base.
	for _, seg := range strings.Split(strings.ReplaceAll(path, "\\", "/"), "/") {
		if seg == ".." {
			return "", fmt.Errorf("path traversal is not allowed")
		}
	}
	cleanBase := filepath.Clean(h.BaseDir)
	p := filepath.Join(cleanBase, path)
	rel, err := filepath.Rel(cleanBase, p)
	if err != nil {
		return "", fmt.Errorf("invalid path")
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes base directory")
	}
	// Prevent symlink escapes.
	real, err := filepath.EvalSymlinks(p)
	if err == nil {
		relReal, err2 := filepath.Rel(cleanBase, real)
		if err2 == nil && (relReal == ".." || strings.HasPrefix(relReal, ".."+string(filepath.Separator))) {
			return "", fmt.Errorf("path escapes base directory")
		}
	}
	return p, nil
}

// fileEntry is a single directory entry.
type fileEntry struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime string `json:"mod_time"`
	IsDir   bool   `json:"is_dir"`
	Path    string `json:"path"`
}

// List handles GET /api/files/list?path=.
func (h *FileHandler) List(w http.ResponseWriter, r *http.Request) {
	requested := r.URL.Query().Get("path")
	dir, err := h.resolve(requested)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "path not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !info.IsDir() {
		writeError(w, http.StatusBadRequest, "path is not a directory")
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result := make([]fileEntry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		full := filepath.Join(dir, e.Name())
		relPath, _ := filepath.Rel(h.BaseDir, full)
		result = append(result, fileEntry{
			Name:    e.Name(),
			Size:    info.Size(),
			Mode:    info.Mode().String(),
			ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
			IsDir:   e.IsDir(),
			Path:    "/" + relPath,
		})
	}
	writeJSON(w, http.StatusOK, result)
}

const maxReadSize = 1 << 20 // 1MB

// Read handles GET /api/files/read?path=.
func (h *FileHandler) Read(w http.ResponseWriter, r *http.Request) {
	requested := r.URL.Query().Get("path")
	p, err := h.resolve(requested)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	info, err := os.Stat(p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if info.IsDir() {
		writeError(w, http.StatusBadRequest, "path is a directory")
		return
	}
	f, err := os.Open(p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxReadSize+1))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path":      p,
		"content":   string(data),
		"truncated": len(data) > maxReadSize,
	})
}

type writeRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// Write handles POST /api/files/write.
func (h *FileHandler) Write(w http.ResponseWriter, r *http.Request) {
	var req writeRequest
	if err := parseJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	p, err := h.resolve(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := os.WriteFile(p, []byte(req.Content), 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "written", "path": req.Path})
}

// Upload handles POST /api/files/upload (multipart form: file + path).
func (h *FileHandler) Upload(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)
	dest := r.FormValue("path")
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	target := filepath.Join(dest, header.Filename)
	p, err := h.resolve(target)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	out, err := os.Create(p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "uploaded", "path": target})
}

// Download handles GET /api/files/download?path=.
func (h *FileHandler) Download(w http.ResponseWriter, r *http.Request) {
	requested := r.URL.Query().Get("path")
	p, err := h.resolve(requested)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(p)+"\"")
	http.ServeFile(w, r, p)
}

// DeleteEntry handles DELETE /api/files/delete (body: {path}).
func (h *FileHandler) DeleteEntry(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := parseJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	p, err := h.resolve(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := os.RemoveAll(p); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "path": req.Path})
}

type mkdirRequest struct {
	Path string `json:"path"`
}

// Mkdir handles POST /api/files/mkdir.
func (h *FileHandler) Mkdir(w http.ResponseWriter, r *http.Request) {
	var req mkdirRequest
	if err := parseJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	p, err := h.resolve(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := os.MkdirAll(p, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "created", "path": req.Path})
}

type renameRequest struct {
	OldPath string `json:"old_path"`
	NewPath string `json:"new_path"`
}

// Rename handles POST /api/files/rename.
func (h *FileHandler) Rename(w http.ResponseWriter, r *http.Request) {
	var req renameRequest
	if err := parseJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	oldP, err := h.resolve(req.OldPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	newP, err := h.resolve(req.NewPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := os.Rename(oldP, newP); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "renamed", "old_path": req.OldPath, "new_path": req.NewPath})
}
