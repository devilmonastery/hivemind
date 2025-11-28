package entities

import (
	"encoding/json"
	"time"
)

// AuditLog represents a security audit log entry
type AuditLog struct {
	ID         string         `json:"id" db:"id"`
	UserID     *string        `json:"user_id,omitempty" db:"user_id"` // null for system events
	Action     AuditAction    `json:"action" db:"action"`
	Resource   AuditResource  `json:"resource" db:"resource"`
	ResourceID *string        `json:"resource_id,omitempty" db:"resource_id"`
	IPAddress  *string        `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent  *string        `json:"user_agent,omitempty" db:"user_agent"`
	Metadata   map[string]any `json:"metadata,omitempty" db:"metadata"` // stored as JSON in DB
	Success    bool           `json:"success" db:"success"`
	ErrorMsg   *string        `json:"error_message,omitempty" db:"error_message"`
	CreatedAt  time.Time      `json:"created_at" db:"created_at"`
}

// AuditAction represents the type of action being audited
type AuditAction string

const (
	// Authentication actions
	ActionUserLogin       AuditAction = "user.login"
	ActionUserLogout      AuditAction = "user.logout"
	ActionUserLoginFailed AuditAction = "user.login_failed"
	ActionUserCreated     AuditAction = "user.created"
	ActionUserUpdated     AuditAction = "user.updated"
	ActionUserDeleted     AuditAction = "user.deleted"
	ActionUserRoleChanged AuditAction = "user.role_changed"

	// Token actions
	ActionTokenCreated AuditAction = "token.created"
	ActionTokenUsed    AuditAction = "token.used"
	ActionTokenRevoked AuditAction = "token.revoked"
	ActionTokenExpired AuditAction = "token.expired"

	// OIDC actions
	ActionOIDCStart    AuditAction = "oidc.started"
	ActionOIDCCallback AuditAction = "oidc.callback"
	ActionOIDCComplete AuditAction = "oidc.completed"
	ActionOIDCFailed   AuditAction = "oidc.failed"

	// Snippet actions
	ActionSnippetCreated AuditAction = "snippet.created"
	ActionSnippetRead    AuditAction = "snippet.read"
	ActionSnippetUpdated AuditAction = "snippet.updated"
	ActionSnippetDeleted AuditAction = "snippet.deleted"

	// System actions
	ActionSystemStartup  AuditAction = "system.startup"
	ActionSystemShutdown AuditAction = "system.shutdown"
	ActionConfigChanged  AuditAction = "system.config_changed"
)

// AuditResource represents the type of resource being acted upon
type AuditResource string

const (
	ResourceUser        AuditResource = "user"
	ResourceToken       AuditResource = "token"
	ResourceOIDCSession AuditResource = "oidc_session"
	ResourceSnippet     AuditResource = "snippet"
	ResourceSystem      AuditResource = "system"
)

// NewAuditLog creates a new audit log entry
func NewAuditLog(userID *string, action AuditAction, resource AuditResource) *AuditLog {
	return &AuditLog{
		UserID:    userID,
		Action:    action,
		Resource:  resource,
		Success:   true,
		CreatedAt: time.Now(),
		Metadata:  make(map[string]any),
	}
}

// WithResourceID sets the resource ID
func (a *AuditLog) WithResourceID(resourceID string) *AuditLog {
	a.ResourceID = &resourceID
	return a
}

// WithIPAddress sets the IP address
func (a *AuditLog) WithIPAddress(ip string) *AuditLog {
	a.IPAddress = &ip
	return a
}

// WithUserAgent sets the user agent
func (a *AuditLog) WithUserAgent(userAgent string) *AuditLog {
	a.UserAgent = &userAgent
	return a
}

// WithError marks the audit log as failed with an error message
func (a *AuditLog) WithError(err error) *AuditLog {
	a.Success = false
	msg := err.Error()
	a.ErrorMsg = &msg
	return a
}

// WithMetadata adds metadata to the audit log
func (a *AuditLog) WithMetadata(key string, value any) *AuditLog {
	if a.Metadata == nil {
		a.Metadata = make(map[string]any)
	}
	a.Metadata[key] = value
	return a
}

// WithAllMetadata sets all metadata at once
func (a *AuditLog) WithAllMetadata(metadata map[string]any) *AuditLog {
	a.Metadata = metadata
	return a
}

// MarshalMetadataToJSON converts metadata map to JSON string for database storage
func (a *AuditLog) MarshalMetadataToJSON() (string, error) {
	if a.Metadata == nil {
		return "{}", nil
	}
	data, err := json.Marshal(a.Metadata)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// UnmarshalMetadataFromJSON converts JSON string from database to metadata map
func (a *AuditLog) UnmarshalMetadataFromJSON(data string) error {
	if data == "" || data == "{}" {
		a.Metadata = make(map[string]any)
		return nil
	}
	return json.Unmarshal([]byte(data), &a.Metadata)
}

// IsAuthentication returns true if this is an authentication-related action
func (a *AuditLog) IsAuthentication() bool {
	switch a.Action {
	case ActionUserLogin, ActionUserLogout, ActionUserLoginFailed,
		ActionOIDCStart, ActionOIDCCallback, ActionOIDCComplete, ActionOIDCFailed:
		return true
	default:
		return false
	}
}

// IsTokenAction returns true if this is a token-related action
func (a *AuditLog) IsTokenAction() bool {
	switch a.Action {
	case ActionTokenCreated, ActionTokenUsed, ActionTokenRevoked, ActionTokenExpired:
		return true
	default:
		return false
	}
}

// IsUserAction returns true if this action was performed by a user (not system)
func (a *AuditLog) IsUserAction() bool {
	return a.UserID != nil
}
