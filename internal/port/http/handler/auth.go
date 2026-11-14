package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Elexation/onyx/internal/domain"
	"github.com/Elexation/onyx/internal/port/http/middleware"
	"github.com/Elexation/onyx/internal/service"
)

type AuthHandler struct {
	auth         *service.AuthService
	rl           *middleware.RateLimiter
	trustedProxy bool
	requireHTTPS bool
}

func NewAuthHandler(auth *service.AuthService, rl *middleware.RateLimiter, trustedProxy, requireHTTPS bool) *AuthHandler {
	return &AuthHandler{auth: auth, rl: rl, trustedProxy: trustedProxy, requireHTTPS: requireHTTPS}
}

func (h *AuthHandler) clientIP(r *http.Request) string {
	return middleware.ClientIP(r, h.trustedProxy)
}

func (h *AuthHandler) Status(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	firstRun, err := h.auth.IsFirstRun()
	if err != nil {
		slog.Warn("auth status failed", "error", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	resp := map[string]any{
		"firstRun":      firstRun,
		"authenticated": false,
	}

	session := middleware.SessionFromContext(r.Context())
	if session != nil {
		resp["authenticated"] = true
		resp["csrfToken"] = session.CSRFToken
		h.ensureDeviceCookie(w, r)
	}

	writeJSON(w, http.StatusOK, resp)
}

// Give a signed-in browser without a valid device cookie one, e.g. sessions from before device cookies existed
func (h *AuthHandler) ensureDeviceCookie(w http.ResponseWriter, r *http.Request) {
	var token string
	if c, err := r.Cookie(deviceCookieName); err == nil {
		token = c.Value
	}
	valid, err := h.auth.DeviceTokenValid(token)
	if err != nil {
		slog.Warn("device cookie check failed", "error", err)
		return
	}
	if !valid {
		h.issueDeviceCookie(w, r)
	}
}

func (h *AuthHandler) Setup(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, `{"error":"invalid content type"}`, http.StatusUnsupportedMediaType)
		return
	}
	var body struct {
		Password  string `json:"password"`
		SetupCode string `json:"setupCode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if len(body.Password) > 1024 {
		http.Error(w, `{"error":"password must not exceed 1024 bytes"}`, http.StatusBadRequest)
		return
	}
	if len([]rune(body.Password)) < 8 {
		http.Error(w, `{"error":"password must be at least 8 characters"}`, http.StatusBadRequest)
		return
	}
	if len(body.SetupCode) > 256 {
		http.Error(w, `{"error":"invalid setup code"}`, http.StatusForbidden)
		return
	}

	session, err := h.auth.Setup(body.Password, body.SetupCode)
	if err != nil {
		if err.Error() == "admin already exists" {
			http.Error(w, `{"error":"admin already exists"}`, http.StatusBadRequest)
			return
		}
		if errors.Is(err, service.ErrInvalidSetupCode) {
			slog.Info("security_event", "event", "setup_code_rejected", "ip", h.clientIP(r))
			http.Error(w, `{"error":"invalid setup code"}`, http.StatusForbidden)
			return
		}
		slog.Warn("setup failed", "error", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	slog.Info("security_event", "event", "admin_setup", "ip", h.clientIP(r))
	h.rl.RecordSuccess(r)
	h.setSessionCookie(w, r, session)
	h.issueDeviceCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"csrfToken":     session.CSRFToken,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, `{"error":"invalid content type"}`, http.StatusUnsupportedMediaType)
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if len(body.Password) > 1024 {
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	var device string
	if c, err := r.Cookie(deviceCookieName); err == nil {
		device = c.Value
	}
	session, device, err := h.auth.Login(r.Context(), body.Password, device)
	if err != nil {
		var throttled *service.LoginThrottledError
		if errors.As(err, &throttled) {
			slog.Info("security_event", "event", "login_throttled", "ip", h.clientIP(r), "retry_after", throttled.RetryAfter.Round(time.Second))
			w.Header().Set("Retry-After", strconv.Itoa(int(throttled.RetryAfter.Seconds())+1))
			http.Error(w, `{"error":"too many attempts, try again later"}`, http.StatusTooManyRequests)
			return
		}
		if !errors.Is(err, service.ErrInvalidCredentials) {
			slog.Warn("login failed", "error", err)
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
		slog.Info("security_event", "event", "login_failure", "ip", h.clientIP(r))
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	slog.Info("security_event", "event", "login_success", "ip", h.clientIP(r))
	h.rl.RecordSuccess(r)
	h.setSessionCookie(w, r, session)
	if device != "" {
		h.setDeviceCookie(w, r, device)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"csrfToken":     session.CSRFToken,
	})
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, `{"error":"invalid content type"}`, http.StatusUnsupportedMediaType)
		return
	}
	var body struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if body.CurrentPassword == "" || body.NewPassword == "" {
		http.Error(w, `{"error":"current and new password are required"}`, http.StatusBadRequest)
		return
	}
	if len(body.CurrentPassword) > 1024 || len(body.NewPassword) > 1024 {
		http.Error(w, `{"error":"password must not exceed 1024 bytes"}`, http.StatusBadRequest)
		return
	}
	if len([]rune(body.NewPassword)) < 8 {
		http.Error(w, `{"error":"password must be at least 8 characters"}`, http.StatusBadRequest)
		return
	}

	session := middleware.SessionFromContext(r.Context())
	if session == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	newSession, err := h.auth.ChangePassword(r.Context(), body.CurrentPassword, body.NewPassword)
	if err != nil {
		if err.Error() == "invalid current password" {
			http.Error(w, `{"error":"invalid current password"}`, http.StatusBadRequest)
			return
		}
		slog.Warn("change password failed", "error", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	h.rl.RecordSuccess(r)
	slog.Info("security_event", "event", "password_change", "ip", h.clientIP(r))
	h.setSessionCookie(w, r, newSession)
	// The old device cookie was signed with the old password, so re-issue for this browser
	h.issueDeviceCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"csrfToken": newSession.CSRFToken,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	session := middleware.SessionFromContext(r.Context())
	if session == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	if err := h.auth.Logout(session.ID); err != nil {
		slog.Warn("logout failed to delete session", "error", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.requireHTTPS || middleware.IsHTTPS(r, h.trustedProxy),
		SameSite: http.SameSiteStrictMode,
	})

	writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
}

func (h *AuthHandler) setSessionCookie(w http.ResponseWriter, r *http.Request, session *domain.Session) {
	maxAge := int(session.ExpiresAt - time.Now().Unix())
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    session.ID,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.requireHTTPS || middleware.IsHTTPS(r, h.trustedProxy),
		SameSite: http.SameSiteStrictMode,
	})
}

// Marks a browser that logged in before; logout keeps it so the shared login limit still skips it
const deviceCookieName = "onyx_device"

func (h *AuthHandler) issueDeviceCookie(w http.ResponseWriter, r *http.Request) {
	token, err := h.auth.DeviceToken()
	if err != nil || token == "" {
		slog.Warn("device cookie not issued", "error", err)
		return
	}
	h.setDeviceCookie(w, r, token)
}

func (h *AuthHandler) setDeviceCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     deviceCookieName,
		Value:    token,
		Path:     "/api/auth",
		MaxAge:   int(service.DeviceCookieMaxAge.Seconds()),
		HttpOnly: true,
		Secure:   h.requireHTTPS || middleware.IsHTTPS(r, h.trustedProxy),
		SameSite: http.SameSiteStrictMode,
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// noStore tags an auth response as uncacheable. Set-Cookie responses aren't
// normally cached by well-behaved proxies, but Status leaks csrfToken, so make
// the no-cache contract explicit.
func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}
