package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// AdminAuthMiddleware protects admin API endpoints and hosted plugin frontends.
// It accepts either a valid admin session cookie or a Bearer API key
// (for programmatic access).
// If no admin credentials exist in the database, all requests are allowed (initial setup).
type AdminAuthMiddleware struct {
	apiKey string
	auth   *AuthHandler
}

func NewAdminAuthMiddleware(auth *AuthHandler) *AdminAuthMiddleware {
	return &AdminAuthMiddleware{
		apiKey: auth.APIKey(),
		auth:   auth,
	}
}

// Wrap returns a handler that enforces authentication on API routes.
func (m *AdminAuthMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Always allow auth endpoints, TSU OAuth, admin static files, metrics, and trigger webhooks.
		if strings.HasPrefix(path, "/api/admin/auth/") ||
			strings.HasPrefix(path, "/api/auth/") ||
			strings.HasPrefix(path, "/api/files/") ||
			strings.HasPrefix(path, "/api/triggers/http/") ||
			strings.HasPrefix(path, "/oauth/") ||
			strings.HasPrefix(path, "/admin/") ||
			path == "/admin" ||
			(!strings.HasPrefix(path, "/api/") && !strings.HasPrefix(path, "/plugins/")) {
			next.ServeHTTP(w, r)
			return
		}

		// If no admin credentials exist, allow all requests (initial setup mode).
		if hasAdmins, err := m.auth.CredStore().HasAny(r.Context()); err == nil && !hasAdmins {
			next.ServeHTTP(w, r)
			return
		}

		if _, ok := m.auth.Authenticate(r); ok {
			next.ServeHTTP(w, r)
			return
		}

		// Fall back to Bearer API key (for programmatic access).
		if m.apiKey != "" {
			if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				token := auth[len("Bearer "):]
				if subtle.ConstantTimeCompare([]byte(token), []byte(m.apiKey)) == 1 {
					next.ServeHTTP(w, r)
					return
				}
			}
		}

		writeError(w, http.StatusUnauthorized, "authentication required")
	})
}
