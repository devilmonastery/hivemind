package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
	"github.com/devilmonastery/hivemind/internal/pkg/idgen"
	"github.com/devilmonastery/hivemind/internal/pkg/metrics"
)

// TokenRepository implements the TokenRepository interface for PostgreSQL
type TokenRepository struct {
	db *sqlx.DB
}

// NewTokenRepository creates a new PostgreSQL token repository
func NewTokenRepository(db *sqlx.DB) repositories.TokenRepository {
	return &TokenRepository{
		db: db,
	}
}

// tokenRow represents a token as stored in the database
type tokenRow struct {
	ID         string       `db:"id"`
	UserID     string       `db:"user_id"`
	TokenHash  string       `db:"token_hash"`
	DeviceName string       `db:"device_name"`
	Scopes     string       `db:"scopes"` // JSON string in database
	ExpiresAt  time.Time    `db:"expires_at"`
	CreatedAt  time.Time    `db:"created_at"`
	LastUsed   sql.NullTime `db:"last_used"`
	RevokedAt  sql.NullTime `db:"revoked_at"`
}

// toEntity converts a tokenRow to a domain entity
func (r *tokenRow) toEntity() (*entities.APIToken, error) {
	token := &entities.APIToken{
		ID:         r.ID,
		UserID:     r.UserID,
		TokenHash:  r.TokenHash,
		DeviceName: r.DeviceName,
		ExpiresAt:  r.ExpiresAt,
		CreatedAt:  r.CreatedAt,
	}

	// Parse JSON scopes
	if err := json.Unmarshal([]byte(r.Scopes), &token.Scopes); err != nil {
		return nil, fmt.Errorf("failed to parse scopes JSON: %w", err)
	}

	if r.LastUsed.Valid {
		token.LastUsed = &r.LastUsed.Time
	}

	if r.RevokedAt.Valid {
		token.RevokedAt = &r.RevokedAt.Time
	}

	return token, nil
}

// tokenRowFromEntity converts a domain entity to a tokenRow
func tokenRowFromEntity(token *entities.APIToken) (*tokenRow, error) {
	// Convert scopes to JSON
	scopesJSON, err := json.Marshal(token.Scopes)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal scopes to JSON: %w", err)
	}

	row := &tokenRow{
		ID:         token.ID,
		UserID:     token.UserID,
		TokenHash:  token.TokenHash,
		DeviceName: token.DeviceName,
		Scopes:     string(scopesJSON),
		ExpiresAt:  token.ExpiresAt,
		CreatedAt:  token.CreatedAt,
	}

	if token.LastUsed != nil {
		row.LastUsed = sql.NullTime{Time: *token.LastUsed, Valid: true}
	}

	if token.RevokedAt != nil {
		row.RevokedAt = sql.NullTime{Time: *token.RevokedAt, Valid: true}
	}

	return row, nil
}

// Create creates a new API token
func (r *TokenRepository) Create(ctx context.Context, token *entities.APIToken) error {
	start := time.Now()
	var err error
	defer func() {
		metrics.RecordDBOperation("token", "create", time.Since(start), 1, err)
	}()

	if token.ID == "" {
		token.ID = idgen.GenerateID()
	}

	now := time.Now()
	token.CreatedAt = now

	row, convertErr := tokenRowFromEntity(token)
	if convertErr != nil {
		err = convertErr
		return fmt.Errorf("failed to convert token to row: %w", err)
	}

	query := `INSERT INTO api_tokens (
		id, user_id, token_hash, device_name, scopes, expires_at, created_at, last_used, revoked_at
	) VALUES (
		:id, :user_id, :token_hash, :device_name, :scopes, :expires_at, :created_at, :last_used, :revoked_at
	)`

	_, err = r.db.NamedExecContext(ctx, query, row)
	if err != nil {
		return fmt.Errorf("failed to create token: %w", err)
	}

	return nil
}

// GetByID retrieves a token by its ID
func (r *TokenRepository) GetByID(ctx context.Context, id string) (*entities.APIToken, error) {
	start := time.Now()
	var err error
	var rowCount int64
	defer func() {
		metrics.RecordDBOperation("token", "get_by_id", time.Since(start), rowCount, err)
	}()

	var row tokenRow
	query := `
		SELECT id, user_id, token_hash, device_name, scopes, expires_at, created_at, last_used, revoked_at
		FROM api_tokens 
		WHERE id = $1`

	err = r.db.GetContext(ctx, &row, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // token not found, but that's not an error
		}
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	rowCount = 1
	return row.toEntity()
}

