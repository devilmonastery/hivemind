package entities

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User represents a user in the system
type User struct {
	ID           string     `json:"id" db:"id"`
	Email        string     `json:"email" db:"email"`
	DisplayName  string     `json:"display_name" db:"name"`               // db column is 'name'
	AvatarURL    *string    `json:"avatar_url,omitempty" db:"avatar_url"` // profile picture from most recent identity
	Timezone     *string    `json:"timezone,omitempty" db:"timezone"`     // user's preferred timezone (IANA Time Zone)
	PasswordHash *string    `json:"-" db:"password_hash"`                 // never serialize to JSON
	Role         Role       `json:"role" db:"role"`
	UserType     UserType   `json:"user_type" db:"user_type"`
	IsActive     bool       `json:"is_active" db:"disabled"`                      // db column is 'disabled' (inverted)
	OIDCSubject  *string    `json:"oidc_subject,omitempty" db:"provider_user_id"` // db column is 'provider_user_id'
	OIDCProvider *string    `json:"oidc_provider,omitempty" db:"provider"`        // db column is 'provider'
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
	LastLogin    *time.Time `json:"last_login,omitempty" db:"last_seen"` // db column is 'last_seen'
}

// Role represents user roles in the system
type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

// UserType represents the type of user account
type UserType string

const (
	UserTypeOIDC   UserType = "oidc"
	UserTypeLocal  UserType = "local"
	UserTypeSystem UserType = "system"
)

// HasRole checks if the user has a specific role
func (u *User) HasRole(role Role) bool {
	return u.Role == role
}

// IsAdmin returns true if the user is an admin
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// Active returns true if the user is active
func (u *User) Active() bool {
	return u.IsActive
}

// IsLocalUser returns true if the user is a local user (not OIDC)
func (u *User) IsLocalUser() bool {
	return u.UserType == UserTypeLocal
}

// IsOIDCUser returns true if the user is an OIDC user
func (u *User) IsOIDCUser() bool {
	return u.UserType == UserTypeOIDC
}

// VerifyPassword checks if the provided password matches the hashed password
func (u *User) VerifyPassword(password string) bool {
	if u.PasswordHash == nil {
		return false
	}
	err := bcrypt.CompareHashAndPassword([]byte(*u.PasswordHash), []byte(password))
	return err == nil
}
