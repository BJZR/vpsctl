package auth

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultAdminPassword = "admin123"
	totpIssuer           = "vpsctl"
)

// ErrAccountLocked is returned when a user exceeded the failed-attempt limit.
var ErrAccountLocked = errors.New("account locked")

// Role represents a user role.
type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

// User represents an authenticated user account.
type User struct {
	Username       string    `json:"username"`
	PasswordHash   string    `json:"password_hash"`
	TOTPSecret     string    `json:"totp_secret,omitempty"`
	TOTPVerified   bool      `json:"totp_verified,omitempty"`
	RequireTOTP    bool      `json:"require_totp"`
	Role           Role      `json:"role"`
	LastLogin      time.Time `json:"last_login"`
	FailedAttempts int       `json:"failed_attempts"`
	LockedUntil    time.Time `json:"locked_until"`
}

// Manager handles user storage and authentication. It is safe for
// concurrent use.
type Manager struct {
	mu     sync.RWMutex
	users  map[string]*User
	path   string
	secret []byte
	max    int
	lock   time.Duration
}

// NewManager creates a user manager backed by a JSON file.
func NewManager(usersFile, dataDir string, secret []byte, maxLoginAttempts int, lockout time.Duration) (*Manager, error) {
	m := &Manager{
		users:  make(map[string]*User),
		path:   usersFile,
		secret: secret,
		max:    maxLoginAttempts,
		lock:   lockout,
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating data dir: %w", err)
	}

	if err := m.load(); err != nil {
		return nil, err
	}

	// Bootstrap default admin when no users exist.
	if len(m.users) == 0 {
		hash, err := bcrypt.GenerateFromPassword([]byte(defaultAdminPassword), 12)
		if err != nil {
			return nil, fmt.Errorf("hashing bootstrap password: %w", err)
		}
		admin := &User{
			Username:     "admin",
			PasswordHash: string(hash),
			Role:         RoleAdmin,
		}
		m.users[admin.Username] = admin
		if err := m.save(); err != nil {
			return nil, err
		}
		fmt.Printf("[vpsctl] Bootstrap admin created: user=%s password=%s\n", admin.Username, defaultAdminPassword)
	}

	return m, nil
}

func (m *Manager) load() error {
	data, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading users file: %w", err)
	}
	if len(data) == 0 {
		return nil
	}
	var list []*User
	if err := json.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("parsing users file: %w", err)
	}
	for _, u := range list {
		m.users[u.Username] = u
	}
	return nil
}

func (m *Manager) save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.saveLocked()
}

// saveLocked writes the users file. The caller must already hold the write
// lock (or m.mu must be held by no one when called from an unlocked context).
func (m *Manager) saveLocked() error {
	list := make([]*User, 0, len(m.users))
	for _, u := range m.users {
		list = append(list, u)
	}

	tmp := m.path + ".tmp"
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling users: %w", err)
	}
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("writing users file: %w", err)
	}
	return os.Rename(tmp, m.path)
}

// Users returns a snapshot of all users.
func (m *Manager) Users() map[string]*User {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]*User, len(m.users))
	for k, v := range m.users {
		cp := *v
		out[k] = &cp
	}
	return out
}

// Get returns a copy of a user by username.
func (m *Manager) Get(username string) (*User, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[username]
	if !ok {
		return nil, false
	}
	cp := *u
	return &cp, true
}

// RegisterUser adds a new user after hashing their password.
func (m *Manager) RegisterUser(username, password string, role Role, requireTOTP bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users[username]; ok {
		return fmt.Errorf("user %q already exists", username)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}
	m.users[username] = &User{
		Username:     username,
		PasswordHash: string(hash),
		Role:         role,
		RequireTOTP:  requireTOTP,
	}
	return m.saveLocked()
}

// Authenticate validates credentials and returns the user on success.
// It tracks failed attempts and enforces lockout.
func (m *Manager) Authenticate(username, password, totpCode string) (*User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	u, ok := m.users[username]
	if !ok {
		// Consume a generic failure to avoid user enumeration timing leaks.
		bcrypt.CompareHashAndPassword([]byte("$2a$10$7EqJtq98hPqEX7fNZaFWoOhi6ES1t0l9aivglrPqJfDqT8q6lUb5a"), []byte(password))
		return nil, errors.New("invalid credentials")
	}

	if time.Now().Before(u.LockedUntil) {
		return nil, fmt.Errorf("%w until %s", ErrAccountLocked, u.LockedUntil.Format(time.RFC3339))
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		u.FailedAttempts++
		if u.FailedAttempts >= m.max {
			u.LockedUntil = time.Now().Add(m.lock)
			u.FailedAttempts = 0
		}
		m.saveLocked()
		return nil, errors.New("invalid credentials")
	}

	if u.RequireTOTP || (u.TOTPVerified && u.TOTPSecret != "") {
		if totpCode == "" {
			return nil, errors.New("TOTP code required")
		}
		if u.TOTPSecret == "" {
			return nil, errors.New("TOTP not configured")
		}
		valid := totp.Validate(totpCode, u.TOTPSecret)
		if !valid {
			u.FailedAttempts++
			if u.FailedAttempts >= m.max {
				u.LockedUntil = time.Now().Add(m.lock)
				u.FailedAttempts = 0
			}
			m.saveLocked()
			return nil, errors.New("invalid TOTP code")
		}
	}

	u.FailedAttempts = 0
	u.LockedUntil = time.Time{}
	u.LastLogin = time.Now()
	m.saveLocked()

	cp := *u
	return &cp, nil
}

