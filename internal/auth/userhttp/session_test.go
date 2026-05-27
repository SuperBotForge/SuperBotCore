package userhttp

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSessionManagerSetSessionUsesConfiguredSameSite(t *testing.T) {
	t.Parallel()

	sessions := NewSessionManagerWithSameSite("test-secret", true, http.SameSiteNoneMode)
	rec := httptest.NewRecorder()

	sessions.SetSession(rec, 42)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies len = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != SessionCookieName {
		t.Fatalf("cookie name = %q, want %q", cookie.Name, SessionCookieName)
	}
	if !cookie.Secure {
		t.Fatal("expected secure cookie")
	}
	if cookie.SameSite != http.SameSiteNoneMode {
		t.Fatalf("cookie SameSite = %v, want %v", cookie.SameSite, http.SameSiteNoneMode)
	}
}

func TestSameSiteModeDefaultsToLax(t *testing.T) {
	t.Parallel()

	if got := SameSiteMode("unexpected"); got != http.SameSiteLaxMode {
		t.Fatalf("SameSiteMode() = %v, want %v", got, http.SameSiteLaxMode)
	}
}
