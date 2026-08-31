package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("VPSCTL_DATA_DIR", t.TempDir())
	t.Setenv("VPSCTL_PORT", "")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Port != 8443 {
		t.Errorf("default port = %d, want 8443", cfg.Port)
	}
	if cfg.MaxLoginAttempts != 5 {
		t.Errorf("default max_login_attempts = %d, want 5", cfg.MaxLoginAttempts)
	}
	if cfg.LockoutDuration != 15*time.Minute {
		t.Errorf("default lockout = %v, want 15m", cfg.LockoutDuration)
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("VPSCTL_DATA_DIR", t.TempDir())
	t.Setenv("VPSCTL_PORT", "9090")
	t.Setenv("VPSCTL_MAX_LOGIN_ATTEMPTS", "3")
	t.Setenv("VPSCTL_JWT_SECRET", "super-secret-env")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Port != 9090 {
		t.Errorf("env port = %d, want 9090", cfg.Port)
	}
	if cfg.MaxLoginAttempts != 3 {
		t.Errorf("env max attempts = %d, want 3", cfg.MaxLoginAttempts)
	}
	if string(cfg.JWTSecret) != "super-secret-env" {
		t.Error("env jwt secret not applied")
	}
	if cfg.Addr() != "0.0.0.0:9090" {
		t.Errorf("Addr() = %q, want 0.0.0.0:9090", cfg.Addr())
	}
}

func TestEnsureDataDir(t *testing.T) {
	dir := t.TempDir() + "/nested/data"
	t.Setenv("VPSCTL_DATA_DIR", dir)
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if err := cfg.EnsureDataDir(); err != nil {
		t.Fatalf("EnsureDataDir failed: %v", err)
	}
	if _, err := os.Stat(cfg.DataDir); err != nil {
		t.Errorf("data dir was not created: %v", err)
	}
}
