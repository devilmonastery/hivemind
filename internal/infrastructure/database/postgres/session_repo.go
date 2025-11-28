package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
	"github.com/devilmonastery/hivemind/internal/pkg/idgen"
)

// SessionRepository implements the SessionRepository interface for SQLite
type SessionRepository struct {
	db *sqlx.DB
}

// NewSessionRepository creates a new SQLite session repository
func NewSessionRepository(db *sqlx.DB) repositories.SessionRepository {
	return &SessionRepository{
		db: db,
	}
}

// oidcSessionRow represents an OIDC session as stored in the database
// Schema: id (PK), user_id, provider, refresh_token_hash, expires_at, created_at, last_refreshed
type oidcSessionRow struct {
	ID               string         `db:"id"` // PK
	UserID           string         `db:"user_id"`
	Provider         string         `db:"provider"`
	RefreshTokenHash sql.NullString `db:"refresh_token_hash"`
	ExpiresAt        sql.NullTime   `db:"expires_at"`
	CreatedAt        time.Time      `db:"created_at"`
	LastRefreshed    sql.NullTime   `db:"last_refreshed"`
}

// toEntity converts an oidcSessionRow to a domain entity
func (r *oidcSessionRow) toEntity() (*entities.OIDCSession, error) {
	session := &entities.OIDCSession{
		ID:        r.ID,
		UserID:    &r.UserID,
		Provider:  r.Provider,
		CreatedAt: r.CreatedAt,
	}

	// Handle nullable fields
	if r.ExpiresAt.Valid {
		session.ExpiresAt = r.ExpiresAt.Time
	}

	if r.LastRefreshed.Valid {
		session.LastRefreshed = &r.LastRefreshed.Time
	}

	// Handle refresh token (stored in refresh_token_hash field for now)
	// TODO: Add proper encryption/decryption
	if r.RefreshTokenHash.Valid && r.RefreshTokenHash.String != "" {
		session.RefreshToken = &r.RefreshTokenHash.String
	}

	return session, nil
}

// oidcSessionRowFromEntity converts a domain entity to an oidcSessionRow
func oidcSessionRowFromEntity(session *entities.OIDCSession) (*oidcSessionRow, error) {
	row := &oidcSessionRow{
		ID:        session.ID,
		Provider:  session.Provider,
		CreatedAt: session.CreatedAt,
	}

	// Handle user ID
	if session.UserID != nil {
		row.UserID = *session.UserID
	}

	// Handle expiration
	if !session.ExpiresAt.IsZero() {
		row.ExpiresAt = sql.NullTime{Time: session.ExpiresAt, Valid: true}
	}

	// Handle last refreshed
	if session.LastRefreshed != nil {
		row.LastRefreshed = sql.NullTime{Time: *session.LastRefreshed, Valid: true}
	}

	// Handle refresh token (stored in refresh_token_hash field for now)
	// TODO: Add proper encryption
	if session.RefreshToken != nil && *session.RefreshToken != "" {
		row.RefreshTokenHash = sql.NullString{String: *session.RefreshToken, Valid: true}
	}

	return row, nil
}

// OIDC Session methods

// CreateOIDCSession creates a new OIDC session
func (r *SessionRepository) CreateOIDCSession(ctx context.Context, session *entities.OIDCSession) error {
	// Generate ID if not set
	if session.ID == "" {
		session.ID = idgen.GenerateID()
	}

	// Set timestamps if not set
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now()
	}

	row, err := oidcSessionRowFromEntity(session)
	if err != nil {
		return err
	}

	// Note: For now, we're only storing basic fields. Full OIDC support would require migration updates.
	query := `
		INSERT INTO oidc_sessions (id, user_id, provider, refresh_token_hash, expires_at, created_at)
		VALUES (:id, :user_id, :provider, :refresh_token_hash, :expires_at, :created_at)
	`

	_, err = r.db.NamedExecContext(ctx, query, row)
	if err != nil {
		return fmt.Errorf("failed to create OIDC session: %w", err)
	}

	return nil
}

// GetOIDCSessionByID retrieves an OIDC session by its ID
func (r *SessionRepository) GetOIDCSessionByID(ctx context.Context, id string) (*entities.OIDCSession, error) {
	var row oidcSessionRow
	query := `SELECT * FROM oidc_sessions WHERE id = $1 LIMIT 1`

	err := r.db.GetContext(ctx, &row, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get OIDC session by ID: %w", err)
	}

	return row.toEntity()
}

// GetOIDCSessionByState retrieves an OIDC session by state
func (r *SessionRepository) GetOIDCSessionByState(ctx context.Context, state string) (*entities.OIDCSession, error) {
	// Note: Current migration schema doesn't support state field
	// This would require a migration update to add state, nonce, etc. fields
	return nil, fmt.Errorf("GetOIDCSessionByState not supported by current schema - state field missing from migration")
}

