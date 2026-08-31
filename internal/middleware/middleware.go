package middleware

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ctxKey is a private type for request context keys.
type ctxKey string

// Context keys for authenticated identity.
const (
	UserKey ctxKey = "vpsctl.user"
	RoleKey ctxKey = "vpsctl.role"
)

// UserFromContext returns the authenticated username from a request context.
func UserFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(UserKey).(string); ok {
		return v
	}
	return ""
}

// RoleFromContext returns the authenticated role from a request context.
func RoleFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(RoleKey).(string); ok {
		return v
	}
	return ""
}

func withUser(r *http.Request, user, role string) *http.Request {
	ctx := r.Context()
	ctx = context.WithValue(ctx, UserKey, user)
	ctx = context.WithValue(ctx, RoleKey, role)
	return r.WithContext(ctx)
}

// TokenVerifier validates a JWT string and returns the username and role.
type TokenVerifier func(tokenStr string) (user string, role string, err error)

// AuthMiddleware parses the JWT from the Authorization header or a cookie
// and injects the user identity into the request context.
func AuthMiddleware(verify TokenVerifier) func(http.Handler) http.Handler {
	if verify == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := ""
			if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
				token = strings.TrimPrefix(h, "Bearer ")
			}
			if token == "" {
				if c, err := r.Cookie("vpsctl_token"); err == nil {
					token = c.Value
				}
			}
			if token == "" {
				writeUnauthorized(w)
				return
			}
			user, role, err := verify(token)
			if err != nil {
				writeUnauthorized(w)
				return
			}
			next.ServeHTTP(w, withUser(r, user, role))
		})
	}
}

// --- Rate limiting ---

type requestInfo struct {
	count       int
	windowStart time.Time
}

// RateLimiter implements a sliding-window rate limiter per IP.
type RateLimiter struct {
	mu      sync.Mutex
	entries map[string]*requestInfo
	limit   int
	window  time.Duration
}

// NewRateLimiter creates a limiter allowing `limit` requests per `window`.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	if limit <= 0 {
		limit = 100
	}
	if window <= 0 {
		window = time.Minute
	}
	return &RateLimiter{
		entries: make(map[string]*requestInfo),
		limit:   limit,
		window:  window,
	}
}

func (rl *RateLimiter) allow(key string) bool {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()
	info, ok := rl.entries[key]
	if !ok || now.Sub(info.windowStart) >= rl.window {
		info = &requestInfo{count: 1, windowStart: now}
		rl.entries[key] = info
		return true
	}
	info.count++
	return info.count <= rl.limit
}

// RateLimitMiddleware limits requests per client IP.
func RateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter != nil && !limiter.allow(clientIP(r)) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// --- Security headers ---

// SecurityHeadersMiddleware sets recommended security headers on every response.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		next.ServeHTTP(w, r)
	})
}

// --- CORS ---

// CORSMiddleware allows cross-origin requests from configured origins.
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && originAllowed(origin, allowedOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func originAllowed(origin string, allowed []string) bool {
	for _, o := range allowed {
		if o == "*" || strings.EqualFold(o, origin) {
			return true
		}
	}
	return false
}

// --- Request logger ---

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Hijack forwards the hijack to support WebSocket upgrades.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := r.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Flush forwards flushes to the underlying writer.
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Push forwards HTTP/2 server push.
func (r *statusRecorder) Push(target string, opts *http.PushOptions) error {
	if p, ok := r.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

// RequestLoggerMiddleware logs method, path, status, duration, and IP.
func RequestLoggerMiddleware(log func(string)) func(http.Handler) http.Handler {
	if log == nil {
		log = func(string) {}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			log(r.Method + " " + r.URL.Path + " " +
				itoa(rec.status) + " " +
				time.Since(start).Round(time.Millisecond).String() + " " +
				clientIP(r))
		})
	}
}

// --- IP whitelist ---

// IPWhitelistMiddleware rejects clients whose IP is not in the allow list.
func IPWhitelistMiddleware(allowedIPs []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(allowedIPs) > 0 && !ipAllowed(clientIP(r), allowedIPs) {
				writeForbidden(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func ipAllowed(ip string, allowed []string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, a := range allowed {
		if _, ipNet, err := net.ParseCIDR(a); err == nil {
			if ipNet.Contains(parsed) {
				return true
			}
			continue
		}
		if allowedIP := net.ParseIP(a); allowedIP != nil && allowedIP.Equal(parsed) {
			return true
		}
	}
	return false
}

// clientIP extracts the client IP from the request, honoring X-Forwarded-For.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ClientIP exposes the client IP for other packages.
func ClientIP(r *http.Request) string { return clientIP(r) }

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
}

func writeForbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(map[string]string{"error": "access denied from this IP"})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
