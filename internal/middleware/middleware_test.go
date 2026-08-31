package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestRateLimiterAllow(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		if !rl.allow("1.2.3.4") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if rl.allow("1.2.3.4") {
		t.Error("4th request should be rate limited")
	}
	if !rl.allow("5.6.7.8") {
		t.Error("a different IP should not be limited")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	limiter := NewRateLimiter(2, time.Minute)
	h := RateLimitMiddleware(limiter)(okHandler())

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200", i+1, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}
}

func TestAuthMiddleware(t *testing.T) {
	verify := func(token string) (string, string, error) {
		if token == "good-token" {
			return "alice", "admin", nil
		}
		return "", "", errors.New("bad token")
	}
	h := AuthMiddleware(verify)(okHandler())

	// No token -> 401.
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("missing token: got %d, want 401", rec.Code)
	}

	// Bad token -> 401.
	req = httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("bad token: got %d, want 401", rec.Code)
	}

	// Good token -> 200 and identity in context.
	req = httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	rec = httptest.NewRecorder()
	gotUser := ""
	h = AuthMiddleware(verify)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || gotUser != "alice" {
		t.Errorf("expected 200 + alice, got %d + %q", rec.Code, gotUser)
	}
}

func TestIPWhitelistMiddleware(t *testing.T) {
	h := IPWhitelistMiddleware([]string{"10.0.0.0/8", "192.168.1.5"})(okHandler())

	allowed := httptest.NewRequest(http.MethodGet, "/x", nil)
	allowed.RemoteAddr = "10.1.2.3:5000"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, allowed)
	if rec.Code != http.StatusOK {
		t.Errorf("allowed IP got %d, want 200", rec.Code)
	}

	denied := httptest.NewRequest(http.MethodGet, "/x", nil)
	denied.RemoteAddr = "8.8.8.8:5000"
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, denied)
	if rec.Code != http.StatusForbidden {
		t.Errorf("denied IP got %d, want 403", rec.Code)
	}

	// Empty whitelist allows everyone.
	h2 := IPWhitelistMiddleware(nil)(okHandler())
	rec = httptest.NewRecorder()
	h2.ServeHTTP(rec, denied)
	if rec.Code != http.StatusOK {
		t.Errorf("empty whitelist got %d, want 200", rec.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	h := SecurityHeadersMiddleware(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	for _, hdr := range []string{"X-Content-Type-Options", "X-Frame-Options", "Content-Security-Policy", "Strict-Transport-Security"} {
		if rec.Header().Get(hdr) == "" {
			t.Errorf("missing security header %s", hdr)
		}
	}
}

func TestClientIPFallback(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := ClientIP(req); !strings.Contains(got, "") {
		t.Fatalf("unexpected IP derivation: %q", got)
	}
}
