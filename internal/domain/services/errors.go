package services

import (
	"errors"

	"github.com/devilmonastery/hivemind/internal/domain/repositories"
)

// GetUserLookupFailureReason returns a human-readable reason string for user lookup failures.
// This is used for audit logging and error reporting in the service layer.
func GetUserLookupFailureReason(err error) string {
	if errors.Is(err, repositories.ErrUserNotFound) {
		return "user_not_found"
	}
	if errors.Is(err, repositories.ErrUserInactive) {
		return "user_inactive"
	}
	return "user_lookup_failed"
}

// IsUserInactive checks if the error indicates an inactive user.
func IsUserInactive(err error) bool {
	return errors.Is(err, repositories.ErrUserInactive)
}

// IsUserNotFound checks if the error indicates user not found.
func IsUserNotFound(err error) bool {
	return errors.Is(err, repositories.ErrUserNotFound)
}
