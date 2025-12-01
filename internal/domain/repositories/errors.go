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

	// ErrDiscordUserNotFound is returned when a Discord user mapping cannot be found
	ErrDiscordUserNotFound = errors.New("discord user not found")

	// ErrDiscordGuildNotFound is returned when a Discord guild cannot be found
	ErrDiscordGuildNotFound = errors.New("discord guild not found")

	// ErrGuildMemberNotFound is returned when a guild member record cannot be found
	ErrGuildMemberNotFound = errors.New("guild member not found")

	// ErrAuditLogNotFound is returned when an audit log cannot be found
	ErrAuditLogNotFound = errors.New("audit log not found")
)
