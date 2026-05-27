package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"SuperBotGo/internal/auth/session"
	"SuperBotGo/internal/auth/userhttp"
	"SuperBotGo/internal/model"
)

const (
	sessionCookieName = "admin_session"
	sessionTTL        = 24 * time.Hour
)

// AuthHandler handles login / logout / session-check endpoints.
type AuthHandler struct {
	apiKey     string
	signer     *session.Signer
	credStore  *PgAdminCredStore
	userAuth   *userhttp.SessionManager
	tsuEnabled bool
}

func NewAuthHandler(apiKey string, credStore *PgAdminCredStore, userAuth *userhttp.SessionManager, tsuEnabled bool) *AuthHandler {
	return &AuthHandler{
		apiKey:     apiKey,
		signer:     session.NewSigner(apiKey, "admin auth"),
		credStore:  credStore,
		userAuth:   userAuth,
		tsuEnabled: tsuEnabled,
	}
}

func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/admin/auth/login", h.handleLogin)
	mux.HandleFunc("POST /api/admin/auth/logout", h.handleLogout)
	mux.HandleFunc("GET /api/admin/auth/check", h.handleCheck)
	mux.HandleFunc("PUT /api/admin/auth/password", h.handleChangePassword)
}

func (h *AuthHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Email == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	userID, err := h.credStore.Authenticate(r.Context(), body.Email, body.Password)
	if err != nil {
		slog.Warn("admin auth: failed login attempt", "email", body.Email, "remote", r.RemoteAddr)
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	h.setAuthenticatedSessions(w, userID)

	slog.Info("admin auth: successful login", "user_id", userID, "email", body.Email, "remote", r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user_id": userID})
}

func (h *AuthHandler) setAuthenticatedSessions(w http.ResponseWriter, userID int64) {
	token := h.signer.CreateToken(userID, sessionTTL)
	h.setSessionCookie(w, token, int(sessionTTL.Seconds()))
	if h.userAuth != nil {
		h.userAuth.SetSession(w, model.GlobalUserID(userID))
	}
}

func (h *AuthHandler) handleLogout(w http.ResponseWriter, _ *http.Request) {
	h.ClearSession(w)
	if h.userAuth != nil {
		h.userAuth.ClearSession(w)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *AuthHandler) handleCheck(w http.ResponseWriter, r *http.Request) {
	// If no admin credentials exist at all, auth is disabled (initial setup).
	hasAdmins, err := h.credStore.HasAny(r.Context())
	if err != nil {
		slog.Error("admin auth: failed to check admin credentials", "error", err)
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	if !hasAdmins {
		writeJSON(w, http.StatusOK, map[string]any{
			"authenticated":  true,
			"setup_required": true,
			"tsu_enabled":    h.tsuEnabled,
		})
		return
	}

	if userID, ok := h.Authenticate(r); ok {
		writeJSON(w, http.StatusOK, map[string]any{
			"authenticated": true,
			"user_id":       userID,
			"tsu_enabled":   h.tsuEnabled,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": false,
		"tsu_enabled":   h.tsuEnabled,
	})
}

func (h *AuthHandler) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.Authenticate(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "session expired")
		return
	}

	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(body.NewPassword) < 8 {
		writeError(w, http.StatusBadRequest, "new password must be at least 8 characters")
		return
	}

	// Verify current password via the credential store.
	if err := h.credStore.VerifyPassword(r.Context(), userID, body.CurrentPassword); err != nil {
		writeError(w, http.StatusForbidden, "current password is incorrect")
		return
	}

	if err := h.credStore.UpdatePassword(r.Context(), userID, body.NewPassword); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	slog.Info("admin auth: password changed", "user_id", userID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// Signer returns the session signer (used by the middleware).
func (h *AuthHandler) Signer() *session.Signer {
	return h.signer
}

// APIKey returns the configured API key (used by the middleware).
func (h *AuthHandler) APIKey() string {
	return h.apiKey
}

// CredStore returns the credentials store (used by the middleware for HasAny check).
func (h *AuthHandler) CredStore() *PgAdminCredStore {
	return h.credStore
}

func (h *AuthHandler) Authenticate(r *http.Request) (int64, bool) {
	return h.AuthenticateSession(r)
}

func (h *AuthHandler) AuthenticateSession(r *http.Request) (int64, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return 0, false
	}
	return h.signer.Validate(cookie.Value)
}

func (h *AuthHandler) SetSession(w http.ResponseWriter, userID int64) {
	h.setSessionCookie(w, h.signer.CreateToken(userID, sessionTTL), int(sessionTTL.Seconds()))
}

func (h *AuthHandler) HasAdminAccess(ctx context.Context, userID int64) (bool, error) {
	return h.credStore.HasUser(ctx, userID)
}

func (h *AuthHandler) ClearSession(w http.ResponseWriter) {
	h.setSessionCookie(w, "", -1)
}

func (h *AuthHandler) setSessionCookie(w http.ResponseWriter, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
