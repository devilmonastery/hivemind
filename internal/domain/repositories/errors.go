package repositories

import "errors"

// Domain-specific repository errors
var (
	// ErrUserNotFound is returned when a user cannot be found
	ErrUserNotFound = errors.New("user not found")

	// ErrUserInactive is returned when a user exists but is inactive/disabled
	ErrUserInactive = errors.New("user is inactive")

	// ErrTokenNotFound is returned when a token cannot be found
	ErrTokenNotFound = errors.New("token not found")

	// ErrSessionNotFound is returned when a session cannot be found
	ErrSessionNotFound = errors.New("session not found")

	// ErrSnippetNotFound is returned when a snippet cannot be found
	ErrSnippetNotFound = errors.New("snippet not found")

	// ErrAuditLogNotFound is returned when an audit log cannot be found
	ErrAuditLogNotFound = errors.New("audit log not found")
)
