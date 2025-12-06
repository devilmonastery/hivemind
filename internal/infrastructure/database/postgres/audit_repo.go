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

	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
	"github.com/devilmonastery/hivemind/internal/pkg/idgen"
	"github.com/devilmonastery/hivemind/internal/pkg/metrics"
)

// AuditRepository implements the AuditRepository interface for SQLite
type AuditRepository struct {
	db  *sqlx.DB
	log *slog.Logger
}

// NewAuditRepository creates a new SQLite audit repository
func NewAuditRepository(db *sqlx.DB) repositories.AuditRepository {
	return &AuditRepository{
		db:  db,
		log: slog.Default().With(slog.String("repo", "audit")),
	}
}

// auditLogRow represents an audit log as stored in the database
type auditLogRow struct {
	ID         string         `db:"id"`
	UserID     sql.NullString `db:"user_id"`
	Action     string         `db:"action"`
	Resource   string         `db:"resource_type"`
	ResourceID sql.NullString `db:"resource_id"`
	IPAddress  sql.NullString `db:"ip_address"`
	UserAgent  sql.NullString `db:"user_agent"`
	Metadata   string         `db:"metadata"`
	Success    bool           `db:"success"`
	CreatedAt  time.Time      `db:"timestamp"`
}

// toEntity converts an auditLogRow to a domain entity
func (r *auditLogRow) toEntity() (*entities.AuditLog, error) {
	auditLog := &entities.AuditLog{
		ID:        r.ID,
		Action:    entities.AuditAction(r.Action),
		Resource:  entities.AuditResource(r.Resource),
		Success:   r.Success,
		CreatedAt: r.CreatedAt,
	}

	// Handle nullable fields
	if r.UserID.Valid {
		auditLog.UserID = &r.UserID.String
	}
	if r.ResourceID.Valid {
		auditLog.ResourceID = &r.ResourceID.String
	}
	if r.IPAddress.Valid {
		auditLog.IPAddress = &r.IPAddress.String
	}
	if r.UserAgent.Valid {
		auditLog.UserAgent = &r.UserAgent.String
	}

	// Unmarshal metadata
	if err := auditLog.UnmarshalMetadataFromJSON(r.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return auditLog, nil
}

// fromEntity converts a domain entity to an auditLogRow
func auditLogRowFromEntity(auditLog *entities.AuditLog) (*auditLogRow, error) {
	row := &auditLogRow{
		ID:        auditLog.ID,
		Action:    string(auditLog.Action),
		Resource:  string(auditLog.Resource),
		Success:   auditLog.Success,
		CreatedAt: auditLog.CreatedAt,
	}

	// Handle nullable fields
	if auditLog.UserID != nil {
		row.UserID = sql.NullString{String: *auditLog.UserID, Valid: true}
	}
	if auditLog.ResourceID != nil {
		row.ResourceID = sql.NullString{String: *auditLog.ResourceID, Valid: true}
	}
	if auditLog.IPAddress != nil {
		row.IPAddress = sql.NullString{String: *auditLog.IPAddress, Valid: true}
	}
	if auditLog.UserAgent != nil {
		row.UserAgent = sql.NullString{String: *auditLog.UserAgent, Valid: true}
	}

	// Marshal metadata
	metadata, err := auditLog.MarshalMetadataToJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}
	row.Metadata = metadata

	return row, nil
}

// Create creates a new audit log entry
func (r *AuditRepository) Create(ctx context.Context, log *entities.AuditLog) error {
	start := time.Now()
	var err error
	defer func() {
		metrics.RecordDBOperation("audit", "create", time.Since(start), 1, err)
	}()

	// Generate ID if not set
	if log.ID == "" {
		log.ID = idgen.GenerateID()
	}

	// Set timestamp if not set
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}

	r.log.Debug("creating audit log",
		slog.String("action", string(log.Action)),
		slog.String("resource", string(log.Resource)),
		slog.Any("resource_id", log.ResourceID),
		slog.Any("user_id", log.UserID))

	row, convertErr := auditLogRowFromEntity(log)
	if convertErr != nil {
		err = convertErr
		return err
	}

	query := `
		INSERT INTO audit_logs (id, user_id, action, resource_type, resource_id, ip_address, user_agent, metadata, success, timestamp)
		VALUES (:id, :user_id, :action, :resource_type, :resource_id, :ip_address, :user_agent, :metadata, :success, :timestamp)
	`

	_, err = r.db.NamedExecContext(ctx, query, row)
	if err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	return nil
}

