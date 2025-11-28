package repositories

import (
	"context"
)

// Repositories is a collection of all repository interfaces
type Repositories struct {
	Users    UserRepository
	Tokens   TokenRepository
	Sessions SessionRepository
	Audit    AuditRepository
}

// UnitOfWork defines transaction management for repositories
type UnitOfWork interface {
	// Begin starts a new transaction
	Begin(ctx context.Context) (Transaction, error)
}

// Transaction defines transaction operations
type Transaction interface {
	// Commit commits the transaction
	Commit() error

	// Rollback rolls back the transaction
	Rollback() error

	// GetRepositories returns repositories bound to this transaction
	GetRepositories() *Repositories
}

// HealthChecker defines health check interface for repositories
type HealthChecker interface {
	// HealthCheck performs a health check on the repository
	HealthCheck(ctx context.Context) error
}
