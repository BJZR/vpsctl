package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration.
type Config struct {
	Port             int           `yaml:"port"`
	Host             string        `yaml:"host"`
	JWTSecret        []byte        `yaml:"jwt_secret"`
	TOTPEnabled      bool          `yaml:"totp_enabled"`
	AllowedIPs       []string      `yaml:"allowed_ips"`
	MaxLoginAttempts int           `yaml:"max_login_attempts"`
	LockoutDuration  time.Duration `yaml:"lockout_duration"`
	TLSCertFile      string        `yaml:"tls_cert_file"`
	TLSKeyFile       string        `yaml:"tls_key_file"`
	DataDir          string        `yaml:"data_dir"`
	BaseDir          string        `yaml:"base_dir"`
}

// Default returns a Config populated with sensible defaults.
func Default() *Config {
	return &Config{
		Port:             8443,
		Host:             "0.0.0.0",
		JWTSecret:        []byte("vpsctl-default-secret-change-me"),
		TOTPEnabled:      false,
		AllowedIPs:       nil,
		MaxLoginAttempts: 5,
		LockoutDuration:  15 * time.Minute,
		TLSCertFile:      "",
		TLSKeyFile:       "",
		DataDir:          "./data",
		BaseDir:          "/",
	}
}

// Load reads configuration from a YAML file if present, then overlays
// environment variables (VPSCTL_ prefix) which take precedence.
func Load(path string) (*Config, error) {
	cfg := Default()

	if path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("parsing config file %s: %w", path, err)
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading config file %s: %w", path, err)
		}
	}

	if err := cfg.applyEnv(); err != nil {
		return nil, err
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) applyEnv() error {
	if v := os.Getenv("VPSCTL_PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("VPSCTL_PORT must be an integer: %w", err)
		}
		c.Port = p
	}
	if v := os.Getenv("VPSCTL_HOST"); v != "" {
		c.Host = v
	}
	if v := os.Getenv("VPSCTL_JWT_SECRET"); v != "" {
		c.JWTSecret = []byte(v)
	}
	if v := os.Getenv("VPSCTL_TOTP_ENABLED"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("VPSCTL_TOTP_ENABLED must be a boolean: %w", err)
		}
		c.TOTPEnabled = b
	}
	if v := os.Getenv("VPSCTL_ALLOWED_IPS"); v != "" {
		c.AllowedIPs = splitCSV(v)
	}
	if v := os.Getenv("VPSCTL_MAX_LOGIN_ATTEMPTS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("VPSCTL_MAX_LOGIN_ATTEMPTS must be an integer: %w", err)
		}
		c.MaxLoginAttempts = n
	}
	if v := os.Getenv("VPSCTL_LOCKOUT_DURATION"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("VPSCTL_LOCKOUT_DURATION must be a duration: %w", err)
		}
		c.LockoutDuration = d
	}
	if v := os.Getenv("VPSCTL_TLS_CERT_FILE"); v != "" {
		c.TLSCertFile = v
	}
	if v := os.Getenv("VPSCTL_TLS_KEY_FILE"); v != "" {
		c.TLSKeyFile = v
	}
	if v := os.Getenv("VPSCTL_DATA_DIR"); v != "" {
		c.DataDir = v
	}
	if v := os.Getenv("VPSCTL_BASE_DIR"); v != "" {
		c.BaseDir = v
	}
	return nil
}

func splitCSV(v string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(v); i++ {
		if i == len(v) || v[i] == ',' {
			s := v[start:i]
			if s != "" {
				out = append(out, s)
			}
			start = i + 1
		}
	}
	return out
}

func (c *Config) validate() error {
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port %d", c.Port)
	}
	if len(c.JWTSecret) < 16 {
		return fmt.Errorf("JWT secret must be at least 16 bytes")
	}
	if c.MaxLoginAttempts <= 0 {
		return fmt.Errorf("max_login_attempts must be positive")
	}
	if c.LockoutDuration <= 0 {
		return fmt.Errorf("lockout_duration must be positive")
	}
	if c.DataDir == "" {
		return fmt.Errorf("data_dir cannot be empty")
	}
	return nil
}

// EnsureDataDir creates the data directory if it does not exist.
func (c *Config) EnsureDataDir() error {
	return os.MkdirAll(c.DataDir, 0o755)
}

// UsersFile returns the path to the users.json file.
func (c *Config) UsersFile() string {
	return filepath.Join(c.DataDir, "users.json")
}

// Addr returns the host:port listen address.
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
