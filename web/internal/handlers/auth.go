package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	authpb "github.com/devilmonastery/hivemind/api/generated/go/authpb"
)

// Login initiates OAuth authorization code flow or shows login page
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	// If already logged in with valid token, redirect to home
	if h.getCurrentUser(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Get available providers from server
	availableProviders, err := h.getAvailableProviders(r.Context())
	if err != nil {
		h.log.Error("failed to get available providers",
			slog.String("error", err.Error()))
		http.Error(w, "Failed to load authentication options", http.StatusInternalServerError)
		return
	}

	h.log.Debug("login: found available providers",
		slog.Int("count", len(availableProviders)))
	for _, p := range availableProviders {
		h.log.Debug("provider available",
			slog.String("name", p.Name),
			slog.String("client_id", p.ClientId))
	}

	// Check for admin access request
	if r.URL.Query().Get("admin") != "" {
		h.renderAdminLogin(w, r, availableProviders)
		return
	}

	// Get provider from query param
	provider := r.URL.Query().Get("provider")

	// If no provider specified, check for single-provider auto-redirect
	if provider == "" {
		// Check if user explicitly wants to see the login page
		if r.URL.Query().Get("show_page") != "" {
			h.renderLoginPage(w, r, availableProviders)
			return
		}

		// Single provider - render login page with auto-trigger
		if len(availableProviders) == 1 {
			h.log.Debug("single provider detected, rendering login page with auto-trigger",
				slog.String("provider", availableProviders[0].Name))
			h.renderLoginPage(w, r, availableProviders)
			return
		}

		// Multiple providers - show login page
		h.renderLoginPage(w, r, availableProviders)
		return
	}

	// Provider was specified, proceed with OAuth flow

	// Get OAuth config from gRPC server (no auth needed)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create an unauthenticated client (no session required for GetOAuthConfig)
	grpcClient, err := h.getUnauthenticatedClient()
	if err != nil {
		h.log.Error("failed to create gRPC client",
			slog.String("error", err.Error()))
		http.Error(w, "Failed to start login process", http.StatusInternalServerError)
		return
	}
	defer grpcClient.Close()

	configResp, err := grpcClient.AuthClient().GetOAuthConfig(ctx, &authpb.GetOAuthConfigRequest{})
	if err != nil {
		h.log.Error("failed to get OAuth config",
			slog.String("error", err.Error()))
		http.Error(w, "Failed to start login process", http.StatusInternalServerError)
		return
	}

	// Find the provider config
	var providerConfig *authpb.OAuthProvider
	for _, p := range configResp.Providers {
		if p.Name == provider {
			providerConfig = p
			break
		}
	}

	if providerConfig == nil {
		http.Error(w, "Unknown OAuth provider", http.StatusBadRequest)
		return
	}

	// Generate PKCE code verifier and challenge
	codeVerifier := generateCodeVerifier()
	codeChallenge := generateCodeChallenge(codeVerifier)

	// Generate state parameter for CSRF protection
	state := generateState()

	// Store state and code_verifier in session for the callback
	session, _ := h.sessionManager.GetSession(r)
	session.Values["oauth_state"] = state
	session.Values["oauth_code_verifier"] = codeVerifier
	session.Values["oauth_provider"] = provider
	if err := session.Save(r, w); err != nil {
		h.log.Error("failed to save session",
			slog.String("error", err.Error()))
		http.Error(w, "Failed to start login", http.StatusInternalServerError)
		return
	}

	// Build authorization URL using the template from server
	redirectURI := h.redirectURI

	// Check if provider has authorization URL
	if providerConfig.AuthorizationUrl == "" {
		http.Error(w, "Provider authorization URL not configured", http.StatusInternalServerError)
		return
	}

	// Substitute placeholders in the authorization URL template
	authURL := providerConfig.AuthorizationUrl
	authURL = strings.Replace(authURL, "{redirect_uri}", url.QueryEscape(redirectURI), 1)
	authURL = strings.Replace(authURL, "{state}", url.QueryEscape(state), 1)
	authURL = strings.Replace(authURL, "{code_challenge}", url.QueryEscape(codeChallenge), 1)

	// Redirect to OAuth provider
	http.Redirect(w, r, authURL, http.StatusFound)
}

