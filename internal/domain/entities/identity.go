package entities

import "time"

// Identity represents a linked OIDC provider identity for a user
// Users can have multiple identities (e.g., Google + GitHub) linked to one account
type Identity struct {
	IdentityID        string     `json:"identity_id" db:"identity_id"`
	UserID            string     `json:"user_id" db:"user_id"`
	Provider          string     `json:"provider" db:"provider"`                                 // "google", "github", "okta", etc.
	ExternalID        string     `json:"external_id" db:"external_id"`                           // Provider's 'sub' claim
	Email             string     `json:"email" db:"email"`                                       // Email from this provider
	EmailVerified     bool       `json:"email_verified" db:"email_verified"`                     // Whether provider verified email
	DisplayName       string     `json:"display_name,omitempty" db:"display_name"`               // Name from provider
	ProfilePictureURL string     `json:"profile_picture_url,omitempty" db:"profile_picture_url"` // Profile pic from provider
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	LastLoginAt       *time.Time `json:"last_login_at,omitempty" db:"last_login_at"` // Last time this identity was used
}

// Provider returns a formatted provider+external_id string for logging
func (i *Identity) ProviderKey() string {
	return i.Provider + ":" + i.ExternalID
}
