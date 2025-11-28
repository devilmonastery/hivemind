package postgres

import (
	"embed"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// Connection manages PostgreSQL database connection
type Connection struct {
	DB *sqlx.DB
}

// NewConnection creates a new PostgreSQL database connection
// connectionString format: "postgres://user:password@localhost:5432/dbname?sslmode=disable"
func NewConnection(connectionString string) (*Connection, error) {
	// Connect to database
	db, err := sqlx.Connect("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)   // Max number of open connections
	db.SetMaxIdleConns(5)    // Max number of idle connections
	db.SetConnMaxLifetime(0) // Connections never expire

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Connection{DB: db}, nil
}

// Close closes the database connection
func (c *Connection) Close() error {
	return c.DB.Close()
}

// RunMigrations runs the database migrations using golang-migrate
func (c *Connection) RunMigrations(migrationFS embed.FS) error {
	// Create a sub-filesystem for PostgreSQL migrations
	postgresMigrations, err := fs.Sub(migrationFS, "postgres")
	if err != nil {
		return fmt.Errorf("failed to create postgres migrations sub-filesystem: %w", err)
	}

	// Create migration source from embedded filesystem
	source, err := iofs.New(postgresMigrations, ".")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	// Get the underlying sql.DB from sqlx.DB
	sqlDB := c.DB.DB

	// Create database driver
	driver, err := postgres.WithInstance(sqlDB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create database driver: %w", err)
	}

	// Create migrate instance with source and database
	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	// Check current migration version and database state
	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	// If the migration is dirty, try to fix it
	if dirty {
		// Check if the database actually has the expected schema for this version
		if c.isDatabaseEmpty() {
			// Database is empty but migration thinks it's at some version - reset to version 0
			if err := m.Force(0); err != nil {
				return fmt.Errorf("failed to force reset dirty migration: %w", err)
			}
		} else {
			// Database has content but is dirty - force to the current version to clean it
			if err := m.Force(int(version)); err != nil {
				return fmt.Errorf("failed to force clean dirty migration: %w", err)
			}
		}
	}

	// Run migrations
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// isDatabaseEmpty checks if the database has no user tables
func (c *Connection) isDatabaseEmpty() bool {
	var count int
	query := `SELECT COUNT(*) FROM information_schema.tables 
              WHERE table_schema = 'public' 
              AND table_name NOT IN ('schema_migrations', 'schema_migration')`
	err := c.DB.Get(&count, query)
	return err == nil && count == 0
}

// ForceMigrationVersion forces the migration version to a specific number
// This should only be used to recover from dirty migration states
func (c *Connection) ForceMigrationVersion(migrationFS embed.FS, version int) error {
	// Create a sub-filesystem for PostgreSQL migrations
	postgresMigrations, err := fs.Sub(migrationFS, "postgres")
	if err != nil {
		return fmt.Errorf("failed to create postgres migrations sub-filesystem: %w", err)
	}

	// Create migration source from embedded filesystem
	source, err := iofs.New(postgresMigrations, ".")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	// Get the underlying sql.DB from sqlx.DB
	sqlDB := c.DB.DB

	// Create database driver
	driver, err := postgres.WithInstance(sqlDB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create database driver: %w", err)
	}

	// Create migrate instance with source and database
	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	// Force the version
	err = m.Force(version)
	if err != nil {
		return fmt.Errorf("failed to force migration version %d: %w", version, err)
	}

	return nil
}
