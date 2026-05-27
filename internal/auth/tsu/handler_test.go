package tsu

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"SuperBotGo/internal/auth/userhttp"
	"SuperBotGo/internal/model"
)

func TestHandleWebLoginCallback_NonAdminRedirectsWithError(t *testing.T) {
	t.Parallel()

	userRepo := &stubUserRepository{
		userByAccountID: map[string]*model.GlobalUser{
			"tsu-123": {
				ID:             42,
				PrimaryChannel: model.ChannelWeb,
				Locale:         "ru",
			},
		},
	}
	adminAuth := &stubAdminSessionManager{}
	handler := &Handler{
		userRepo:  userRepo,
		sessions:  userhttp.NewSessionManager("test-secret", false),
		adminAuth: adminAuth,
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	req := httptest.NewRequest(http.MethodGet, "/oauth/login", nil)
	rec := httptest.NewRecorder()

	handler.handleWebLoginCallback(rec, req, "/admin/plugins", "tsu-123")

	res := rec.Result()
	if res.StatusCode != http.StatusFound {
		t.Fatalf("expected redirect status %d, got %d", http.StatusFound, res.StatusCode)
	}
	if got := res.Header.Get("Location"); got != "/admin/plugins?auth_error=admin_required" {
		t.Fatalf("expected redirect with auth error, got %q", got)
	}
	if !adminAuth.cleared {
		t.Fatal("expected admin session to be cleared")
	}
	if adminAuth.sessionUserID != 0 {
		t.Fatalf("expected no admin session to be created, got user %d", adminAuth.sessionUserID)
	}
	if len(res.Cookies()) == 0 {
		t.Fatal("expected response to include cookies")
	}

	var hasUserSession bool
	for _, cookie := range res.Cookies() {
		if cookie.Name == userhttp.SessionCookieName && cookie.MaxAge > 0 {
			hasUserSession = true
		}
	}
	if !hasUserSession {
		t.Fatal("expected user session cookie to be set")
	}
}

func TestHandleWebLoginCallback_AdminRedirectSetsUserSession(t *testing.T) {
	t.Parallel()

	userRepo := &stubUserRepository{
		userByAccountID: map[string]*model.GlobalUser{
			"tsu-123": {
				ID:             42,
				PrimaryChannel: model.ChannelWeb,
				Locale:         "ru",
			},
		},
	}
	adminAuth := &stubAdminSessionManager{hasAccess: true}
	handler := &Handler{
		userRepo:  userRepo,
		sessions:  userhttp.NewSessionManager("test-secret", false),
		adminAuth: adminAuth,
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	req := httptest.NewRequest(http.MethodGet, "/oauth/login", nil)
	rec := httptest.NewRecorder()

	handler.handleWebLoginCallback(rec, req, "/admin/plugins", "tsu-123")

	res := rec.Result()
	if res.StatusCode != http.StatusFound {
		t.Fatalf("expected redirect status %d, got %d", http.StatusFound, res.StatusCode)
	}
	if got := res.Header.Get("Location"); got != "/admin/plugins" {
		t.Fatalf("expected admin redirect, got %q", got)
	}
	if adminAuth.sessionUserID != 42 {
		t.Fatalf("expected admin session for user 42, got %d", adminAuth.sessionUserID)
	}
	if !hasCookie(res.Cookies(), userhttp.SessionCookieName) {
		t.Fatal("expected user session cookie to be set")
	}
}

func TestWithAuthError_PreservesExistingQuery(t *testing.T) {
	t.Parallel()

	got := withAuthError("/admin/plugins?page=2", adminAuthError)
	want := "/admin/plugins?auth_error=admin_required&page=2"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestSanitizeReturnTo_AllowsRegisteredExternalURL(t *testing.T) {
	t.Parallel()

	handler := &Handler{
		externalReturnToValidator: func(_ context.Context, origin string) (bool, error) {
			return origin == "http://localhost:5173", nil
		},
	}

	returnTo := "http://localhost:5173/admin/schedule?tab=main#today"
	if got := handler.sanitizeReturnTo(t.Context(), returnTo); got != returnTo {
		t.Fatalf("sanitizeReturnTo() = %q, want %q", got, returnTo)
	}
}

func TestSanitizeReturnTo_RejectsUnregisteredExternalURL(t *testing.T) {
	t.Parallel()

	handler := &Handler{
		externalReturnToValidator: func(context.Context, string) (bool, error) {
			return false, nil
		},
	}

	if got := handler.sanitizeReturnTo(t.Context(), "https://evil.example/admin"); got != defaultReturnTo {
		t.Fatalf("sanitizeReturnTo() = %q, want %q", got, defaultReturnTo)
	}
}

func TestSanitizeReturnToForRequest_AllowsSameOriginAbsoluteURL(t *testing.T) {
	t.Parallel()

	handler := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:4000/api/auth/tsu/start", nil)
	returnTo := "http://127.0.0.1:4000/plugins/schedule/app/?room=203#today"

	got := handler.sanitizeReturnToForRequest(t.Context(), req, returnTo)
	want := "/plugins/schedule/app/?room=203#today"
	if got != want {
		t.Fatalf("sanitizeReturnToForRequest() = %q, want %q", got, want)
	}
}

func TestSanitizeReturnToForRequest_RejectsDifferentHostAbsoluteURL(t *testing.T) {
	t.Parallel()

	handler := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:4000/api/auth/tsu/start", nil)

	got := handler.sanitizeReturnToForRequest(t.Context(), req, "http://evil.example/plugins/schedule/app/")
	if got != defaultReturnTo {
		t.Fatalf("sanitizeReturnToForRequest() = %q, want %q", got, defaultReturnTo)
	}
}

func TestHandleCallback_MissingStateCookieShowsFriendlyPage(t *testing.T) {
	t.Parallel()

	handler := &Handler{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	req := httptest.NewRequest(http.MethodGet, "/oauth/login?token=temp-token", nil)
	rec := httptest.NewRecorder()

	handler.handleCallback(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, "Сессия входа истекла") {
		t.Fatalf("expected friendly title in body, got %q", text)
	}
	if !strings.Contains(text, "Попробуйте снова начать вход через ТГУ.Аккаунты.") {
		t.Fatalf("expected retry message in body, got %q", text)
	}
}

type stubUserRepository struct {
	userByAccountID map[string]*model.GlobalUser
}

func (s *stubUserRepository) FindByID(context.Context, model.GlobalUserID) (*model.GlobalUser, error) {
	return nil, nil
}

func (s *stubUserRepository) FindByTsuAccountsID(_ context.Context, tsuAccountsID string) (*model.GlobalUser, error) {
	return s.userByAccountID[tsuAccountsID], nil
}

func (s *stubUserRepository) Save(context.Context, *model.GlobalUser) (*model.GlobalUser, error) {
	panic("unexpected call to Save")
}

func (s *stubUserRepository) Delete(context.Context, model.GlobalUserID) error {
	return nil
}

func (s *stubUserRepository) SetTsuAccountsID(context.Context, model.GlobalUserID, string) error {
	return nil
}

func (s *stubUserRepository) UpdateLocale(context.Context, model.GlobalUserID, string) error {
	return nil
}

type stubAdminSessionManager struct {
	cleared       bool
	hasAccess     bool
	sessionUserID int64
}

func (s *stubAdminSessionManager) SetSession(_ http.ResponseWriter, userID int64) {
	s.sessionUserID = userID
}

func (s *stubAdminSessionManager) HasAdminAccess(context.Context, int64) (bool, error) {
	return s.hasAccess, nil
}

func (s *stubAdminSessionManager) ClearSession(http.ResponseWriter) {
	s.cleared = true
}

func hasCookie(cookies []*http.Cookie, name string) bool {
	for _, cookie := range cookies {
		if cookie.Name == name && cookie.MaxAge > 0 {
			return true
		}
	}
	return false
}