// AuthCallback handles the OAuth callback
func (h *Handler) AuthCallback(w http.ResponseWriter, r *http.Request) {
	// Get state and code from query params
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	errorParam := r.URL.Query().Get("error")

	if errorParam != "" {
		h.log.Error("OAuth error received",
			slog.String("error", errorParam),
			slog.String("error_description", r.URL.Query().Get("error_description")))
		http.Error(w, "Authentication failed", http.StatusBadRequest)
		return
	}

	if code == "" {
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	// Verify state for CSRF protection
	session, _ := h.sessionManager.GetSession(r)
	savedState, ok := session.Values["oauth_state"].(string)
	if !ok || savedState != state {
		h.log.Warn("invalid state parameter - possible CSRF attempt",
			slog.String("expected", savedState),
			slog.String("received", state))
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	// Get code verifier and provider from session
	codeVerifier, ok := session.Values["oauth_code_verifier"].(string)
	if !ok {
		http.Error(w, "Missing code verifier", http.StatusBadRequest)
		return
	}

	provider, ok := session.Values["oauth_provider"].(string)
	if !ok {
		provider = "google"
	}

	// Exchange authorization code for token via gRPC
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create an unauthenticated client (no session required for ExchangeAuthCode)
	grpcClient, err := h.getUnauthenticatedClient()
	if err != nil {
		h.log.Error("failed to create gRPC client",
			slog.String("error", err.Error()))
		http.Error(w, "Failed to complete authentication", http.StatusInternalServerError)
		return
	}
	defer grpcClient.Close()

	h.log.Info("exchanging auth code",
		slog.String("provider", provider),
		slog.String("redirect_uri", h.redirectURI))

	// Must match the redirect_uri used in the authorization request
	redirectURI := h.redirectURI

	// Get timezone from session if available
	timezone := ""
	if tz, ok := session.Values["client_timezone"].(string); ok {
		timezone = tz
	}

	// Exchange authorization code for token (no retries for OAuth errors)
	resp, err := grpcClient.AuthClient().ExchangeAuthCode(ctx, &authpb.ExchangeAuthCodeRequest{
		Provider:     provider,
		Code:         code,
		CodeVerifier: codeVerifier,
		RedirectUri:  redirectURI,
		Timezone:     timezone,
	})
	if err != nil {
		h.log.Error("failed to exchange authorization code",
			slog.String("error", err.Error()),
			slog.String("provider", provider))
		http.Error(w, "Failed to complete authentication", http.StatusInternalServerError)
		return
	}

	// Store the API token and token ID in session
	if err := h.sessionManager.SetToken(r, w, resp.ApiToken, resp.TokenId); err != nil {
		h.log.Error("failed to save session",
			slog.String("error", err.Error()))
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	// Clear OAuth temporary data
	delete(session.Values, "oauth_state")
	delete(session.Values, "oauth_code_verifier")
	delete(session.Values, "oauth_provider")
	session.Save(r, w)

	// Redirect to home page
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Logout handles user logout
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	// Clear the session cookie
	h.sessionManager.ClearToken(r, w)

	// Redirect to home page
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Helper functions for PKCE

func generateCodeVerifier() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func generateCodeChallenge(verifier string) string {
	// S256: base64url(sha256(verifier))
	h := sha256.New()
	h.Write([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// getAvailableProviders fetches the list of available OAuth providers from the server
func (h *Handler) getAvailableProviders(ctx context.Context) ([]*authpb.OAuthProvider, error) {
	// Create an unauthenticated client
	grpcClient, err := h.getUnauthenticatedClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}
	defer grpcClient.Close()

	configResp, err := grpcClient.AuthClient().GetOAuthConfig(ctx, &authpb.GetOAuthConfigRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth config: %w", err)
	}

	return configResp.Providers, nil
}

// renderLoginPage renders the login page with available providers
func (h *Handler) renderLoginPage(w http.ResponseWriter, r *http.Request, providers []*authpb.OAuthProvider) {
	reason := r.URL.Query().Get("reason")
	var message string
	switch reason {
	case "expired":
		message = "Your session has expired. Please sign in again to continue."
	case "required":
		message = "Authentication required to access this page."
	}

	data := map[string]interface{}{
		"Message":       message,
		"Providers":     providers,
		"CurrentPage":   "login",
		"ShowAdminLink": len(providers) > 0, // Show admin link if there are OAuth providers
	}
	h.renderTemplate(w, "login.html", data)
}

// renderAdminLogin renders the admin login form
func (h *Handler) renderAdminLogin(w http.ResponseWriter, r *http.Request, providers []*authpb.OAuthProvider) {
	reason := r.URL.Query().Get("reason")
	var message string
	switch reason {
	case "expired":
		message = "Your session has expired. Please sign in again to continue."
	case "required":
		message = "Authentication required to access this page."
	case "invalid":
		message = "Invalid username or password. Please try again."
	}

	data := map[string]interface{}{
		"Message":     message,
		"Providers":   providers,
		"CurrentPage": "login",
		"AdminMode":   true,
	}
	h.renderTemplate(w, "login.html", data)
}

// AdminLogin handles admin login form submission
func (h *Handler) AdminLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// If already logged in with valid token, redirect to home
	if h.getCurrentUser(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		h.log.Error("Failed to parse admin login form",
			slog.String("error", err.Error()))
		http.Redirect(w, r, "/login?admin=1&reason=invalid", http.StatusSeeOther)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == "" || password == "" {
		http.Redirect(w, r, "/login?admin=1&reason=invalid", http.StatusSeeOther)
		return
	}

	// Authenticate with server
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcClient, err := h.getUnauthenticatedClient()
	if err != nil {
		h.log.Error("Failed to create gRPC client for admin login",
			slog.String("error", err.Error()))
		http.Error(w, "Failed to authenticate", http.StatusInternalServerError)
		return
	}
	defer grpcClient.Close()

	resp, err := grpcClient.AuthClient().AuthenticateLocal(ctx, &authpb.AuthenticateLocalRequest{
		Username: username,
		Password: password,
	})
	if err != nil {
		h.log.Error("Admin authentication failed",
			slog.String("username", username),
			slog.String("error", err.Error()))
		http.Redirect(w, r, "/login?admin=1&reason=invalid", http.StatusSeeOther)
		return
	}

	// Store the API token and token ID in session
	if err := h.sessionManager.SetToken(r, w, resp.ApiToken, resp.TokenId); err != nil {
		h.log.Error("Failed to save admin session",
			slog.String("error", err.Error()))
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	// Redirect to home page
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
