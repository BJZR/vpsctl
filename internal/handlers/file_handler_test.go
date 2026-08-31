package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileResolveBlocksTraversal(t *testing.T) {
	h := NewFileHandler("/tmp/vpsctl-base")
	if err := os.MkdirAll(h.BaseDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	cases := []string{
		"../etc/shadow",
		"/tmp/../../etc/shadow",
		"..",
		"foo/../../bar",
		`..\windows\system32`,
		"a/../b/../../etc/passwd",
	}
	for _, c := range cases {
		if _, err := h.resolve(c); err == nil {
			t.Errorf("resolve(%q) should have been rejected", c)
		} else if !strings.Contains(err.Error(), "not allowed") {
			t.Errorf("resolve(%q) rejected for wrong reason: %v", c, err)
		}
	}
}

func TestFileResolveAllowsInBase(t *testing.T) {
	h := NewFileHandler("/tmp/vpsctl-base")
	if err := os.MkdirAll(filepath.Join(h.BaseDir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	p, err := h.resolve("/sub/file.txt")
	if err != nil {
		t.Fatalf("resolve should allow paths inside base: %v", err)
	}
	if !strings.HasPrefix(p, h.BaseDir) {
		t.Errorf("resolved path %q escaped base %q", p, h.BaseDir)
	}

	p, err = h.resolve("sub/file.txt")
	if err != nil {
		t.Fatalf("resolve should allow relative path inside base: %v", err)
	}
	if !strings.HasPrefix(p, h.BaseDir) {
		t.Errorf("resolved path %q escaped base %q", p, h.BaseDir)
	}
}
