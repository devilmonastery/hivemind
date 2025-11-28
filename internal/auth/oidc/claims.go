package oidc

import "time"

// Claims represents the standardized claims extracted from an OIDC ID token
type Claims struct {
	// Subject - unique identifier for the user at the provider
	Subject string

	// Email address of the user
	Email string

	// EmailVerified indicates if the email has been verified by the provider
	EmailVerified bool

	// Name is the user's full display name
	Name string

	// Picture is the URL to the user's profile picture
	Picture string

	// Issuer is the OIDC provider that issued the token (e.g., "https://accounts.google.com")
	Issuer string

	// Audience is the client ID the token was issued for
	Audience string

	// IssuedAt is when the token was issued
	IssuedAt time.Time

	// ExpiresAt is when the token expires
	ExpiresAt time.Time

	// HostedDomain is the G Suite domain (Google-specific, optional)
	HostedDomain string

	// Organization is the GitHub organization (GitHub-specific, optional)
	Organization string
}

// IsExpired checks if the token has expired
func (c *Claims) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// IsValid performs basic validation of required claims
func (c *Claims) IsValid() bool {
	if c.Subject == "" {
		return false
	}
	if c.Email == "" {
		return false
	}
	if c.Issuer == "" {
		return false
	}
	if c.Audience == "" {
		return false
	}
	if c.IsExpired() {
		return false
	}
	return true
}
