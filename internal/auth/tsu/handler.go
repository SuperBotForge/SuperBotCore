package tsu

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"SuperBotGo/internal/auth/userhttp"
	"SuperBotGo/internal/locale"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/user"
	"SuperBotGo/internal/weborigin"
)

const (
	stateCookieName = "tsu_auth_state"
	defaultReturnTo = "/"
	adminAuthError  = "admin_required"
)

type Handler struct {
	client       *Client
	stateStore   *StateStore
	linker       *Linker
	userRepo     user.UserRepository
	personLinker PersonLinker
	sessions     *userhttp.SessionManager
	adminAuth    AdminSessionManager
	secureCookie bool
	logger       *slog.Logger

	externalReturnToValidator ExternalReturnToValidator
}

type AdminSessionManager interface {
	SetSession(w http.ResponseWriter, userID int64)
	HasAdminAccess(ctx context.Context, userID int64) (bool, error)
	ClearSession(w http.ResponseWriter)
}

type ExternalReturnToValidator func(ctx context.Context, origin string) (bool, error)

func NewHandler(
	client *Client,
	stateStore *StateStore,
	linker *Linker,
	userRepo user.UserRepository,
	personLinker PersonLinker,
	sessions *userhttp.SessionManager,
	adminAuth AdminSessionManager,
	callbackURL string,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		client:       client,
		stateStore:   stateStore,
		linker:       linker,
		userRepo:     userRepo,
		personLinker: personLinker,
		sessions:     sessions,
		adminAuth:    adminAuth,
		secureCookie: strings.HasPrefix(callbackURL, "https://"),
		logger:       logger,
	}
}

func (h *Handler) SetExternalReturnToValidator(fn ExternalReturnToValidator) {
	h.externalReturnToValidator = fn
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/tsu/start", h.handleStartLogin)
	mux.HandleFunc("GET /oauth/authorize", h.handleLogin)
	mux.HandleFunc("GET /oauth/login", h.handleCallback)
}

func (h *Handler) handleStartLogin(w http.ResponseWriter, r *http.Request) {
	if h.sessions == nil || h.userRepo == nil {
		http.Error(w, "authentication is unavailable", http.StatusServiceUnavailable)
		return
	}

	loginURL, err := h.stateStore.GenerateLoginURL(h.sanitizeReturnToForRequest(r.Context(), r, r.URL.Query().Get("return_to")))
	if err != nil {
		h.logger.Error("tsu login start: failed to generate login URL", slog.Any("error", err))
		http.Error(w, "failed to start authentication", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, loginURL, http.StatusFound)
}

// handleLogin validates the state and redirects the user to TSU login page.
func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	if state == "" {
		http.Error(w, "missing state parameter", http.StatusBadRequest)
		return
	}

	if !h.stateStore.Verify(state) {
		http.Error(w, "invalid or expired state", http.StatusBadRequest)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Path:     "/oauth/",
		MaxAge:   int(stateTTL.Seconds()),
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, h.client.LoginURL(), http.StatusFound)
}

// handleCallback is called by TSU after user authentication.
// It supports both account-link and browser-login flows.
func (h *Handler) handleCallback(w http.ResponseWriter, r *http.Request) {
	tempToken := r.URL.Query().Get("token")
	if tempToken == "" {
		http.Error(w, "missing token parameter", http.StatusBadRequest)
		return
	}

	cookie, err := r.Cookie(stateCookieName)
	if err != nil {
		h.logger.Warn("tsu callback: missing state cookie")
		writeCallbackStatusPage(w,
			http.StatusOK,
			"Сессия входа истекла",
			"Попробуйте снова начать вход через ТГУ.Аккаунты.",
		)
		return
	}

	// Clear the cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    "",
		Path:     "/oauth/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})

	flow, ok := h.stateStore.Consume(cookie.Value)
	if !ok {
		h.logger.Warn("tsu callback: invalid or expired state")
		writeCallbackStatusPage(w,
			http.StatusOK,
			"Сессия входа истекла",
			"Попробуйте снова начать вход через ТГУ.Аккаунты.",
		)
		return
	}

	result, err := h.client.ExchangeToken(r.Context(), tempToken)
	if err != nil {
		h.logger.Error("tsu callback: token exchange failed", slog.Any("error", err))
		http.Error(w, "authentication failed, please try again", http.StatusInternalServerError)
		return
	}

	h.logger.Info("tsu callback: token exchanged",
		slog.String("flow", string(flow.Kind)),
		slog.String("account_id", result.AccountID))

	switch flow.Kind {
	case flowKindLink:
		h.handleLinkCallback(w, r, flow.UserID, result.AccountID)
	case flowKindLogin:
		h.handleWebLoginCallback(w, r, flow.ReturnTo, result.AccountID)
	default:
		http.Error(w, "invalid authentication flow", http.StatusBadRequest)
	}
}

