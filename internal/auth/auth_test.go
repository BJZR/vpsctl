package auth

import (
	"testing"
	"time"
)

func TestHashAndValidatePassword(t *testing.T) {
	hash, err := HashPassword("sup3r-secret")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if !ValidatePassword(hash, "sup3r-secret") {
		t.Error("ValidatePassword should accept the correct password")
	}
	if ValidatePassword(hash, "wrong") {
		t.Error("ValidatePassword should reject a wrong password")
	}
}

func TestManagerLockout(t *testing.T) {
	m, err := NewManager(t.TempDir()+"/users.json", t.TempDir(), []byte("test-secret"), 3, time.Minute)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if err := m.RegisterUser("testuser", "pass1234", RoleAdmin, false); err != nil {
		t.Fatalf("RegisterUser failed: %v", err)
	}

	for i := 0; i < 3; i++ {
		if _, err := m.Authenticate("admin", "bad", ""); err == nil {
			t.Fatalf("attempt %d should fail", i+1)
		}
	}

	if _, err := m.Authenticate("admin", "pass1234", ""); err == nil {
		t.Error("login should be blocked after max failed attempts")
	} else if !IsLocked(err) {
		t.Errorf("expected IsLocked error, got: %v", err)
	}
}

func TestTOTPFlow(t *testing.T) {
	m, err := NewManager(t.TempDir()+"/users.json", t.TempDir(), []byte("test-secret"), 3, time.Minute)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if err := m.RegisterUser("testuser", "pass1234", RoleAdmin, false); err != nil {
		t.Fatalf("RegisterUser failed: %v", err)
	}

	secret, _, err := m.RegisterTOTPSecret("admin")
	if err != nil {
		t.Fatalf("RegisterTOTPSecret failed: %v", err)
	}

	// Generate a real code from the secret.
	code, err := GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode failed: %v", err)
	}

	if err := m.VerifyTOTP("admin", code); err != nil {
		t.Fatalf("VerifyTOTP should accept a valid code: %v", err)
	}
	if err := m.VerifyTOTP("admin", "000000"); err == nil {
		t.Error("VerifyTOTP should reject an invalid code")
	}
}

func TestJWTRoundTrip(t *testing.T) {
	m, err := NewManager(t.TempDir()+"/users.json", t.TempDir(), []byte("test-secret"), 3, time.Minute)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	u := &User{Username: "admin", Role: RoleAdmin}
	token, exp, err := m.GenerateJWT(u)
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}
	if exp.Before(time.Now()) {
		t.Error("token expiry should be in the future")
	}

	claims, err := m.ParseJWT(token)
	if err != nil {
		t.Fatalf("ParseJWT failed: %v", err)
	}
	if claims.User != "admin" || claims.Role != RoleAdmin {
		t.Errorf("unexpected claims: %+v", claims)
	}

	if _, err := m.ParseJWT(token + "tampered"); err == nil {
		t.Error("ParseJWT should reject tampered tokens")
	}
}

func TestBootstrap(t *testing.T) {
	m, err := NewManager(t.TempDir()+"/users.json", t.TempDir(), []byte("test-secret"), 3, time.Minute)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if err := m.EnsureBootstrap(); err != nil {
		t.Fatalf("EnsureBootstrap failed: %v", err)
	}
	if _, ok := m.Get("admin"); !ok {
		t.Error("bootstrap should create the admin user")
	}
	// Second call must be a no-op.
	if err := m.EnsureBootstrap(); err != nil {
		t.Fatalf("EnsureBootstrap is not idempotent: %v", err)
	}
}
