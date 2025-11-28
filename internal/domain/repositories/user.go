package repositories

import (
	"context"
	"time"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
)

// UserRepository defines the interface for user data access
type UserRepository interface {
	// Create a new user
	Create(ctx context.Context, user *entities.User) error

	// GetByID retrieves a user by their ID
	GetByID(ctx context.Context, id string) (*entities.User, error)

	// GetByEmail retrieves a user by their email address
	GetByEmail(ctx context.Context, email string) (*entities.User, error)

	// GetByOIDCSubject retrieves a user by their OIDC subject
	GetByOIDCSubject(ctx context.Context, subject string) (*entities.User, error)

	// Update an existing user
	Update(ctx context.Context, user *entities.User) error

	// Delete a user (soft delete by setting deleted_at)
	Delete(ctx context.Context, id string) error

	// List users with pagination and optional filtering
	List(ctx context.Context, opts ListUsersOptions) ([]*entities.User, int64, error)

	// UpdateLastLogin updates the user's last login timestamp
	UpdateLastLogin(ctx context.Context, userID string, loginTime time.Time) error

	// Exists checks if a user exists by ID
	Exists(ctx context.Context, id string) (bool, error)

	// ExistsByEmail checks if a user exists by email
	ExistsByEmail(ctx context.Context, email string) (bool, error)
}

// ListUsersOptions provides filtering and pagination options for listing users
type ListUsersOptions struct {
	// Pagination
	Limit  int
	Offset int

	// Filtering
	Role     *entities.Role     // filter by role
	UserType *entities.UserType // filter by user type
	IsActive *bool              // filter by active status
	Search   string             // search in display_name or email

	// Sorting
	SortBy    string // field to sort by (created_at, display_name, email, last_login)
	SortOrder string // asc or desc
}
