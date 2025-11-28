package repositories

import (
	"context"
	"time"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
)

// AuditRepository defines the interface for audit log data access
type AuditRepository interface {
	// Create a new audit log entry
	Create(ctx context.Context, log *entities.AuditLog) error

	// GetByID retrieves an audit log by its ID
	GetByID(ctx context.Context, id string) (*entities.AuditLog, error)

	// List audit logs with filtering and pagination
	List(ctx context.Context, opts ListAuditLogsOptions) ([]*entities.AuditLog, int64, error)

	// ListByUser retrieves audit logs for a specific user
	ListByUser(ctx context.Context, userID string, opts ListAuditLogsOptions) ([]*entities.AuditLog, int64, error)

	// ListByResource retrieves audit logs for a specific resource
	ListByResource(ctx context.Context, resource entities.AuditResource, resourceID string, opts ListAuditLogsOptions) ([]*entities.AuditLog, int64, error)

	// ListSecurityEvents retrieves security-related audit logs (failed logins, token issues, etc.)
	ListSecurityEvents(ctx context.Context, opts ListAuditLogsOptions) ([]*entities.AuditLog, int64, error)

	// Delete old audit logs (cleanup job)
	DeleteBefore(ctx context.Context, before time.Time) (int64, error)

	// Count audit logs by action within a time range (for metrics)
	CountByAction(ctx context.Context, action entities.AuditAction, since time.Time) (int64, error)

	// Count failed login attempts for a user within a time range
	CountFailedLoginsByUser(ctx context.Context, userID string, since time.Time) (int64, error)

	// Count failed login attempts from an IP within a time range
	CountFailedLoginsByIP(ctx context.Context, ipAddress string, since time.Time) (int64, error)

	// GetRecentActivity gets recent activity for a user
	GetRecentActivityByUser(ctx context.Context, userID string, limit int) ([]*entities.AuditLog, error)
}

// ListAuditLogsOptions provides filtering and pagination options for listing audit logs
type ListAuditLogsOptions struct {
	// Pagination
	Limit  int
	Offset int

	// Filtering
	UserID        *string                 // filter by user ID
	Action        *entities.AuditAction   // filter by specific action
	Actions       []entities.AuditAction  // filter by multiple actions
	Resource      *entities.AuditResource // filter by resource type
	ResourceID    *string                 // filter by specific resource ID
	Success       *bool                   // filter by success status
	IPAddress     *string                 // filter by IP address
	CreatedAfter  *time.Time              // filter by creation date
	CreatedBefore *time.Time              // filter by creation date

	// Special filters
	SecurityEventsOnly bool // only return security-related events
	FailedOnly         bool // only return failed events
	UserActionsOnly    bool // only return user-initiated actions (exclude system)

	// Sorting
	SortBy    string // field to sort by (created_at, action, resource, success)
	SortOrder string // asc or desc
}
