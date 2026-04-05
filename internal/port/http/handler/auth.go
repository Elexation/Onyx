package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Elexation/onyx/internal/domain"
	"github.com/Elexation/onyx/internal/port/http/middleware"
	"github.com/Elexation/onyx/internal/service"
)

type AuthHandler struct {
	auth *service.AuthService
	rl   *middleware.RateLimiter
}

func NewAuthHandler(auth *service.AuthService, rl *middleware.RateLimiter) *AuthHandler {
	return &AuthHandler{auth: auth, rl: rl}
}

func (h *AuthHandler) Status(w http.ResponseWriter, r *http.Request) {
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
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if body.Password == "" {
		http.Error(w, `{"error":"password is required"}`, http.StatusBadRequest)
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

	setSessionCookie(w, r, session)
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"csrfToken":     session.CSRFToken,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	session, err := h.auth.Login(body.Password)
	if err != nil {
		h.rl.RecordFailure(r)
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	h.rl.RecordSuccess(r)
	setSessionCookie(w, r, session)
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"csrfToken":     session.CSRFToken,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	session := middleware.SessionFromContext(r.Context())
	if session == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	h.auth.Logout(session.ID)

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, session *domain.Session) {
	maxAge := int(session.ExpiresAt - time.Now().Unix())
	cookie := &http.Cookie{
		Name:     "session",
		Value:    session.ID,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
	if r.TLS != nil {
		cookie.Secure = true
	}
	http.SetCookie(w, cookie)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
