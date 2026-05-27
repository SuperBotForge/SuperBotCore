package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"SuperBotGo/internal/auth/userhttp"
)

func TestSetAuthenticatedSessionsSetsAdminAndUserSessions(t *testing.T) {
	t.Parallel()

	userSessions := userhttp.NewSessionManager("user-secret", false)
	handler := NewAuthHandler("admin-secret", nil, userSessions, false)
	rec := httptest.NewRecorder()

	handler.setAuthenticatedSessions(rec, 42)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range rec.Result().Cookies() {
		req.AddCookie(cookie)
	}

	adminUserID, adminOK := handler.AuthenticateSession(req)
	if !adminOK || adminUserID != 42 {
		t.Fatalf("admin session = %d, %v; want 42, true", adminUserID, adminOK)
	}

	userID, userOK := userSessions.Authenticate(req)
	if !userOK || userID != 42 {
		t.Fatalf("user session = %d, %v; want 42, true", userID, userOK)
	}
}