func (h *Handler) handleLinkCallback(w http.ResponseWriter, r *http.Request, userID model.GlobalUserID, accountID string) {
	if err := h.linker.Link(r.Context(), userID, accountID); err != nil {
		h.logger.Error("tsu callback: account linking failed",
			slog.Int64("user_id", int64(userID)),
			slog.Any("error", err))
		http.Error(w, "account linking failed, please try again", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(successHTML))
}

func (h *Handler) handleWebLoginCallback(w http.ResponseWriter, r *http.Request, returnTo string, accountID string) {
	if h.sessions == nil || h.userRepo == nil {
		http.Error(w, "authentication is unavailable", http.StatusServiceUnavailable)
		return
	}

	userID, err := h.ensureWebUser(r.Context(), accountID)
	if err != nil {
		h.logger.Error("tsu callback: web login failed", slog.Any("error", err))
		http.Error(w, "authentication failed, please try again", http.StatusInternalServerError)
		return
	}

	safeReturnTo := h.sanitizeReturnToForRequest(r.Context(), r, returnTo)
	if strings.HasPrefix(safeReturnTo, "/admin") && h.adminAuth != nil {
		hasAccess, err := h.adminAuth.HasAdminAccess(r.Context(), int64(userID))
		if err != nil {
			h.logger.Error("tsu callback: failed to check admin access",
				slog.Int64("user_id", int64(userID)),
				slog.Any("error", err))
			http.Error(w, "authentication failed, please try again", http.StatusInternalServerError)
			return
		}
		if hasAccess {
			h.sessions.SetSession(w, userID)
			h.adminAuth.SetSession(w, int64(userID))
			http.Redirect(w, r, safeReturnTo, http.StatusFound)
			return
		}

		h.logger.Warn("tsu callback: denied admin login for non-admin user",
			slog.Int64("user_id", int64(userID)),
			slog.String("return_to", safeReturnTo))
		h.adminAuth.ClearSession(w)
		h.sessions.SetSession(w, userID)
		http.Redirect(w, r, withAuthError(safeReturnTo, adminAuthError), http.StatusFound)
		return
	}

	h.sessions.SetSession(w, userID)
	http.Redirect(w, r, safeReturnTo, http.StatusFound)
}

func (h *Handler) ensureWebUser(ctx context.Context, accountID string) (model.GlobalUserID, error) {
	existing, err := h.userRepo.FindByTsuAccountsID(ctx, accountID)
	if err != nil {
		return 0, err
	}
	if existing != nil {
		h.autoLinkPerson(ctx, existing.ID, accountID)
		return existing.ID, nil
	}

	userRec := &model.GlobalUser{
		PrimaryChannel: model.ChannelWeb,
		Locale:         locale.Default(),
	}
	saved, err := h.userRepo.Save(ctx, userRec)
	if err != nil {
		return 0, err
	}
	if err := h.userRepo.SetTsuAccountsID(ctx, saved.ID, accountID); err != nil {
		return 0, err
	}
	h.autoLinkPerson(ctx, saved.ID, accountID)
	return saved.ID, nil
}

func (h *Handler) autoLinkPerson(ctx context.Context, userID model.GlobalUserID, accountID string) {
	if h.personLinker == nil {
		return
	}
	if err := h.personLinker.LinkByExternalID(ctx, userID, accountID); err != nil {
		h.logger.Warn("tsu callback: auto-link person failed",
			slog.Int64("user_id", int64(userID)),
			slog.String("account_id", accountID),
			slog.Any("error", err))
	}
}

func (h *Handler) sanitizeReturnTo(ctx context.Context, value string) string {
	if local := sanitizeReturnTo(value); local != defaultReturnTo {
		return local
	}
	value = strings.TrimSpace(value)
	if value == "" || h == nil || h.externalReturnToValidator == nil {
		return defaultReturnTo
	}
	origin, err := weborigin.FromURL(value)
	if err != nil {
		return defaultReturnTo
	}
	allowed, err := h.externalReturnToValidator(ctx, origin)
	if err != nil {
		if h.logger != nil {
			h.logger.Warn("tsu login: failed to validate external return_to",
				slog.String("origin", origin),
				slog.Any("error", err))
		}
		return defaultReturnTo
	}
	if !allowed {
		return defaultReturnTo
	}
	return value
}

func (h *Handler) sanitizeReturnToForRequest(ctx context.Context, r *http.Request, value string) string {
	if local := h.sameOriginReturnTo(r, value); local != defaultReturnTo {
		return local
	}
	return h.sanitizeReturnTo(ctx, value)
}

func (h *Handler) sameOriginReturnTo(r *http.Request, value string) string {
	if r == nil {
		return defaultReturnTo
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultReturnTo
	}
	parsed, err := url.Parse(value)
	if err != nil || !parsed.IsAbs() || parsed.Host != r.Host {
		return defaultReturnTo
	}

	localPath := parsed.EscapedPath()
	if localPath == "" {
		localPath = defaultReturnTo
	}
	if parsed.RawQuery != "" {
		localPath += "?" + parsed.RawQuery
	}
	if parsed.Fragment != "" {
		localPath += "#" + parsed.EscapedFragment()
	}
	return sanitizeReturnTo(localPath)
}

func sanitizeReturnTo(value string) string {
	if value == "" {
		return defaultReturnTo
	}
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		return value
	}
	return defaultReturnTo
}

