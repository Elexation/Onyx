package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
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
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) Setup(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if len([]rune(body.Password)) < 8 {
		http.Error(w, `{"error":"password must be at least 8 characters"}`, http.StatusBadRequest)
		return
	}

	session, err := h.auth.Setup(body.Password)
	if err != nil {
		if err.Error() == "admin already exists" {
			http.Error(w, `{"error":"admin already exists"}`, http.StatusBadRequest)
			return
		}
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	slog.Info("security_event", "event", "admin_setup", "ip", h.clientIP(r))
	h.setSessionCookie(w, r, session)
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"csrfToken":     session.CSRFToken,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	session, err := h.auth.Login(body.Password)
	if err != nil {
		slog.Info("security_event", "event", "login_failure", "ip", h.clientIP(r))
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	slog.Info("security_event", "event", "login_success", "ip", h.clientIP(r))
	h.rl.RecordSuccess(r)
	h.setSessionCookie(w, r, session)
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"csrfToken":     session.CSRFToken,
	})
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	noStore(w)
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
	if len([]rune(body.NewPassword)) < 8 {
		http.Error(w, `{"error":"password must be at least 8 characters"}`, http.StatusBadRequest)
		return
	}

	session := middleware.SessionFromContext(r.Context())
	if session == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	newSession, err := h.auth.ChangePassword(body.CurrentPassword, body.NewPassword)
	if err != nil {
		if err.Error() == "invalid current password" {
			http.Error(w, `{"error":"invalid current password"}`, http.StatusBadRequest)
			return
		}
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	slog.Info("security_event", "event", "password_change", "ip", h.clientIP(r))
	h.setSessionCookie(w, r, newSession)
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

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// noStore tags an auth response as uncacheable. Set-Cookie responses aren't
// normally cached by well-behaved proxies, but Status leaks csrfToken — make
// the no-cache contract explicit.
func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}
