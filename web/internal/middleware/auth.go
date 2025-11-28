package middleware

import (
	"log"
	"net/http"

	"github.com/devilmonastery/hivemind/web/internal/session"
)

// AuthMiddleware handles authentication checks for requests
// Token refresh is now handled automatically by the per-request gRPC clients
type AuthMiddleware struct {
	sessionManager *session.Manager
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(sessionManager *session.Manager) *AuthMiddleware {
	return &AuthMiddleware{
		sessionManager: sessionManager,
	}
}

// RequireAuth is middleware that ensures the user is authenticated
// Token refresh is handled automatically by the gRPC interceptor in handlers
func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if user has a token
		token, err := m.sessionManager.GetToken(r)
		if err != nil || token == "" {
			log.Printf("No token found in session, redirecting to login")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Token exists, pass through to handler
		// The handler's gRPC client will automatically refresh if expired
		next.ServeHTTP(w, r)
	})
}
