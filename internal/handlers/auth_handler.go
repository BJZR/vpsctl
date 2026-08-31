package handlers

import (
	"net/http"
	"time"

	"github.com/vpsctl/vpsctl/internal/auth"
	"github.com/vpsctl/vpsctl/internal/middleware"
)

// AuthHandler serves authentication endpoints.
type AuthHandler struct {
	Manager *auth.Manager
	// RefreshStore is an in-memory map of valid refresh tokens.
	RefreshStore map[string]auth.RefreshToken
	cookieSecure bool
	cookieDomain string
}

// NewAuthHandler creates an auth handler.
func NewAuthHandler(m *auth.Manager) *AuthHandler {
	return &AuthHandler{
		Manager:      m,
		RefreshStore: make(map[string]auth.RefreshToken),
		cookieSecure: false,
	}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TOTPCode string `json:"totp_code"`
}

type loginResponse struct {
	Token        string                 `json:"token"`
	RefreshToken string                 `json:"refresh_token"`
	ExpiresAt    time.Time              `json:"expires_at"`
	User         map[string]interface{} `json:"user"`
}

// Login handles POST /api/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := parseJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	u, err := h.Manager.Authenticate(req.Username, req.Password, req.TOTPCode)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	token, expires, err := h.Manager.GenerateJWT(u)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	rt, err := auth.GenerateRefresh(u.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate refresh token")
		return
	}
	h.RefreshStore[rt.Token] = *rt

	h.setTokenCookie(w, token)

	writeJSON(w, http.StatusOK, loginResponse{
		Token:        token,
		RefreshToken: rt.Token,
		ExpiresAt:    expires,
		User:         h.withUser(u),
	})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Refresh handles POST /api/auth/refresh.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := parseJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	rt, ok := h.RefreshStore[req.RefreshToken]
	if !ok || time.Now().After(rt.ExpiresAt) {
		writeError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	u, ok := h.Manager.Get(rt.Username)
	if !ok {
		writeError(w, http.StatusUnauthorized, "user not found")
		return
	}

	token, expires, err := h.Manager.GenerateJWT(u)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	h.setTokenCookie(w, token)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token":      token,
		"expires_at": expires,
	})
}

// Logout handles POST /api/auth/logout.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Invalidate any refresh tokens and clear the cookie.
	for token := range h.RefreshStore {
		delete(h.RefreshStore, token)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "vpsctl_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

type setupTOTPResponse struct {
	Secret string `json:"secret"`
	URL    string `json:"url"`
}

// SetupTOTP handles POST /api/auth/setup-totp.
func (h *AuthHandler) SetupTOTP(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	secret, qrURL, err := h.Manager.RegisterTOTPSecret(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, setupTOTPResponse{Secret: secret, URL: qrURL})
}

type verifyTOTPRequest struct {
	Code string `json:"code"`
}

// VerifyTOTP handles POST /api/auth/verify-totp.
func (h *AuthHandler) VerifyTOTP(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	var req verifyTOTPRequest
	if err := parseJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.Manager.VerifyTOTP(user, req.Code); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "totp enabled"})
}

func (h *AuthHandler) withUser(u *auth.User) map[string]interface{} {
	return map[string]interface{}{
		"username":     u.Username,
		"role":         string(u.Role),
		"totp_enabled": u.RequireTOTP || (u.TOTPVerified && u.TOTPSecret != ""),
		"last_login":   u.LastLogin,
		"require_totp": u.RequireTOTP,
	}
}

// Me handles GET /api/auth/me.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	u, ok := h.Manager.Get(user)
	if !ok {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, h.withUser(u))
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ChangePassword handles POST /api/auth/change-password.
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	var req changePasswordRequest
	if err := parseJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.Manager.ChangePassword(user, req.OldPassword, req.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "password changed"})
}

// setTokenCookie sets an HTTP-only cookie with the JWT.
func (h *AuthHandler) setTokenCookie(w http.ResponseWriter, token string) {
	cookie := &http.Cookie{
		Name:     "vpsctl_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		MaxAge:   int((24 * time.Hour).Seconds()),
	}
	if h.cookieDomain != "" {
		cookie.Domain = h.cookieDomain
	}
	http.SetCookie(w, cookie)
}