// HashPassword hashes a plaintext password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// ValidatePassword checks a plaintext password against a bcrypt hash.
func ValidatePassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// GenerateTOTP creates a new TOTP secret and a QR code provisioning URL.
func GenerateTOTP(username string) (secret string, qrURL string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: username,
	})
	if err != nil {
		return "", "", fmt.Errorf("generating TOTP secret: %w", err)
	}
	return key.Secret(), key.URL(), nil
}

// ValidateTOTP checks a TOTP code against a secret.
func ValidateTOTP(secret, code string) (bool, error) {
	return totp.ValidateCustom(code, secret, time.Now().UTC(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
}

// SetTOTPSecret records a generated secret for a user pending verification.
func (m *Manager) SetTOTPSecret(username, secret string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[username]
	if !ok {
		return errors.New("user not found")
	}
	u.TOTPSecret = secret
	u.TOTPVerified = false
	return m.saveLocked()
}

// VerifyTOTP marks a user's TOTP as verified after a successful code check.
func (m *Manager) VerifyTOTP(username, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[username]
	if !ok {
		return errors.New("user not found")
	}
	if u.TOTPSecret == "" {
		return errors.New("TOTP not configured")
	}
	valid := totp.Validate(code, u.TOTPSecret)
	if !valid {
		return errors.New("invalid TOTP code")
	}
	u.TOTPVerified = true
	u.RequireTOTP = true
	return m.saveLocked()
}

// RegisterTOTPSecret generates and stores a fresh TOTP secret for a user.
func (m *Manager) RegisterTOTPSecret(username string) (string, string, error) {
	secret, qrURL, err := GenerateTOTP(username)
	if err != nil {
		return "", "", err
	}
	if err := m.SetTOTPSecret(username, secret); err != nil {
		return "", "", err
	}
	return secret, qrURL, nil
}

// ChangePassword updates the password for a user after validating the old one.
func (m *Manager) ChangePassword(username, oldPassword, newPassword string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[username]
	if !ok {
		return errors.New("user not found")
	}
	if !ValidatePassword(u.PasswordHash, oldPassword) {
		return errors.New("old password is incorrect")
	}
	if len(newPassword) < 8 {
		return errors.New("new password must be at least 8 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return m.saveLocked()
}

// Claims are the JWT claim fields used by vpsctl.
type Claims struct {
	User string `json:"user"`
	Role Role   `json:"role"`
	jwt.RegisteredClaims
}

// GenerateJWT creates a signed JWT with a 24 hour expiry.
func (m *Manager) GenerateJWT(u *User) (string, time.Time, error) {
	expires := time.Now().Add(24 * time.Hour)
	claims := Claims{
		User: u.Username,
		Role: u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expires),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        randToken(16),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("signing JWT: %w", err)
	}
	return signed, expires, nil
}

// ParseJWT validates a JWT and returns its claims.
func (m *Manager) ParseJWT(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// RefreshToken is a generated refresh token with its expiry and owner.
type RefreshToken struct {
	Token     string    `json:"token"`
	Username  string    `json:"username"`
	ExpiresAt time.Time `json:"expires_at"`
}

// GenerateRefresh produces a new random refresh token (14 day expiry).
func GenerateRefresh(username string) (*RefreshToken, error) {
	return &RefreshToken{
		Token:     randToken(32),
		Username:  username,
		ExpiresAt: time.Now().Add(14 * 24 * time.Hour),
	}, nil
}

func randToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
}

// EnsureBootstrap returns whether the admin account exists, creating it if not.
// This is exposed for the router setup path.
func (m *Manager) EnsureBootstrap() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users["admin"]; ok {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(defaultAdminPassword), 12)
	if err != nil {
		return err
	}
	m.users["admin"] = &User{
		Username:     "admin",
		PasswordHash: string(hash),
		Role:         RoleAdmin,
	}
	if err := m.saveLocked(); err != nil {
		return err
	}
	fmt.Printf("[vpsctl] Bootstrap admin created: user=admin password=%s\n", defaultAdminPassword)
	return nil
}

// DataDirDefault is a helper for constructing the users file path.
func DataDirDefault() string {
	dir, err := os.Getwd()
	if err != nil {
		return "./data"
	}
	return filepath.Join(dir, "data")
}

// IsLocked reports whether err is an account-lock error.
func IsLocked(err error) bool {
	return err != nil && errors.Is(err, ErrAccountLocked)
}

// GenerateCode computes a TOTP code for secret at time t (used in tests and
// CLI tooling; the browser flow uses the QR setup instead).
func GenerateCode(secret string, t time.Time) (string, error) {
	return totp.GenerateCode(secret, t)
}
