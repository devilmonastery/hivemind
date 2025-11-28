package cli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	authpb "github.com/devilmonastery/hivemind/api/generated/go/authpb"
)

// formatDuration formats a duration in a human-friendly way (e.g., "2 days, 3 hours, 45 minutes")
func formatDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}

	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	var parts []string
	if days > 0 {
		if days == 1 {
			parts = append(parts, "1 day")
		} else {
			parts = append(parts, fmt.Sprintf("%d days", days))
		}
	}
	if hours > 0 {
		if hours == 1 {
			parts = append(parts, "1 hour")
		} else {
			parts = append(parts, fmt.Sprintf("%d hours", hours))
		}
	}
	if minutes > 0 {
		if minutes == 1 {
			parts = append(parts, "1 minute")
		} else {
			parts = append(parts, fmt.Sprintf("%d minutes", minutes))
		}
	}
	if len(parts) == 0 && seconds > 0 {
		if seconds == 1 {
			parts = append(parts, "1 second")
		} else {
			parts = append(parts, fmt.Sprintf("%d seconds", seconds))
		}
	}
	if len(parts) == 0 {
		return "0 seconds"
	}

	// Join parts with commas and "and" for the last one
	if len(parts) == 1 {
		return parts[0]
	}
	if len(parts) == 2 {
		return parts[0] + " and " + parts[1]
	}
	// For 3+ parts, join all but last with ", " and add "and" before last
	result := ""
	for i := 0; i < len(parts)-1; i++ {
		if i > 0 {
			result += ", "
		}
		result += parts[i]
	}
	result += " and " + parts[len(parts)-1]
	return result
}

func newAuthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication commands",
		Long:  `Manage authentication for the Hivemind CLI`,
	}

	cmd.AddCommand(newAuthLoginCommand())
	cmd.AddCommand(newAuthLogoutCommand())
	cmd.AddCommand(newAuthStatusCommand())
	cmd.AddCommand(newAuthTokenCommand())

	return cmd
}

