package repositories

import (
	"context"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
)

// IdentityRepository defines the interface for user identity data access
// Supports multi-provider authentication where users can link multiple
// OIDC providers (Google, GitHub, etc.) to a single account
type IdentityRepository interface {
	// Create creates a new identity for a user
	Create(ctx context.Context, identity *entities.Identity) error

	// GetByProviderAndExternalID retrieves an identity by provider and external ID
	// This is the primary lookup during login to find existing identities
	GetByProviderAndExternalID(ctx context.Context, provider, externalID string) (*entities.Identity, error)

	// ListByUserID retrieves all identities linked to a user
	// Used to show user which providers they've linked
	ListByUserID(ctx context.Context, userID string) ([]*entities.Identity, error)

	// Update updates an existing identity (e.g., last_login_at, display_name)
	Update(ctx context.Context, identity *entities.Identity) error

	// Delete removes an identity link
	// Users can unlink providers (must keep at least one)
	Delete(ctx context.Context, identityID string) error

	// CountByUserID counts how many identities a user has linked
	// Useful to prevent unlinking the last identity
	CountByUserID(ctx context.Context, userID string) (int, error)
}