// GetByTokenHash retrieves a token by its hash (for authentication)
func (r *TokenRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*entities.APIToken, error) {
	start := time.Now()
	var err error
	var rowCount int64
	defer func() {
		metrics.RecordDBOperation("token", "get_by_token_hash", time.Since(start), rowCount, err)
	}()

	var row tokenRow
	query := `
		SELECT id, user_id, token_hash, device_name, scopes, expires_at, created_at, last_used, revoked_at
		FROM api_tokens 
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > $2`

	err = r.db.GetContext(ctx, &row, query, tokenHash, time.Now())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // token not found or expired/revoked
		}
		return nil, fmt.Errorf("failed to get token by hash: %w", err)
	}

	rowCount = 1
	return row.toEntity()
}

// Update an existing token
func (r *TokenRepository) Update(ctx context.Context, token *entities.APIToken) error {
	start := time.Now()
	var err error
	var rowsAffected int64
	defer func() {
		metrics.RecordDBOperation("token", "update", time.Since(start), rowsAffected, err)
	}()

	row, convertErr := tokenRowFromEntity(token)
	if convertErr != nil {
		err = convertErr
		return fmt.Errorf("failed to convert token to row: %w", err)
	}

	query := `
		UPDATE api_tokens SET 
			user_id = :user_id,
			token_hash = :token_hash,
			device_name = :device_name,
			scopes = :scopes,
			expires_at = :expires_at,
			last_used = :last_used,
			revoked_at = :revoked_at
		WHERE id = :id`

	result, err := r.db.NamedExecContext(ctx, query, row)
	if err != nil {
		return fmt.Errorf("failed to update token: %w", err)
	}

	rowsAffected, err = result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		err = fmt.Errorf("token not found")
		return err
	}

	return nil
}

// Revoke a token by ID
func (r *TokenRepository) Revoke(ctx context.Context, tokenID string) error {
	start := time.Now()
	var err error
	var rowsAffected int64
	defer func() {
		metrics.RecordDBOperation("token", "revoke", time.Since(start), rowsAffected, err)
	}()

	query := `UPDATE api_tokens SET revoked_at = $1 WHERE id = $2 AND revoked_at IS NULL`

	result, err := r.db.ExecContext(ctx, query, time.Now(), tokenID)
	if err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}

	rowsAffected, _ = result.RowsAffected()
	// Don't check rows affected - revoke is idempotent
	return nil
}

// RevokeAllForUser revokes all tokens for a user
func (r *TokenRepository) RevokeAllForUser(ctx context.Context, userID string) error {
	start := time.Now()
	var err error
	var rowsAffected int64
	defer func() {
		metrics.RecordDBOperation("token", "revoke_all_for_user", time.Since(start), rowsAffected, err)
	}()

	query := `UPDATE api_tokens SET revoked_at = $1 WHERE user_id = $2 AND revoked_at IS NULL`

	result, err := r.db.ExecContext(ctx, query, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to revoke all tokens for user: %w", err)
	}

	// Return the number of tokens revoked (could be useful for logging)
	rowsAffected, _ = result.RowsAffected()
	return nil
}

// ListByUser lists tokens for a user with pagination
func (r *TokenRepository) ListByUser(ctx context.Context, userID string, opts repositories.ListTokensOptions) ([]*entities.APIToken, int64, error) {
	// Force user ID filter
	opts.UserID = &userID
	return r.List(ctx, opts)
}

