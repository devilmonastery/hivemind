package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
	"github.com/devilmonastery/hivemind/internal/pkg/idgen"
)

// IdentityRepository implements the IdentityRepository interface for PostgreSQL
type IdentityRepository struct {
	db *sqlx.DB
}

// NewIdentityRepository creates a new PostgreSQL identity repository
func NewIdentityRepository(db *sqlx.DB) *IdentityRepository {
	return &IdentityRepository{db: db}
}

// Create creates a new identity
func (r *IdentityRepository) Create(ctx context.Context, identity *entities.Identity) error {
	// Generate ID if not set
	if identity.IdentityID == "" {
		identity.IdentityID = idgen.GenerateID()
	}

	// Set timestamps
	if identity.CreatedAt.IsZero() {
		identity.CreatedAt = time.Now()
	}

	query := `
		INSERT INTO user_identities (
			identity_id, user_id, provider, external_id, email, email_verified,
			display_name, profile_picture_url, created_at, last_login_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.db.ExecContext(ctx, query,
		identity.IdentityID,
		identity.UserID,
		identity.Provider,
		identity.ExternalID,
		identity.Email,
		identity.EmailVerified,
		identity.DisplayName,
		identity.ProfilePictureURL,
		identity.CreatedAt,
		identity.LastLoginAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create identity: %w", err)
	}

	return nil
}

// GetByProviderAndExternalID retrieves an identity by provider and external ID
func (r *IdentityRepository) GetByProviderAndExternalID(ctx context.Context, provider, externalID string) (*entities.Identity, error) {
	var identity entities.Identity
	query := `
		SELECT identity_id, user_id, provider, external_id, email, email_verified,
		       display_name, profile_picture_url, created_at, last_login_at
		FROM user_identities
		WHERE provider = $1 AND external_id = $2
		LIMIT 1
	`

	err := r.db.GetContext(ctx, &identity, query, provider, externalID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get identity by provider and external ID: %w", err)
	}

	return &identity, nil
}

// ListByUserID retrieves all identities for a user
func (r *IdentityRepository) ListByUserID(ctx context.Context, userID string) ([]*entities.Identity, error) {
	var identities []*entities.Identity
	query := `
		SELECT identity_id, user_id, provider, external_id, email, email_verified,
		       display_name, profile_picture_url, created_at, last_login_at
		FROM user_identities
		WHERE user_id = $1
		ORDER BY created_at ASC
	`

	err := r.db.SelectContext(ctx, &identities, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list identities by user ID: %w", err)
	}

	return identities, nil
}

// Update updates an existing identity
func (r *IdentityRepository) Update(ctx context.Context, identity *entities.Identity) error {
	query := `
		UPDATE user_identities
		SET email = $1,
		    email_verified = $2,
		    display_name = $3,
		    profile_picture_url = $4,
		    last_login_at = $5
		WHERE identity_id = $6
	`

	result, err := r.db.ExecContext(ctx, query,
		identity.Email,
		identity.EmailVerified,
		identity.DisplayName,
		identity.ProfilePictureURL,
		identity.LastLoginAt,
		identity.IdentityID,
	)
	if err != nil {
		return fmt.Errorf("failed to update identity: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("identity not found: %s", identity.IdentityID)
	}

	return nil
}

// Delete deletes an identity
func (r *IdentityRepository) Delete(ctx context.Context, identityID string) error {
	query := `DELETE FROM user_identities WHERE identity_id = $1`

	result, err := r.db.ExecContext(ctx, query, identityID)
	if err != nil {
		return fmt.Errorf("failed to delete identity: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("identity not found: %s", identityID)
	}

	return nil
}

// CountByUserID counts how many identities a user has
func (r *IdentityRepository) CountByUserID(ctx context.Context, userID string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM user_identities WHERE user_id = $1`

	err := r.db.GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to count identities: %w", err)
	}

	return count, nil
}

// Ensure IdentityRepository implements repositories.IdentityRepository
var _ repositories.IdentityRepository = (*IdentityRepository)(nil)
