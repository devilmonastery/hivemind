package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/devilmonastery/hivemind/internal/client"
	"github.com/devilmonastery/hivemind/web/internal/render"
	"github.com/devilmonastery/hivemind/web/internal/session"
)

// Handler holds dependencies for all web handlers
type Handler struct {
	serverAddress  string
	sessionManager *session.Manager
	templates      *render.TemplateSet
	redirectURI    string
}

// New creates a new handler with dependencies
func New(serverAddress string, sessionManager *session.Manager, templates *render.TemplateSet, redirectURI string) *Handler {
	return &Handler{
		serverAddress:  serverAddress,
		sessionManager: sessionManager,
		templates:      templates,
		redirectURI:    redirectURI,
	}
}

// getClient creates a per-request gRPC client with automatic token refresh
// This uses gRPC's built-in connection pooling, so it's efficient despite creating a new client per request
func (h *Handler) getClient(r *http.Request, w http.ResponseWriter) (*client.Client, error) {
	tm := session.NewSessionTokenManager(h.sessionManager, r, w)
	return client.NewClient(h.serverAddress, "", tm)
}

// getUnauthenticatedClient creates a gRPC client without any authentication
// Used for public endpoints like GetOAuthConfig
func (h *Handler) getUnauthenticatedClient() (*client.Client, error) {
	return client.NewClient(h.serverAddress, "", nil)
}

// renderTemplate renders a template with data
func (h *Handler) renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	if h.templates == nil {
		http.Error(w, "Templates not loaded", http.StatusInternalServerError)
		return
	}
	log.Printf("Rendering template: %s", name)

	// Execute the named page template using the TemplateSet's Execute method
	// This will render the "base" template with the page's specific content
	err := h.templates.Execute(w, name, data)
	if err != nil {
		log.Printf("Error rendering template %s: %v", name, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// isAuthError checks if a gRPC error indicates authentication failure
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	return st.Code() == codes.Unauthenticated || st.Code() == codes.NotFound
}

// clearSessionAndRedirect clears the session and redirects to login
func (h *Handler) clearSessionAndRedirect(w http.ResponseWriter, r *http.Request) {
	log.Printf("Clearing invalid session and redirecting to login")
	if err := h.sessionManager.ClearToken(r, w); err != nil {
		log.Printf("Error clearing session: %v", err)
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// getCurrentUser gets the current user info from the session token
// Returns nil if not authenticated
func (h *Handler) getCurrentUser(r *http.Request) map[string]interface{} {
	tokenString, err := h.sessionManager.GetToken(r)
	if err != nil || tokenString == "" {
		return nil
	}

	// Parse JWT without verification (since it's already verified by the backend)
	// We just need to extract the claims
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		log.Printf("Failed to parse JWT: %v", err)
		return nil
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		log.Printf("Failed to get claims from JWT")
		return nil
	}

	// Extract user info from claims
	user := make(map[string]interface{})

	// Email can be in either "email" or "username" claim
	// (our JWT uses "username" for the email address)
	if email, ok := claims["email"].(string); ok {
		user["Email"] = email
	} else if username, ok := claims["username"].(string); ok {
		user["Email"] = username
	}

	if userID, ok := claims["user_id"].(string); ok {
		user["UserID"] = userID
	}

	if displayName, ok := claims["display_name"].(string); ok {
		user["DisplayName"] = displayName
	}

	if role, ok := claims["role"].(string); ok {
		user["Role"] = role
	}

	if picture, ok := claims["picture"].(string); ok {
		user["Picture"] = picture
	}

	if timezone, ok := claims["timezone"].(string); ok {
		user["Timezone"] = timezone
	}

	return user
}

// isAuthError checks if an error is an authentication error
// Returns true if the error indicates authentication/authorization failure
func (h *Handler) isAuthError(err error) bool {
	if err == nil {
		return false
	}

	// Check for gRPC status codes
	if st, ok := status.FromError(err); ok {
		code := st.Code()
		return code == codes.Unauthenticated || code == codes.PermissionDenied
	}

	// Check for common auth error strings
	errStr := err.Error()
	return errStr == "invalid token" ||
		errStr == "token expired" ||
		errStr == "no refresh token available - please login again"
}

// SetTimezone stores the client's timezone in the session for use during OAuth flow
func (h *Handler) SetTimezone(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse JSON request
	type timezoneRequest struct {
		Timezone string `json:"timezone"`
	}

	var req timezoneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate timezone
	if req.Timezone == "" {
		http.Error(w, "Timezone is required", http.StatusBadRequest)
		return
	}

	// Store timezone in session
	session, _ := h.sessionManager.GetSession(r)
	session.Values["client_timezone"] = req.Timezone
	if err := session.Save(r, w); err != nil {
		log.Printf("Failed to save timezone to session: %v", err)
		http.Error(w, "Failed to save timezone", http.StatusInternalServerError)
		return
	}

	// Return success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success": true}`))
}
