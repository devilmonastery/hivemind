package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
	"github.com/devilmonastery/hivemind/internal/pkg/idgen"
	"github.com/devilmonastery/hivemind/internal/pkg/metrics"
)

// UserRepository implements the UserRepository interface for PostgreSQL
type UserRepository struct {
	db  *sqlx.DB
	log *slog.Logger
}

// NewUserRepository creates a new PostgreSQL user repository
func NewUserRepository(db *sqlx.DB) repositories.UserRepository {
	return &UserRepository{
		db:  db,
		log: slog.Default().With(slog.String("repo", "user")),
	}
}

// userRow represents a user as stored in the database
type userRow struct {
	ID           string         `db:"id"`
	Email        sql.NullString `db:"email"`
	DisplayName  string         `db:"name"` // database column is 'name'
	AvatarURL    sql.NullString `db:"avatar_url"`
	Timezone     sql.NullString `db:"timezone"`
	PasswordHash sql.NullString `db:"password_hash"`
	Role         string         `db:"role"`
	UserType     string         `db:"user_type"`
	Disabled     bool           `db:"disabled"`         // database stores 'disabled', not 'is_active'
	OIDCSubject  sql.NullString `db:"provider_user_id"` // database column is 'provider_user_id'
	OIDCProvider sql.NullString `db:"provider"`         // database column is 'provider'
	CreatedAt    time.Time      `db:"created_at"`
	UpdatedAt    time.Time      `db:"updated_at"`
	LastLogin    sql.NullTime   `db:"last_seen"` // database column is 'last_seen'
}

// toEntity converts a userRow to a domain entity
func (r *userRow) toEntity() *entities.User {
	user := &entities.User{
		ID:          r.ID,
		Email:       r.Email.String, // Empty string if NULL
		DisplayName: r.DisplayName,
		Role:        entities.Role(r.Role),
		UserType:    entities.UserType(r.UserType),
		IsActive:    !r.Disabled, // invert the disabled flag to get IsActive
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}

	if r.AvatarURL.Valid {
		user.AvatarURL = &r.AvatarURL.String
	}

	if r.Timezone.Valid {
		user.Timezone = &r.Timezone.String
	}

	if r.PasswordHash.Valid {
		user.PasswordHash = &r.PasswordHash.String
	}

	if r.OIDCSubject.Valid {
		user.OIDCSubject = &r.OIDCSubject.String
	}

	if r.OIDCProvider.Valid {
		user.OIDCProvider = &r.OIDCProvider.String
	}

	if r.LastLogin.Valid {
		user.LastLogin = &r.LastLogin.Time
	}

	return user
}

// fromEntity converts a domain entity to a userRow
func userRowFromEntity(user *entities.User) *userRow {
	row := &userRow{
		ID:          user.ID,
		Email:       sql.NullString{String: user.Email, Valid: user.Email != ""},
		DisplayName: user.DisplayName,
		Role:        string(user.Role),
		UserType:    string(user.UserType),
		Disabled:    !user.IsActive, // invert IsActive to get disabled flag
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
	}

	if user.AvatarURL != nil {
		row.AvatarURL = sql.NullString{String: *user.AvatarURL, Valid: true}
	}

	if user.Timezone != nil {
		row.Timezone = sql.NullString{String: *user.Timezone, Valid: true}
	}

	if user.PasswordHash != nil {
		row.PasswordHash = sql.NullString{String: *user.PasswordHash, Valid: true}
	}

	if user.OIDCSubject != nil {
		row.OIDCSubject = sql.NullString{String: *user.OIDCSubject, Valid: true}
	}

	if user.OIDCProvider != nil {
		row.OIDCProvider = sql.NullString{String: *user.OIDCProvider, Valid: true}
	}

	if user.LastLogin != nil {
		row.LastLogin = sql.NullTime{Time: *user.LastLogin, Valid: true}
	}

	return row
}