// GetByID retrieves an audit log by its ID
func (r *AuditRepository) GetByID(ctx context.Context, id string) (*entities.AuditLog, error) {
	start := time.Now()
	var err error
	var rowCount int64
	defer func() {
		metrics.RecordDBOperation("audit", "get_by_id", time.Since(start), rowCount, err)
	}()

	var row auditLogRow
	query := `SELECT * FROM audit_logs WHERE id = $1 LIMIT 1`

	err = r.db.GetContext(ctx, &row, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get audit log by ID: %w", err)
	}

	rowCount = 1
	return row.toEntity()
}

// List audit logs with filtering and pagination
func (r *AuditRepository) List(ctx context.Context, opts repositories.ListAuditLogsOptions) ([]*entities.AuditLog, int64, error) {
	start := time.Now()
	var err error
	var rowCount int64
	defer func() {
		metrics.RecordDBOperation("audit", "list", time.Since(start), rowCount, err)
	}()

	// Build query conditions
	var conditions []string
	var args []interface{}
	argIndex := 1

	// Apply filters
	if opts.UserID != nil {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", argIndex))
		args = append(args, *opts.UserID)
		argIndex++
	}

	if opts.Action != nil {
		conditions = append(conditions, fmt.Sprintf("action = $%d", argIndex))
		args = append(args, string(*opts.Action))
		argIndex++
	}

	if len(opts.Actions) > 0 {
		placeholders := make([]string, len(opts.Actions))
		for i, action := range opts.Actions {
			placeholders[i] = fmt.Sprintf("$%d", argIndex)
			args = append(args, string(action))
			argIndex++
		}
		conditions = append(conditions, fmt.Sprintf("action IN (%s)", strings.Join(placeholders, ",")))
	}

	if opts.Resource != nil {
		conditions = append(conditions, fmt.Sprintf("resource_type = $%d", argIndex))
		args = append(args, string(*opts.Resource))
		argIndex++
	}

	if opts.ResourceID != nil {
		conditions = append(conditions, fmt.Sprintf("resource_id = $%d", argIndex))
		args = append(args, *opts.ResourceID)
		argIndex++
	}

	if opts.Success != nil {
		conditions = append(conditions, fmt.Sprintf("success = $%d", argIndex))
		args = append(args, *opts.Success)
		argIndex++
	}

	if opts.IPAddress != nil {
		conditions = append(conditions, fmt.Sprintf("ip_address = $%d", argIndex))
		args = append(args, *opts.IPAddress)
		argIndex++
	}

	if opts.CreatedAfter != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp >= $%d", argIndex))
		args = append(args, *opts.CreatedAfter)
		argIndex++
	}

	if opts.CreatedBefore != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp <= $%d", argIndex))
		args = append(args, *opts.CreatedBefore)
		argIndex++
	}

	if opts.SecurityEventsOnly {
		securityActions := []string{
			string(entities.ActionUserLogin),
			string(entities.ActionUserLoginFailed),
			string(entities.ActionUserLogout),
			string(entities.ActionTokenCreated),
			string(entities.ActionTokenRevoked),
		}
		placeholders := make([]string, len(securityActions))
		for i, action := range securityActions {
			placeholders[i] = fmt.Sprintf("$%d", argIndex)
			args = append(args, action)
			argIndex++
		}
		conditions = append(conditions, fmt.Sprintf("action IN (%s)", strings.Join(placeholders, ",")))
	}

	if opts.FailedOnly {
		conditions = append(conditions, "success = false")
	}

	if opts.UserActionsOnly {
		conditions = append(conditions, "user_id IS NOT NULL")
	}

	// Build WHERE clause
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total records
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM audit_logs %s", whereClause)
	var total int64
	err = r.db.GetContext(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count audit logs: %w", err)
	}

	// Build ORDER BY clause
	sortBy := "timestamp"
	if opts.SortBy != "" {
		switch opts.SortBy {
		case "created_at", "timestamp":
			sortBy = "timestamp"
		case "action":
			sortBy = "action"
		case "resource":
			sortBy = "resource_type"
		case "success":
			sortBy = "success"
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
		SELECT * FROM audit_logs %s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, whereClause, sortBy, sortOrder, argIndex, argIndex+1)

	args = append(args, limit, offset)

	var rows []auditLogRow
	err = r.db.SelectContext(ctx, &rows, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list audit logs: %w", err)
	}

	// Convert to entities
	auditLogs := make([]*entities.AuditLog, len(rows))
	for i, row := range rows {
		auditLog, convertErr := row.toEntity()
		if convertErr != nil {
			err = convertErr
			return nil, 0, fmt.Errorf("failed to convert audit log row: %w", err)
		}
		auditLogs[i] = auditLog
	}

	rowCount = int64(len(rows))
	return auditLogs, total, nil
}

// ListByUser retrieves audit logs for a specific user
func (r *AuditRepository) ListByUser(ctx context.Context, userID string, opts repositories.ListAuditLogsOptions) ([]*entities.AuditLog, int64, error) {
	opts.UserID = &userID
	return r.List(ctx, opts)
}

