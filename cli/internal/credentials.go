package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devilmonastery/hivemind/internal/client"
)

// Credentials stores the authentication credentials
type Credentials struct {
	AccessToken string    `json:"access_token"`
	TokenID     string    `json:"token_id"`
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	ExpiresAt   time.Time `json:"expires_at"`
	Provider    string    `json:"provider,omitempty"` // OIDC provider (e.g., "google", "github")
	// Note: OAuth refresh tokens are stored server-side for security
}

// IsExpired checks if the token is expired
func (c *Credentials) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// NeedsRefresh checks if the token needs to be refreshed soon (within 5 minutes)
func (c *Credentials) NeedsRefresh() bool {
	return time.Now().Add(5 * time.Minute).After(c.ExpiresAt)
}

// NewFileCredentials creates a new file-based credential manager that implements TokenManager
func NewFileCredentials() client.TokenManager {
	return &FileCredentials{}
}

// FileCredentials implements TokenManager using file-based credential storage
type FileCredentials struct{}

// GetToken returns the current access token from file
func (f *FileCredentials) GetToken() (string, error) {
	creds, err := LoadCredentials()
	if err != nil {
		log.Printf("[CLI-TOKEN] Failed to load credentials: %v", err)
		return "", err
	}
	preview := creds.AccessToken
	if len(preview) > 30 {
		preview = preview[:30] + "..."
	}
	log.Printf("[CLI-TOKEN] GetToken returning: %s (TokenID: %s)", preview, creds.TokenID)
	return creds.AccessToken, nil
}

// GetTokenID returns the token ID from file
func (f *FileCredentials) GetTokenID() (string, error) {
	creds, err := LoadCredentials()
	if err != nil {
		return "", err
	}
	return creds.TokenID, nil
}

// SaveToken saves the token and token ID to file
func (f *FileCredentials) SaveToken(token, tokenID string) error {
	preview := token
	if len(preview) > 30 {
		preview = preview[:30] + "..."
	}
	log.Printf("[CLI-TOKEN] SaveToken called: %s (TokenID: %s)", preview, tokenID)

	creds, err := LoadCredentials()
	if err != nil {
		// If load fails, create new credentials
		log.Printf("[CLI-TOKEN] Creating new credentials (load failed: %v)", err)
		creds = &Credentials{}
	}

	creds.AccessToken = token
	creds.TokenID = tokenID

	// Decode JWT to extract expiry
	expiresAt, decodeErr := extractJWTExpiry(token)
	if decodeErr != nil {
		log.Printf("[CLI-TOKEN] Warning: Failed to decode JWT expiry: %v", decodeErr)
	} else {
		creds.ExpiresAt = expiresAt
		log.Printf("[CLI-TOKEN] Extracted expiry from JWT: %v", expiresAt)
	}

	err = SaveCredentials(creds)
	if err != nil {
		log.Printf("[CLI-TOKEN] Failed to save credentials: %v", err)
		return err
	}
	log.Printf("[CLI-TOKEN] Credentials saved successfully")
	return nil
}

// ClearToken removes the credentials file
func (f *FileCredentials) ClearToken() error {
	return RemoveCredentials()
}

// extractJWTExpiry decodes a JWT and extracts the expiration time
func extractJWTExpiry(token string) (time.Time, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("invalid JWT format")
	}

	// Decode the payload (second part)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, err
	}

	// Parse the JSON payload
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return time.Time{}, err
	}

	// Extract exp claim
	exp, ok := claims["exp"].(float64)
	if !ok {
		return time.Time{}, fmt.Errorf("exp claim not found or invalid")
	}

	return time.Unix(int64(exp), 0), nil
}

// credentialsPath returns the path to the credentials file for the current context
func credentialsPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	// Load config to get current context
	config, err := LoadConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}

	// Use context-specific credentials file
	configDir := filepath.Join(homeDir, ".config", "hivemind")
	filename := fmt.Sprintf("credentials-%s.json", config.CurrentContext)
	return filepath.Join(configDir, filename), nil
}

// SaveCredentials saves credentials to disk
func SaveCredentials(creds *Credentials) error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal credentials
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	// Write with restricted permissions (read/write for owner only)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write credentials: %w", err)
	}

	return nil
}

// LoadCredentials loads credentials from disk
func LoadCredentials() (*Credentials, error) {
	path, err := credentialsPath()
	if err != nil {
		return nil, err
	}
	log.Printf("[CLI-CREDS] Loading credentials from: %s", path)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not logged in")
		}
		return nil, fmt.Errorf("failed to read credentials: %w", err)
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}

	return &creds, nil
}

// RemoveCredentials removes the credentials file
func RemoveCredentials() error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove credentials: %w", err)
	}

	return nil
}