// Create creates a new user
func (r *UserRepository) Create(ctx context.Context, user *entities.User) error {
	start := time.Now()
	var err error
	defer func() {
		metrics.RecordDBOperation("user", "create", time.Since(start), 1, err)
	}()

	if user.ID == "" {
		user.ID = idgen.GenerateID()
	}

	r.log.Debug("creating user",
		slog.String("id", user.ID),
		slog.String("email", user.Email),
		slog.String("role", string(user.Role)))

	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	// Note: Password should already be hashed by the caller
	// We don't hash it here to avoid double-hashing

	row := userRowFromEntity(user)

	query := `INSERT INTO users (
			id, email, name, password_hash, role, user_type, 
			disabled, created_at, updated_at, last_seen, avatar_url, timezone
		) VALUES (
			:id, :email, :name, :password_hash, :role, :user_type,
			:disabled, :created_at, :updated_at, :last_seen, :avatar_url, :timezone
		)`

	_, err = r.db.NamedExecContext(ctx, query, row)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetByID retrieves a user by their ID
func (r *UserRepository) GetByID(ctx context.Context, id string) (*entities.User, error) {
	start := time.Now()
	var err error
	var rowCount int64
	defer func() {
		metrics.RecordDBOperation("user", "get_by_id", time.Since(start), rowCount, err)
	}()

	var row userRow
	query := `
		SELECT id, email, name, password_hash, role, user_type,
		       disabled, created_at, updated_at, last_seen, avatar_url, timezone
		FROM users 
		WHERE id = $1`

	err = r.db.GetContext(ctx, &row, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			err = repositories.ErrUserNotFound
			return nil, err
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if row.Disabled {
		err = repositories.ErrUserInactive
		return nil, err
	}

	rowCount = 1
	return row.toEntity(), nil
}

// GetByEmail retrieves a user by their email address
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*entities.User, error) {
	start := time.Now()
	var err error
	var rowCount int64
	defer func() {
		metrics.RecordDBOperation("user", "get_by_email", time.Since(start), rowCount, err)
	}()

	var row userRow
	query := `
		SELECT id, email, name, password_hash, role, user_type,
		       disabled, created_at, updated_at, last_seen, avatar_url, timezone
		FROM users 
		WHERE email = $1`

	err = r.db.GetContext(ctx, &row, query, email)
	if err != nil {
		if err == sql.ErrNoRows {
			err = repositories.ErrUserNotFound
			return nil, err
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	if row.Disabled {
		err = repositories.ErrUserInactive
		return nil, err
	}

	rowCount = 1
	return row.toEntity(), nil
}

// GetByOIDCSubject retrieves a user by their OIDC subject
// Note: This queries the user_identities table since provider info is stored there
func (r *UserRepository) GetByOIDCSubject(ctx context.Context, subject string) (*entities.User, error) {
	var row userRow
	query := `
		SELECT u.id, u.email, u.name, u.password_hash, u.role, u.user_type,
		       u.disabled, u.created_at, u.updated_at, u.last_seen, u.avatar_url, u.timezone
		FROM users u
		INNER JOIN user_identities i ON i.user_id = u.id
		WHERE i.external_id = $1`

	err := r.db.GetContext(ctx, &row, query, subject)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, repositories.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by OIDC subject: %w", err)
	}

	if row.Disabled {
		return nil, repositories.ErrUserInactive
	}

	return row.toEntity(), nil
}

// Update an existing user
func (r *UserRepository) Update(ctx context.Context, user *entities.User) error {
	start := time.Now()
	var err error
	var rowsAffected int64
	defer func() {
		metrics.RecordDBOperation("user", "update", time.Since(start), rowsAffected, err)
	}()

	r.log.Debug("updating user",
		slog.String("id", user.ID),
		slog.String("email", user.Email))

	user.UpdatedAt = time.Now()

	// Hash password if it's being updated and not already hashed
	if user.PasswordHash != nil && *user.PasswordHash != "" && !strings.HasPrefix(*user.PasswordHash, "$2") {
		hashedPassword, hashErr := bcrypt.GenerateFromPassword([]byte(*user.PasswordHash), bcrypt.DefaultCost)
		if hashErr != nil {
			err = hashErr
			return fmt.Errorf("failed to hash password: %w", err)
		}
		hashedStr := string(hashedPassword)
		user.PasswordHash = &hashedStr
	}

	row := userRowFromEntity(user)

	query := `
		UPDATE users SET 
			email = :email,
			name = :name,
			avatar_url = :avatar_url,
			timezone = :timezone,
			password_hash = :password_hash,
			role = :role,
			user_type = :user_type,
			disabled = :disabled,
			updated_at = :updated_at
		WHERE id = :id`

	result, err := r.db.NamedExecContext(ctx, query, row)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rowsAffected, err = result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		err = fmt.Errorf("user not found")
		return err
	}

	return nil
}

// Delete a user (soft delete by setting disabled = true)
func (r *UserRepository) Delete(ctx context.Context, id string) error {
	start := time.Now()
	var err error
	var rowsAffected int64
	defer func() {
		metrics.RecordDBOperation("user", "delete", time.Since(start), rowsAffected, err)
	}()

	query := `UPDATE users SET disabled = true, updated_at = $1 WHERE id = $2`

	result, err := r.db.ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rowsAffected, _ = result.RowsAffected()
	// Don't check rows affected - delete is idempotent
	// Even if user doesn't exist, it's not an error
	return nil
}

// List users with pagination and optional filtering
func (r *UserRepository) List(ctx context.Context, opts repositories.ListUsersOptions) ([]*entities.User, int64, error) {
	start := time.Now()
	var err error
	var rowCount int64
	defer func() {
		metrics.RecordDBOperation("user", "list", time.Since(start), rowCount, err)
	}()

	// Build query conditions
	var conditions []string
	var args []interface{}
	paramIndex := 1 // PostgreSQL uses $1, $2, etc.

	if opts.Role != nil {
		conditions = append(conditions, fmt.Sprintf("role = $%d", paramIndex))
		args = append(args, string(*opts.Role))
		paramIndex++
	}

	if opts.UserType != nil {
		conditions = append(conditions, fmt.Sprintf("user_type = $%d", paramIndex))
		args = append(args, string(*opts.UserType))
		paramIndex++
	}

	if opts.IsActive != nil {
		// Convert IsActive to disabled logic (invert)
		conditions = append(conditions, fmt.Sprintf("disabled = $%d", paramIndex))
		args = append(args, !*opts.IsActive)
		paramIndex++
	}

	if opts.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(name LIKE $%d OR email LIKE $%d)", paramIndex, paramIndex+1))
		searchPattern := "%" + opts.Search + "%"
		args = append(args, searchPattern, searchPattern)
		paramIndex += 2
	}

	// Build WHERE clause
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total records
	countQuery := "SELECT COUNT(*) FROM users " + whereClause
	var total int64
	err = r.db.GetContext(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	// Build ORDER BY clause
	orderBy := "created_at DESC" // default
	if opts.SortBy != "" {
		// Map domain field names to database column names
		sortFieldMap := map[string]string{
			"created_at":   "created_at",
			"display_name": "name", // domain uses display_name, db uses name
			"email":        "email",
			"last_login":   "last_seen", // domain uses last_login, db uses last_seen
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
		SELECT id, email, name, password_hash, role, user_type,
		       disabled, created_at, updated_at, last_seen, avatar_url, timezone
		FROM users 
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

	var rows []userRow
	err = r.db.SelectContext(ctx, &rows, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}

	rowCount = int64(len(rows))

	// Convert to entities
	users := make([]*entities.User, len(rows))
	for i, row := range rows {
		users[i] = row.toEntity()
	}

	return users, total, nil
}

// UpdateLastLogin updates the user's last login timestamp
func (r *UserRepository) UpdateLastLogin(ctx context.Context, userID string, loginTime time.Time) error {
	start := time.Now()
	var err error
	var rowsAffected int64
	defer func() {
		metrics.RecordDBOperation("user", "update_last_login", time.Since(start), rowsAffected, err)
	}()

	r.log.Debug("updating user last login",
		slog.String("user_id", userID),
		slog.Time("login_time", loginTime))

	query := `UPDATE users SET last_seen = $1, updated_at = $2 WHERE id = $3`

	result, err := r.db.ExecContext(ctx, query, loginTime, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}

	rowsAffected, err = result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		err = fmt.Errorf("user not found")
		return err
	}

	return nil
}

// Exists checks if a user exists by ID
func (r *UserRepository) Exists(ctx context.Context, id string) (bool, error) {
	start := time.Now()
	var err error
	var rowCount int64
	defer func() {
		metrics.RecordDBOperation("user", "exists", time.Since(start), rowCount, err)
	}()

	var count int
	query := `SELECT COUNT(*) FROM users WHERE id = $1`

	err = r.db.GetContext(ctx, &count, query, id)
	if err != nil {
		return false, fmt.Errorf("failed to check user existence: %w", err)
	}

	rowCount = int64(count)
	return count > 0, nil
}

// ExistsByEmail checks if a user exists by email
func (r *UserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	start := time.Now()
	var err error
	var rowCount int64
	defer func() {
		metrics.RecordDBOperation("user", "exists_by_email", time.Since(start), rowCount, err)
	}()

	var count int
	query := `SELECT COUNT(*) FROM users WHERE email = $1`

	err = r.db.GetContext(ctx, &count, query, email)
	if err != nil {
		return false, fmt.Errorf("failed to check user existence by email: %w", err)
	}

	rowCount = int64(count)
	return count > 0, nil
}
