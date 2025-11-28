package entities

import "time"

// OIDCSession represents an OIDC authentication session
type OIDCSession struct {
	ID            string     `json:"id" db:"id"`
	UserID        *string    `json:"user_id,omitempty" db:"user_id"` // user this session belongs to
	Provider      string     `json:"provider" db:"provider"`         // "google", "github", etc.
	State         string     `json:"state" db:"state"`               // for web flow
	Nonce         string     `json:"nonce" db:"nonce"`               // for web flow
	CodeVerifier  string     `json:"-" db:"code_verifier"`           // PKCE for web flow, never serialize to JSON
	RedirectURI   string     `json:"redirect_uri" db:"redirect_uri"` // for web flow
	Scopes        []string   `json:"scopes" db:"scopes"`             // stored as JSON in DB
	ExpiresAt     time.Time  `json:"expires_at" db:"expires_at"`     // when refresh token expires
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	LastRefreshed *time.Time `json:"last_refreshed,omitempty" db:"last_refreshed"` // last time we used the refresh token
	CompletedAt   *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	IDToken       *string    `json:"-" db:"id_token"`      // never serialize to JSON
	AccessToken   *string    `json:"-" db:"access_token"`  // never serialize to JSON
	RefreshToken  *string    `json:"-" db:"refresh_token"` // OAuth refresh token, never serialize to JSON
}

// IsExpired returns true if the OIDC session has expired
func (o *OIDCSession) IsExpired() bool {
	return time.Now().After(o.ExpiresAt)
}

// IsComplete returns true if the OIDC flow is complete
func (o *OIDCSession) IsComplete() bool {
	return o.UserID != nil && o.CompletedAt != nil
}

// Complete marks the session as complete with tokens
func (o *OIDCSession) Complete(userID string, idToken, accessToken, refreshToken *string) {
	o.UserID = &userID
	now := time.Now()
	o.CompletedAt = &now
	o.IDToken = idToken
	o.AccessToken = accessToken
	o.RefreshToken = refreshToken
}