func newAuthLoginCommand() *cobra.Command {
	var (
		deviceName string
		username   string
		password   string
		provider   string
		adminFlag  bool
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to the Hivemind server",
		Long: `Authenticate with the Hivemind server using OIDC provider or local username/password.

By default, uses browser-based authorization code flow.

Examples:
  # Login with specific provider (opens browser)
  hivemind auth login --provider google

  # Login with local admin account
  hivemind auth login --admin --username user@example.com

  # Login and specify device name
  hivemind auth login --provider google --device-name "my-laptop"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := slog.Default().With("command", "login")
			logger.Info("Starting login process",
				"provider", provider,
				"device_name", deviceName,
				"admin_mode", adminFlag)

			// If admin flag is set, force local authentication
			if adminFlag {
				logger.Info("Admin mode selected, using local authentication")
				return loginLocal(username, password, deviceName)
			}

			// If provider is specified, use OIDC
			if provider != "" {
				return loginWithOIDCBrowser(provider, deviceName)
			}

			// No provider specified - auto-detect or prompt for selection
			return loginWithAutoProviderDetection(deviceName)
		},
	}

	cmd.Flags().StringVar(&deviceName, "device-name", "", "Name for this device (default: hostname)")
	cmd.Flags().StringVarP(&username, "username", "u", "", "Username/email for local auth (if not provided, will prompt)")
	cmd.Flags().StringVarP(&password, "password", "p", "", "Password for local auth (if not provided, will prompt)")
	cmd.Flags().StringVar(&provider, "provider", "", "OAuth provider (auto-detected if only one available)")
	cmd.Flags().BoolVar(&adminFlag, "admin", false, "Use local admin authentication (skips provider detection)")

	return cmd
}

// loginLocal handles local username/password authentication
func loginLocal(username, password, deviceName string) error {
	// Get credentials from user or flags
	var err error
	if username == "" || password == "" {
		username, password, err = promptCredentials()
		if err != nil {
			return err
		}
	}

	// Use hostname as device name if not provided
	if deviceName == "" {
		hostname, err := os.Hostname()
		if err != nil {
			deviceName = "cli-client"
		} else {
			deviceName = hostname
		}
	}

	// Connect to server (no auth needed for login)
	grpcClient, err := NewUnauthenticatedClient()
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer grpcClient.Close()

	// Create auth client
	authClient := authpb.NewAuthServiceClient(grpcClient.Conn())

	// Authenticate
	resp, err := authClient.AuthenticateLocal(context.Background(), &authpb.AuthenticateLocalRequest{
		Username:   username,
		Password:   password,
		DeviceName: deviceName,
		Scopes:     []string{"hivemind:read", "hivemind:write"},
	})
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Store credentials
	creds := &Credentials{
		AccessToken: resp.ApiToken,
		TokenID:     resp.TokenId,
		UserID:      resp.User.UserId,
		Username:    resp.User.Email, // use email as username
		ExpiresAt:   resp.ExpiresAt.AsTime(),
	}

	if err := SaveCredentials(creds); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	fmt.Printf("✓ Successfully logged in as %s\n", resp.User.Email)
	fmt.Printf("  Token expires: %s\n", resp.ExpiresAt.AsTime().Format("2006-01-02 15:04:05"))

	return nil
}

// loginWithAutoProviderDetection handles automatic provider detection and login
func loginWithAutoProviderDetection(deviceName string) error {
	logger := slog.Default().With("function", "loginWithAutoProviderDetection")

	// Connect to server to get OAuth configuration
	grpcClient, err := NewUnauthenticatedClient()
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer grpcClient.Close()

	authClient := authpb.NewAuthServiceClient(grpcClient.Conn())

	// Get OAuth config from server
	oauthConfigResp, err := authClient.GetOAuthConfig(context.Background(), &authpb.GetOAuthConfigRequest{})
	if err != nil {
		return fmt.Errorf("failed to get OAuth config from server: %w", err)
	}

	logger.Info("Retrieved OAuth configuration", "provider_count", len(oauthConfigResp.Providers))

	// All providers from server are available
	availableProviders := oauthConfigResp.Providers

	logger.Info("Available providers", "available_count", len(availableProviders))

	if len(availableProviders) == 0 {
		return fmt.Errorf("no OAuth providers available. Use --admin for local authentication")
	}

	if len(availableProviders) == 1 {
		// Single provider - auto-select it
		provider := availableProviders[0]
		logger.Info("Single provider detected, auto-selecting", "provider", provider.Name)
		fmt.Printf("Using %s for authentication...\n", provider.Name)
		return loginWithOIDCBrowser(provider.Name, deviceName)
	}

	// Multiple providers - prompt user to choose
	fmt.Println("\nMultiple authentication providers available:")
	for i, p := range availableProviders {
		fmt.Printf("  %d. %s\n", i+1, p.Name)
	}
	fmt.Printf("  %d. Local admin login\n", len(availableProviders)+1)

	fmt.Print("\nSelect authentication method (1-" + fmt.Sprintf("%d", len(availableProviders)+1) + "): ")
	var choice int
	_, err = fmt.Scanf("%d", &choice)
	if err != nil {
		return fmt.Errorf("invalid selection: %w", err)
	}

	if choice < 1 || choice > len(availableProviders)+1 {
		return fmt.Errorf("invalid selection: must be between 1 and %d", len(availableProviders)+1)
	}

	if choice == len(availableProviders)+1 {
		// User chose local admin login
		logger.Info("User selected local admin authentication")
		return loginLocal("", "", deviceName)
	}

	// User chose an OAuth provider
	selectedProvider := availableProviders[choice-1]
	logger.Info("User selected OAuth provider", "provider", selectedProvider.Name)
	return loginWithOIDCBrowser(selectedProvider.Name, deviceName)
}

// loginWithOIDCBrowser handles OIDC authorization code flow (opens browser)
func loginWithOIDCBrowser(provider, deviceName string) error {
	// Use hostname as device name if not provided
	if deviceName == "" {
		hostname, err := os.Hostname()
		if err != nil {
			deviceName = "cli-client"
		} else {
			deviceName = hostname
		}
	}

	// Connect to server
	grpcClient, err := NewUnauthenticatedClient()
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer grpcClient.Close()

	authClient := authpb.NewAuthServiceClient(grpcClient.Conn())

	// Get OAuth config from server
	oauthConfigResp, err := authClient.GetOAuthConfig(context.Background(), &authpb.GetOAuthConfigRequest{})
	if err != nil {
		return fmt.Errorf("failed to get OAuth config from server: %w", err)
	}

	// Find the requested provider
	var providerConfig *authpb.OAuthProvider
	for _, p := range oauthConfigResp.Providers {
		if p.Name == provider {
			providerConfig = p
			break
		}
	}

	if providerConfig == nil {
		return fmt.Errorf("provider %s not available. Available providers: %v", provider, getProviderNames(oauthConfigResp.Providers))
	}

	// Start local callback server on fixed port for OAuth
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	// Use fixed port 8085 for CLI OAuth callbacks (must be registered with Google)
	port := 8085
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return fmt.Errorf("failed to start callback server on port %d: %w (is another instance running?)", port, err)
	}

	// Start HTTP server for callback
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no authorization code received")
			http.Error(w, "Authorization failed", http.StatusBadRequest)
			return
		}

		// Check if we should also provide web UI link
		webURL := os.Getenv("HIVEMIND_WEB_URL")
		if webURL == "" {
			webURL = "http://localhost:8080"
		}

		// Define template for success page
		successTemplate := `<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Authentication Successful</title>
	<style>
		body {
			font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
			text-align: center;
			padding: 50px;
			background: #f5f5f5;
		}
		.container {
			max-width: 500px;
			margin: 0 auto;
			background: white;
			padding: 40px;
			border-radius: 12px;
			box-shadow: 0 2px 8px rgba(0,0,0,0.1);
		}
		h1 {
			color: #10b981;
			margin: 0 0 10px 0;
		}
		.checkmark {
			color: #10b981;
			font-size: 48px;
		}
		.message {
			color: #666;
			margin: 20px 0;
		}
		.web-ui-box {
			margin-top: 30px;
			padding: 20px;
			background: #f0f9ff;
			border-radius: 8px;
		}
		.web-ui-box p {
			margin: 0 0 15px 0;
			color: #333;
		}
		.web-ui-link {
			display: inline-block;
			padding: 12px 24px;
			background: #3b82f6;
			color: white;
			text-decoration: none;
			border-radius: 6px;
			font-weight: 500;
			transition: background 0.2s;
		}
		.web-ui-link:hover {
			background: #2563eb;
		}
		.note {
			margin: 15px 0 0 0;
			font-size: 12px;
			color: #666;
		}
	</style>
</head>
<body>
	<div class="container">
		<div class="checkmark">✓</div>
		<h1>Authentication Successful!</h1>
		<p class="message">CLI authentication complete. You can close this window and return to the terminal.</p>
		
		<div class="web-ui-box">
			<p>Want to use the web UI?</p>
			<a href="{{.WebURL}}/login" class="web-ui-link">
				Open Web UI →
			</a>
			<p class="note">(You'll need to sign in there separately)</p>
		</div>
	</div>
</body>
</html>`

		// Parse and execute template
		tmpl, err := template.New("success").Parse(successTemplate)
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data := struct {
			WebURL string
		}{
			WebURL: webURL,
		}

		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		codeChan <- code
	})

	server := &http.Server{Handler: mux}
	go func() {
		server.Serve(listener)
	}()
	defer server.Close()

	// Generate PKCE verifier and challenge
	verifier := generateCodeVerifier()
	challenge := generateCodeChallenge(verifier)
	state := generateCodeVerifier()[:16] // Use random state for CSRF protection

	// Check if provider has authorization URL
	if providerConfig.AuthorizationUrl == "" {
		return fmt.Errorf("provider %s does not have authorization URL configured", providerConfig.Name)
	}

	// Build authorization URL using template from server
	authURL := providerConfig.AuthorizationUrl
	authURL = strings.Replace(authURL, "{redirect_uri}", url.QueryEscape(redirectURI), 1)
	authURL = strings.Replace(authURL, "{state}", url.QueryEscape(state), 1)
	authURL = strings.Replace(authURL, "{code_challenge}", url.QueryEscape(challenge), 1)

	// Open browser
	fmt.Println("\n🔐 Opening browser for authentication...")
	fmt.Printf("If the browser doesn't open automatically, visit:\n%s\n\n", authURL)

	if err := openBrowser(authURL); err != nil {
		fmt.Printf("Failed to open browser automatically: %v\n", err)
	}

	fmt.Println("Waiting for authentication...")

	// Wait for callback or timeout
	var code string
	select {
	case code = <-codeChan:
		// Success
	case err := <-errChan:
		return err
	case <-time.After(5 * time.Minute):
		return fmt.Errorf("authentication timeout")
	}

	// Detect system timezone
	timezone := detectSystemTimezone()

	// Exchange code for token
	exchangeResp, err := authClient.ExchangeAuthCode(context.Background(), &authpb.ExchangeAuthCodeRequest{
		Provider:     provider,
		Code:         code,
		RedirectUri:  redirectURI,
		CodeVerifier: verifier,
		DeviceName:   deviceName,
		Timezone:     timezone,
		Scopes:       []string{"hivemind:read", "hivemind:write"},
	})
	if err != nil {
		return fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	// Store credentials
	creds := &Credentials{
		AccessToken: exchangeResp.ApiToken,
		TokenID:     exchangeResp.TokenId,
		UserID:      exchangeResp.User.UserId,
		Username:    exchangeResp.User.Email,
		ExpiresAt:   exchangeResp.ExpiresAt.AsTime(),
		Provider:    provider,
	}

	if err := SaveCredentials(creds); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	fmt.Println("\n✓ Successfully authenticated!")
	fmt.Printf("  Logged in as: %s\n", exchangeResp.User.Email)
	fmt.Printf("  Token expires: %s\n", exchangeResp.ExpiresAt.AsTime().Format("2006-01-02 15:04:05"))

	return nil
}

// Helper functions for PKCE
func generateCodeVerifier() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func generateCodeChallenge(verifier string) string {
	h := sha256.New()
	h.Write([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

// openBrowser tries to open the URL in a browser
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}

// getProviderNames extracts provider names from OAuthProvider list
func getProviderNames(providers []*authpb.OAuthProvider) []string {
	names := make([]string, len(providers))
	for i, p := range providers {
		names[i] = p.Name
	}
	return names
}

func newAuthLogoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Logout from the Hivemind server",
		Long:  `Remove stored credentials and optionally revoke the token on the server`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load credentials
			creds, err := LoadCredentials()
			if err != nil {
				return fmt.Errorf("not logged in: %w", err)
			}

			// Try to revoke token on server
			config, _ := LoadConfig()
			if config != nil {
				serverAddress, err := config.ServerAddress()
				if err == nil {
					conn, err := grpc.Dial(
						serverAddress,
						grpc.WithTransportCredentials(insecure.NewCredentials()),
					)
					if err == nil {
						defer conn.Close()
						authClient := authpb.NewAuthServiceClient(conn)
						_, _ = authClient.RevokeToken(context.Background(), &authpb.RevokeTokenRequest{
							TokenId: creds.TokenID,
						})
					}
				}
			}

			// Remove credentials file
			if err := RemoveCredentials(); err != nil {
				return fmt.Errorf("failed to remove credentials: %w", err)
			}

			fmt.Println("✓ Successfully logged out")
			return nil
		},
	}
}

func newAuthStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds, err := LoadCredentials()
			if err != nil {
				fmt.Println("Not logged in")
				return nil
			}

			fmt.Printf("Logged in as: %s\n", creds.Username)
			fmt.Printf("User ID: %s\n", creds.UserID)

			// Show expiry in local timezone
			localExpiry := creds.ExpiresAt.Local()
			fmt.Printf("Token expires: %s\n", localExpiry.Format("2006-01-02 15:04:05 MST"))

			// Calculate and show time until expiration
			now := time.Now()
			if creds.IsExpired() {
				duration := now.Sub(creds.ExpiresAt)
				fmt.Printf("⚠  Token expired %s ago - automatic refresh will be attempted on next request\n", formatDuration(duration))
			} else {
				duration := creds.ExpiresAt.Sub(now)
				fmt.Printf("✓  Valid for %s\n", formatDuration(duration))
			}

			return nil
		},
	}
}

func newAuthTokenCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "token",
		Short: "Display the current access token",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds, err := LoadCredentials()
			if err != nil {
				return fmt.Errorf("not logged in: %w", err)
			}

			fmt.Println(creds.AccessToken)
			return nil
		},
	}
}

func promptCredentials() (username, password string, err error) {
	// Get username
	fmt.Print("Username: ")
	_, err = fmt.Scanln(&username)
	if err != nil {
		return "", "", fmt.Errorf("failed to read username: %w", err)
	}

	// Get password (hidden)
	fmt.Print("Password: ")
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // newline after password input
	if err != nil {
		return "", "", fmt.Errorf("failed to read password: %w", err)
	}

	password = string(passwordBytes)
	return username, password, nil
}

// detectSystemTimezone attempts to detect the system's timezone
// possible improvement: detect time difference between client and server and adjust accordingly.
func detectSystemTimezone() string {
	// Try to get timezone from environment variable first
	if tz := os.Getenv("TZ"); tz != "" {
		return tz
	}

	// Get the current time and location
	now := time.Now()
	zone, _ := now.Zone()

	// Try to get the IANA timezone name
	if loc := now.Location(); loc != nil && loc.String() != "Local" {
		return loc.String()
	}

	// Fallback to zone abbreviation (not ideal but better than nothing)
	if zone != "" {
		return zone
	}

	// Ultimate fallback
	return "UTC"
}

// getTodayInLocalTimezone returns today's date in the local system timezone
func getTodayInLocalTimezone() string {
	timezone := detectSystemTimezone()
	if timezone == "" {
		// Fallback to local time
		return time.Now().Format("2006-01-02")
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		// If timezone is invalid, fallback to local time
		return time.Now().Format("2006-01-02")
	}

	return time.Now().In(loc).Format("2006-01-02")
}
