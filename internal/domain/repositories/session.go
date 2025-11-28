package repositories

import (
	"context"
	"time"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
)

// SessionRepository defines the interface for authentication session data access
type SessionRepository interface {
	// OIDC Session methods
	CreateOIDCSession(ctx context.Context, session *entities.OIDCSession) error
	GetOIDCSessionByID(ctx context.Context, id string) (*entities.OIDCSession, error)
	GetOIDCSessionByState(ctx context.Context, state string) (*entities.OIDCSession, error)
	GetOIDCSessionByUserAndProvider(ctx context.Context, userID, provider string) (*entities.OIDCSession, error)
	UpdateOIDCSession(ctx context.Context, session *entities.OIDCSession) error
	DeleteOIDCSession(ctx context.Context, id string) error
	DeleteExpiredOIDCSessions(ctx context.Context, before time.Time) (int64, error)
	ListOIDCSessionsByUser(ctx context.Context, userID string, opts ListSessionsOptions) ([]*entities.OIDCSession, int64, error)

	// Cleanup methods for expired sessions
	CleanupExpiredSessions(ctx context.Context, before time.Time) (oidcDeleted int64, err error)
}

// ListSessionsOptions provides filtering and pagination options for listing sessions
type ListSessionsOptions struct {
	// Pagination
	Limit  int
	Offset int

	// Filtering
	UserID        *string    // filter by user ID
	IsExpired     *bool      // filter by expired status
	IsComplete    *bool      // filter by completion status
	CreatedAfter  *time.Time // filter by creation date
	CreatedBefore *time.Time // filter by creation date

	// Sorting
	SortBy    string // field to sort by (created_at, expires_at, authorized_at, completed_at)
	SortOrder string // asc or desc
}