// ListByResource retrieves audit logs for a specific resource
func (r *AuditRepository) ListByResource(ctx context.Context, resource entities.AuditResource, resourceID string, opts repositories.ListAuditLogsOptions) ([]*entities.AuditLog, int64, error) {
	opts.Resource = &resource
	opts.ResourceID = &resourceID
	return r.List(ctx, opts)
}

// ListSecurityEvents retrieves security-related audit logs
func (r *AuditRepository) ListSecurityEvents(ctx context.Context, opts repositories.ListAuditLogsOptions) ([]*entities.AuditLog, int64, error) {
	opts.SecurityEventsOnly = true
	return r.List(ctx, opts)
}

// DeleteBefore deletes old audit logs (cleanup job)
func (r *AuditRepository) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	start := time.Now()
	var err error
	var rowsAffected int64
	defer func() {
		metrics.RecordDBOperation("audit", "cleanup", time.Since(start), rowsAffected, err)
	}()

	query := `DELETE FROM audit_logs WHERE timestamp < $1`
	result, err := r.db.ExecContext(ctx, query, before)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old audit logs: %w", err)
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	rowsAffected = deleted
	return deleted, nil
}

// CountByAction counts audit logs by action within a time range
func (r *AuditRepository) CountByAction(ctx context.Context, action entities.AuditAction, since time.Time) (int64, error) {
	start := time.Now()
	var err error
	var rowCount int64
	defer func() {
		metrics.RecordDBOperation("audit", "count_by_action", time.Since(start), rowCount, err)
	}()

	query := `SELECT COUNT(*) FROM audit_logs WHERE action = $1 AND timestamp >= $2`
	var count int64
	err = r.db.GetContext(ctx, &count, query, string(action), since)
	if err != nil {
		return 0, fmt.Errorf("failed to count audit logs by action: %w", err)
	}
	rowCount = count
	return count, nil
}

// CountFailedLoginsByUser counts failed login attempts for a user within a time range
func (r *AuditRepository) CountFailedLoginsByUser(ctx context.Context, userID string, since time.Time) (int64, error) {
	start := time.Now()
	var err error
	var rowCount int64
	defer func() {
		metrics.RecordDBOperation("audit", "count_failed_logins_by_user", time.Since(start), rowCount, err)
	}()

	query := `
		SELECT COUNT(*) FROM audit_logs 
		WHERE action = $1 AND user_id = $2 AND success = false AND timestamp >= $3
	`
	var count int64
	err = r.db.GetContext(ctx, &count, query, string(entities.ActionUserLoginFailed), userID, since)
	if err != nil {
		return 0, fmt.Errorf("failed to count failed logins by user: %w", err)
	}
	rowCount = count
	return count, nil
}

// CountFailedLoginsByIP counts failed login attempts from an IP within a time range
func (r *AuditRepository) CountFailedLoginsByIP(ctx context.Context, ipAddress string, since time.Time) (int64, error) {
	start := time.Now()
	var err error
	var rowCount int64
	defer func() {
		metrics.RecordDBOperation("audit", "count_failed_logins_by_ip", time.Since(start), rowCount, err)
	}()

	query := `
		SELECT COUNT(*) FROM audit_logs 
		WHERE action = $1 AND ip_address = $2 AND success = false AND timestamp >= $3
	`
	var count int64
	err = r.db.GetContext(ctx, &count, query, string(entities.ActionUserLoginFailed), ipAddress, since)
	if err != nil {
		return 0, fmt.Errorf("failed to count failed logins by IP: %w", err)
	}
	rowCount = count
	return count, nil
}

// GetRecentActivityByUser gets recent activity for a user
func (r *AuditRepository) GetRecentActivityByUser(ctx context.Context, userID string, limit int) ([]*entities.AuditLog, error) {
	start := time.Now()
	var err error
	var rowCount int64
	defer func() {
		metrics.RecordDBOperation("audit", "get_recent_activity_by_user", time.Since(start), rowCount, err)
	}()

	if limit <= 0 {
		limit = 10
	}

	var rows []auditLogRow
	query := `
		SELECT * FROM audit_logs 
		WHERE user_id = $1 
		ORDER BY timestamp DESC 
		LIMIT $2
	`

	err = r.db.SelectContext(ctx, &rows, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent activity: %w", err)
	}

	// Convert to entities
	auditLogs := make([]*entities.AuditLog, len(rows))
	for i, row := range rows {
		auditLog, convertErr := row.toEntity()
		if convertErr != nil {
			err = convertErr
			return nil, fmt.Errorf("failed to convert audit log row: %w", err)
		}
		auditLogs[i] = auditLog
	}

	rowCount = int64(len(rows))
	return auditLogs, nil
}
