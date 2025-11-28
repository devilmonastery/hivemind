package repositories

import (
	"context"
	"time"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
)

// TokenRepository defines the interface for API token data access
type TokenRepository interface {
	// Create a new API token
	Create(ctx context.Context, token *entities.APIToken) error

	// GetByID retrieves a token by its ID
	GetByID(ctx context.Context, id string) (*entities.APIToken, error)

	// GetByTokenHash retrieves a token by its hash (for authentication)
	GetByTokenHash(ctx context.Context, tokenHash string) (*entities.APIToken, error)

	// Update an existing token (for last_used, revoked_at updates)
	Update(ctx context.Context, token *entities.APIToken) error

	// Revoke a token by ID
	Revoke(ctx context.Context, tokenID string) error

	// RevokeAllForUser revokes all tokens for a user
	RevokeAllForUser(ctx context.Context, userID string) error

	// List tokens for a user with pagination
	ListByUser(ctx context.Context, userID string, opts ListTokensOptions) ([]*entities.APIToken, int64, error)

	// List all tokens with pagination and filtering (admin only)
	List(ctx context.Context, opts ListTokensOptions) ([]*entities.APIToken, int64, error)

	// Delete expired tokens (cleanup job)
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)

	// Delete revoked tokens older than specified time (cleanup job)
	DeleteRevokedBefore(ctx context.Context, before time.Time) (int64, error)

	// UpdateLastUsed updates the last_used timestamp for a token
	UpdateLastUsed(ctx context.Context, tokenID string, lastUsed time.Time) error

	// Count active tokens for a user
	CountActiveByUser(ctx context.Context, userID string) (int64, error)
}

// ListTokensOptions provides filtering and pagination options for listing tokens
type ListTokensOptions struct {
	// Pagination
	Limit  int
	Offset int

	// Filtering
	UserID        *string    // filter by user ID
	DeviceName    *string    // filter by device name
	IsRevoked     *bool      // filter by revoked status
	IsExpired     *bool      // filter by expired status
	HasScopes     []string   // filter tokens that have any of these scopes
	CreatedAfter  *time.Time // filter by creation date
	CreatedBefore *time.Time // filter by creation date

	// Sorting
	SortBy    string // field to sort by (created_at, last_used, expires_at, device_name)
	SortOrder string // asc or desc
}