// List all tokens with pagination and filtering
func (r *TokenRepository) List(ctx context.Context, opts repositories.ListTokensOptions) ([]*entities.APIToken, int64, error) {
	start := time.Now()
	var err error
	var rowCount int64
	defer func() {
		metrics.RecordDBOperation("token", "list", time.Since(start), rowCount, err)
	}()

	// Build query conditions
	var conditions []string
	var args []interface{}
	paramIndex := 1

	if opts.UserID != nil {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", paramIndex))
		args = append(args, *opts.UserID)
		paramIndex++
	}

	if opts.DeviceName != nil {
		conditions = append(conditions, fmt.Sprintf("device_name LIKE $%d", paramIndex))
		args = append(args, "%"+*opts.DeviceName+"%")
		paramIndex++
	}

	if opts.IsRevoked != nil {
		if *opts.IsRevoked {
			conditions = append(conditions, "revoked_at IS NOT NULL")
		} else {
			conditions = append(conditions, "revoked_at IS NULL")
		}
	}

	if opts.IsExpired != nil {
		if *opts.IsExpired {
			conditions = append(conditions, fmt.Sprintf("expires_at <= $%d", paramIndex))
			args = append(args, time.Now())
			paramIndex++
		} else {
			conditions = append(conditions, fmt.Sprintf("expires_at > $%d", paramIndex))
			args = append(args, time.Now())
			paramIndex++
		}
	}

	if opts.CreatedAfter != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", paramIndex))
		args = append(args, *opts.CreatedAfter)
		paramIndex++
	}

	if opts.CreatedBefore != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", paramIndex))
		args = append(args, *opts.CreatedBefore)
		paramIndex++
	}

	// Build WHERE clause
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total records
	countQuery := "SELECT COUNT(*) FROM api_tokens " + whereClause
	var total int64
	err = r.db.GetContext(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count tokens: %w", err)
	}

	// Build ORDER BY clause
	orderBy := "created_at DESC" // default
	if opts.SortBy != "" {
		// Map domain field names to database column names
		sortFieldMap := map[string]string{
			"created_at":  "created_at",
			"last_used":   "last_used",
			"expires_at":  "expires_at",
			"device_name": "device_name",
		}
		if dbField, exists := sortFieldMap[opts.SortBy]; exists {
			direction := "DESC"
			if opts.SortOrder == "asc" {
				direction = "ASC"
			}
			orderBy = fmt.Sprintf("%s %s", dbField, direction)
		}
	}

	// Build main query with pagination
	query := fmt.Sprintf(`
		SELECT id, user_id, token_hash, device_name, scopes, expires_at, created_at, last_used, revoked_at
		FROM api_tokens 
		%s 
		ORDER BY %s 
		LIMIT $%d OFFSET $%d`, whereClause, orderBy, paramIndex, paramIndex+1)

	// Set default pagination
	limit := opts.Limit
	if limit <= 0 {
		limit = 50 // default page size
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	args = append(args, limit, offset)

	var rows []tokenRow
	err = r.db.SelectContext(ctx, &rows, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list tokens: %w", err)
	}

	// Convert rows to entities
	tokens := make([]*entities.APIToken, len(rows))
	for i, row := range rows {
		token, convertErr := row.toEntity()
		if convertErr != nil {
			err = convertErr
			return nil, 0, fmt.Errorf("failed to convert row to entity: %w", err)
		}
		tokens[i] = token
	}

	rowCount = int64(len(rows))
	return tokens, total, nil
}

// DeleteExpired deletes expired tokens (cleanup job)
func (r *TokenRepository) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	start := time.Now()
	var err error
	var rowsAffected int64
	defer func() {
		metrics.RecordDBOperation("token", "delete_expired", time.Since(start), rowsAffected, err)
	}()

	query := `DELETE FROM api_tokens WHERE expires_at <= $1`

	result, err := r.db.ExecContext(ctx, query, before)
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired tokens: %w", err)
	}

	rowsAffected, err = result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}

// DeleteRevokedBefore deletes revoked tokens older than specified time (cleanup job)
func (r *TokenRepository) DeleteRevokedBefore(ctx context.Context, before time.Time) (int64, error) {
	start := time.Now()
	var err error
	var rowsAffected int64
	defer func() {
		metrics.RecordDBOperation("token", "delete_revoked_before", time.Since(start), rowsAffected, err)
	}()

	query := `DELETE FROM api_tokens WHERE revoked_at IS NOT NULL AND revoked_at <= $1`

	result, err := r.db.ExecContext(ctx, query, before)
	if err != nil {
		return 0, fmt.Errorf("failed to delete revoked tokens: %w", err)
	}

	rowsAffected, err = result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}

// UpdateLastUsed updates the last_used timestamp for a token
func (r *TokenRepository) UpdateLastUsed(ctx context.Context, tokenID string, lastUsed time.Time) error {
	start := time.Now()
	var err error
	var rowsAffected int64
	defer func() {
		metrics.RecordDBOperation("token", "update_last_used", time.Since(start), rowsAffected, err)
	}()

	query := `UPDATE api_tokens SET last_used = $1 WHERE id = $2`

	result, err := r.db.ExecContext(ctx, query, lastUsed, tokenID)
	if err != nil {
		return fmt.Errorf("failed to update last used: %w", err)
	}

	rowsAffected, _ = result.RowsAffected()
	return nil
}

// CountActiveByUser counts active tokens for a user
func (r *TokenRepository) CountActiveByUser(ctx context.Context, userID string) (int64, error) {
	start := time.Now()
	var err error
	var rowCount int64
	defer func() {
		metrics.RecordDBOperation("token", "count_active_by_user", time.Since(start), rowCount, err)
	}()

	query := `
		SELECT COUNT(*) 
		FROM api_tokens 
		WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > $2`

	var count int64
	err = r.db.GetContext(ctx, &count, query, userID, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to count active tokens: %w", err)
	}

	rowCount = count
	return count, nil
}