func withAuthError(path string, code string) string {
	parts := strings.SplitN(path, "?", 2)
	values := url.Values{}
	if len(parts) == 2 {
		var err error
		values, err = url.ParseQuery(parts[1])
		if err != nil {
			return path
		}
	}
	values.Set("auth_error", code)
	if len(parts) == 1 {
		return parts[0] + "?" + values.Encode()
	}
	return parts[0] + "?" + values.Encode()
}

const successHTML = `<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Аккаунт привязан</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            margin: 0;
            background: #f5f5f5;
        }
        .card {
            background: white;
            border-radius: 12px;
            padding: 2rem;
            text-align: center;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
            max-width: 400px;
        }
        .check { font-size: 3rem; margin-bottom: 1rem; }
        h1 { font-size: 1.25rem; margin: 0 0 0.5rem; }
        p { color: #666; margin: 0; }
    </style>
</head>
<body>
    <div class="card">
        <div class="check">&#10004;</div>
        <h1>Аккаунт успешно привязан</h1>
        <p>Можете вернуться в мессенджер.</p>
    </div>
</body>
</html>`

func writeCallbackStatusPage(w http.ResponseWriter, status int, title string, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(fmt.Sprintf(callbackStatusHTML, title, title, message)))
}

const callbackStatusHTML = `<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>%s</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            margin: 0;
            background: #f5f5f5;
            color: #171717;
        }
        .card {
            background: white;
            border-radius: 12px;
            padding: 2rem;
            text-align: center;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
            max-width: 420px;
        }
        .icon { font-size: 2.5rem; margin-bottom: 1rem; }
        h1 { font-size: 1.25rem; margin: 0 0 0.5rem; }
        p { color: #666; margin: 0; line-height: 1.5; }
    </style>
</head>
<body>
    <div class="card">
        <div class="icon">&#9888;</div>
        <h1>%s</h1>
        <p>%s</p>
    </div>
</body>
</html>`
