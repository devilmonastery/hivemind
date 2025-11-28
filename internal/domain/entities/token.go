package entities

import (
	"encoding/json"
	"time"
)

// APIToken represents an API token for programmatic access
type APIToken struct {
	ID         string     `json:"id" db:"id"`
	UserID     string     `json:"user_id" db:"user_id"`
	TokenHash  string     `json:"-" db:"token_hash"` // never serialize to JSON
	DeviceName string     `json:"device_name" db:"device_name"`
	Scopes     []string   `json:"scopes" db:"scopes"` // stored as JSON in DB
	ExpiresAt  time.Time  `json:"expires_at" db:"expires_at"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
	LastUsed   *time.Time `json:"last_used" db:"last_used"`
	RevokedAt  *time.Time `json:"revoked_at" db:"revoked_at"`
}

// TokenScope represents available scopes for API tokens
type TokenScope string

const (
	ScopeSnippetsRead  TokenScope = "hivemind:read"
	ScopeSnippetsWrite TokenScope = "hivemind:write"
	ScopeTokensRead    TokenScope = "tokens:read"
	ScopeTokensWrite   TokenScope = "tokens:write"
	ScopeUsersRead     TokenScope = "users:read"
	ScopeUsersWrite    TokenScope = "users:write"
	ScopeAdminAll      TokenScope = "admin:*"
)

// DefaultUserScopes returns the default scopes for regular users
func DefaultUserScopes() []string {
	return []string{
		string(ScopeSnippetsRead),
		string(ScopeSnippetsWrite),
		string(ScopeTokensRead),
		string(ScopeTokensWrite),
	}
}

// AdminScopes returns all available scopes for admin users
func AdminScopes() []string {
	return []string{
		string(ScopeSnippetsRead),
		string(ScopeSnippetsWrite),
		string(ScopeTokensRead),
		string(ScopeTokensWrite),
		string(ScopeUsersRead),
		string(ScopeUsersWrite),
		string(ScopeAdminAll),
	}
}

// IsRevoked returns true if the token has been revoked
func (t *APIToken) IsRevoked() bool {
	return t.RevokedAt != nil
}

// IsExpired returns true if the token has expired
func (t *APIToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsValid returns true if the token is not revoked and not expired
func (t *APIToken) IsValid() bool {
	return !t.IsRevoked() && !t.IsExpired()
}

// HasScope checks if the token has a specific scope
func (t *APIToken) HasScope(scope TokenScope) bool {
	scopeStr := string(scope)
	for _, s := range t.Scopes {
		if s == scopeStr || s == string(ScopeAdminAll) {
			return true
		}
	}
	return false
}

// UpdateLastUsed updates the last used timestamp to now
func (t *APIToken) UpdateLastUsed() {
	now := time.Now()
	t.LastUsed = &now
}

// Revoke marks the token as revoked
func (t *APIToken) Revoke() {
	now := time.Now()
	t.RevokedAt = &now
}

// MarshalScopesToJSON converts scopes slice to JSON string for database storage
func (t *APIToken) MarshalScopesToJSON() (string, error) {
	data, err := json.Marshal(t.Scopes)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// UnmarshalScopesFromJSON converts JSON string from database to scopes slice
func (t *APIToken) UnmarshalScopesFromJSON(data string) error {
	return json.Unmarshal([]byte(data), &t.Scopes)
}
