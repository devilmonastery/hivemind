package middleware

import (
	"log/slog"
	"net/http"

	"github.com/devilmonastery/hivemind/web/internal/session"
)

// AuthMiddleware handles authentication checks for requests
// Token refresh is now handled automatically by the per-request gRPC clients
type AuthMiddleware struct {
	sessionManager *session.Manager
	log            *slog.Logger
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(sessionManager *session.Manager, logger *slog.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		sessionManager: sessionManager,
		log:            logger.With(slog.String("component", "auth_middleware")),
	}
}

// RequireAuth is middleware that ensures the user is authenticated
// Token refresh is handled automatically by the gRPC interceptor in handlers
func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if user has a valid, non-expired token
		_, err := m.sessionManager.GetValidatedUser(r)
		if err != nil {
			m.log.Info("no valid token found in session, redirecting to login",
				slog.String("path", r.URL.Path),
				slog.String("method", r.Method),
				slog.String("reason", err.Error()))
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Token is valid, pass through to handler
		// The handler's gRPC client will automatically refresh if it expires during the request
		next.ServeHTTP(w, r)
	})
}
