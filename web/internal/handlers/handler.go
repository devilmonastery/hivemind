package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/devilmonastery/hivemind/internal/client"
	"github.com/devilmonastery/hivemind/internal/pkg/urlutil"
	"github.com/devilmonastery/hivemind/web/internal/render"
	"github.com/devilmonastery/hivemind/web/internal/session"
)

// Handler holds dependencies for all web handlers
type Handler struct {
	serverAddress   string
	sessionManager  *session.Manager
	templates       *render.TemplateSet
	redirectURI     string
	discordGuildURL string // Cached Discord guild install URL
	discordUserURL  string // Cached Discord user install URL
	log             *slog.Logger
}

// New creates a new handler with dependencies
func New(serverAddress string, sessionManager *session.Manager, templates *render.TemplateSet, redirectURI string, logger *slog.Logger) *Handler {
	h := &Handler{
		serverAddress:  serverAddress,
		sessionManager: sessionManager,
		templates:      templates,
		redirectURI:    redirectURI,
		log:            logger.With(slog.String("component", "web_handler")),
	}

	// Fetch Discord install URLs at startup (cached for lifetime of handler)
	// Retry for up to 60 seconds to allow the gRPC server to start up
	// This blocks the web server from becoming "ready" until backend is available
	h.log.Info("waiting for gRPC backend to be ready")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	maxRetries := 12
	for i := 0; i < maxRetries; i++ {
		h.discordGuildURL, h.discordUserURL = h.getDiscordInstallURLs(ctx)
		if h.discordGuildURL != "" {
			h.log.Info("backend ready - Discord install URLs initialized successfully")
			break
		}

		if i < maxRetries-1 {
			h.log.Info("backend not ready yet, retrying",
				slog.Int("attempt", i+1),
				slog.Int("max_retries", maxRetries))
			time.Sleep(5 * time.Second)
		} else {
			h.log.Error("FATAL: Failed to connect to gRPC backend - check that the server is running and accessible",
				slog.Int("max_retries", maxRetries))
			os.Exit(1)
		}
	}

	return h
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

// newTemplateData creates a new template data map with standard fields populated
// Callers can add page-specific fields to the returned map
func (h *Handler) newTemplateData(r *http.Request) map[string]interface{} {
	return map[string]interface{}{
		"User":            h.getCurrentUser(r),
		"DiscordGuildURL": h.discordGuildURL,
		"DiscordUserURL":  h.discordUserURL,
	}
}

// renderTemplate renders a template with data
func (h *Handler) renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	if h.templates == nil {
		http.Error(w, "Templates not loaded", http.StatusInternalServerError)
		return
	}
	h.log.Debug("rendering template", slog.String("template", name))

	// Execute the named page template using the TemplateSet's Execute method
	// This will render the "base" template with the page's specific content
	err := h.templates.Execute(w, name, data)
	if err != nil {
		h.log.Error("template rendering failed",
			slog.String("template", name),
			slog.String("error", err.Error()))
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
	h.log.Info("clearing invalid session and redirecting to login")
	if err := h.sessionManager.ClearToken(r, w); err != nil {
		h.log.Error("error clearing session", slog.String("error", err.Error()))
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// getCurrentUser gets the current user info from the session token
// Returns nil if not authenticated, expired, or invalid
func (h *Handler) getCurrentUser(r *http.Request) map[string]interface{} {
	user, err := h.sessionManager.GetValidatedUser(r)
	if err != nil {
		if err != session.ErrNoToken {
			h.log.Debug("failed to get validated user",
				slog.String("error", err.Error()))
		}
		return nil
	}
	return user
}

// getDiscordInstallURLs fetches Discord client ID from OAuth providers and generates install URLs
func (h *Handler) getDiscordInstallURLs(ctx context.Context) (guildURL, userURL string) {
	providers, err := h.getAvailableProviders(ctx)
	if err != nil {
		h.log.Error("failed to get OAuth providers for Discord install URLs", slog.String("error", err.Error()))
		return "", ""
	}

	h.log.Info("found OAuth providers during initialization", slog.Int("count", len(providers)))

	// Find Discord provider
	var discordClientID string
	for _, p := range providers {
		h.log.Debug("OAuth provider",
			slog.String("name", p.Name),
			slog.String("client_id", p.ClientId))
		if p.Name == "discord" {
			discordClientID = p.ClientId
		}
	}

	if discordClientID == "" {
		h.log.Warn("Discord provider not found in OAuth config - bot invite links will not be available")
		return "", ""
	}

	// Discord OAuth permissions: 277025507392
	// This includes: Send Messages, Embed Links, Read Message History, etc.
	// TODO: move to config
	permissions := "277025507392"
	guildURL = urlutil.DiscordOAuthURL(discordClientID, permissions, 0)
	userURL = urlutil.DiscordOAuthURL(discordClientID, permissions, 1)
	return guildURL, userURL
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
		h.log.Error("failed to save timezone to session", slog.String("error", err.Error()))
		http.Error(w, "Failed to save timezone", http.StatusInternalServerError)
		return
	}

	// Return success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success": true}`))
}