// GetOIDCSessionByUserAndProvider retrieves the OIDC session (with refresh token) for a user+provider
func (r *SessionRepository) GetOIDCSessionByUserAndProvider(ctx context.Context, userID, provider string) (*entities.OIDCSession, error) {
	var row oidcSessionRow
	query := `SELECT * FROM oidc_sessions WHERE user_id = $1 AND provider = $2 ORDER BY last_refreshed DESC LIMIT 1`

	err := r.db.GetContext(ctx, &row, query, userID, provider)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get OIDC session by user and provider: %w", err)
	}

	return row.toEntity()
}

// UpdateOIDCSession updates an existing OIDC session
func (r *SessionRepository) UpdateOIDCSession(ctx context.Context, session *entities.OIDCSession) error {
	row, err := oidcSessionRowFromEntity(session)
	if err != nil {
		return err
	}

	query := `
		UPDATE oidc_sessions 
		SET user_id = :user_id, provider = :provider, refresh_token_hash = :refresh_token_hash, 
		    expires_at = :expires_at, last_refreshed = :last_refreshed
		WHERE id = :id
	`

	result, err := r.db.NamedExecContext(ctx, query, row)
	if err != nil {
		return fmt.Errorf("failed to update OIDC session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("OIDC session not found: %s", session.ID)
	}

	return nil
}

// DeleteOIDCSession deletes an OIDC session by ID
func (r *SessionRepository) DeleteOIDCSession(ctx context.Context, id string) error {
	query := `DELETE FROM oidc_sessions WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete OIDC session: %w", err)
	}
	return nil
}

// DeleteExpiredOIDCSessions deletes expired OIDC sessions
func (r *SessionRepository) DeleteExpiredOIDCSessions(ctx context.Context, before time.Time) (int64, error) {
	query := `DELETE FROM oidc_sessions WHERE expires_at < $1`
	result, err := r.db.ExecContext(ctx, query, before)
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired OIDC sessions: %w", err)
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return deleted, nil
}

// ListOIDCSessionsByUser lists OIDC sessions for a user with pagination
func (r *SessionRepository) ListOIDCSessionsByUser(ctx context.Context, userID string, opts repositories.ListSessionsOptions) ([]*entities.OIDCSession, int64, error) {
	// Build query conditions
	var conditions []string
	var args []interface{}
	argIndex := 1

	conditions = append(conditions, fmt.Sprintf("user_id = $%d", argIndex))
	args = append(args, userID)
	argIndex++

	// Apply additional filters
	if opts.IsExpired != nil {
		if *opts.IsExpired {
			conditions = append(conditions, fmt.Sprintf("expires_at < $%d", argIndex))
			args = append(args, time.Now())
		} else {
			conditions = append(conditions, fmt.Sprintf("expires_at >= $%d", argIndex))
			args = append(args, time.Now())
		}
		argIndex++
	}

	if opts.CreatedAfter != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIndex))
		args = append(args, *opts.CreatedAfter)
		argIndex++
	}

	if opts.CreatedBefore != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIndex))
		args = append(args, *opts.CreatedBefore)
		argIndex++
	}

	whereClause := "WHERE " + strings.Join(conditions, " AND ")

	// Count total records
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM oidc_sessions %s", whereClause)
	var total int64
	err := r.db.GetContext(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count OIDC sessions: %w", err)
	}

	// Build ORDER BY clause
	sortBy := "created_at"
	if opts.SortBy != "" {
		switch opts.SortBy {
		case "created_at", "expires_at", "last_refreshed":
			sortBy = opts.SortBy
		}
	}

	sortOrder := "DESC"
	if opts.SortOrder == "asc" {
		sortOrder = "ASC"
	}

	// Set defaults for pagination
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	// Build and execute main query
	query := fmt.Sprintf(`
		SELECT * FROM oidc_sessions %s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, whereClause, sortBy, sortOrder, argIndex, argIndex+1)

	args = append(args, limit, offset)

	var rows []oidcSessionRow
	err = r.db.SelectContext(ctx, &rows, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list OIDC sessions: %w", err)
	}

	// Convert to entities
	sessions := make([]*entities.OIDCSession, len(rows))
	for i, row := range rows {
		session, err := row.toEntity()
		if err != nil {
			return nil, 0, fmt.Errorf("failed to convert OIDC session row: %w", err)
		}
		sessions[i] = session
	}

	return sessions, total, nil
}

// CleanupExpiredSessions deletes expired sessions and returns counts
func (r *SessionRepository) CleanupExpiredSessions(ctx context.Context, before time.Time) (oidcDeleted int64, err error) {
	// Delete expired OIDC sessions
	oidcDeleted, err = r.DeleteExpiredOIDCSessions(ctx, before)
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired OIDC sessions: %w", err)
	}

	return oidcDeleted, nil
}
