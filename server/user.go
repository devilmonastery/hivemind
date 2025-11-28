package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"

	"github.com/devilmonastery/hivemind/internal/config"
	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/infrastructure/database/postgres"
	"github.com/devilmonastery/hivemind/internal/pkg/idgen"
	"github.com/devilmonastery/hivemind/migrations"
)

func newUserCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "User management commands",
		Long:  "Commands for managing users in the Hivemind database",
	}

	cmd.AddCommand(newUserCreateCommand())

	return cmd
}

func newUserCreateCommand() *cobra.Command {
	var (
		email      string
		password   string
		name       string
		role       string
		userType   string
		isActive   bool
		configPath string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new user",
		Long:  "Create a new user with the specified email, password, and role",
		Example: `  # Create an admin user
  server user create --email admin@example.com --password secret123 --role admin --name "Admin User"

  # Create a regular user
  server user create --email user@example.com --password pass123 --role user --name "Regular User"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return createUser(configPath, email, password, name, role, userType, isActive)
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "User email (required)")
	cmd.Flags().StringVar(&password, "password", "", "User password (required)")
	cmd.Flags().StringVar(&name, "name", "", "User display name (optional)")
	cmd.Flags().StringVar(&role, "role", "user", "User role (user, admin)")
	cmd.Flags().StringVar(&userType, "type", "local", "User type (local, oidc)")
	cmd.Flags().BoolVar(&isActive, "active", true, "Whether user is active")
	cmd.Flags().StringVar(&configPath, "config", "", "Path to config file (optional)")

	cmd.MarkFlagRequired("email")
	cmd.MarkFlagRequired("password")

	return cmd
}

func createUser(configPath, email, password, name, role, userType string, isActive bool) error {
	// Initialize ID generator
	if err := idgen.Initialize(1); err != nil {
		return fmt.Errorf("failed to initialize ID generator: %w", err)
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize database
	pgConn, err := postgres.NewConnection(cfg.Database.Postgres.ConnectionString())
	if err != nil {
		return fmt.Errorf("failed to connect to PostgreSQL database: %w", err)
	}
	defer pgConn.Close()

	// Run migrations to ensure database is up to date
	if err := pgConn.RunMigrations(migrations.FS); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Initialize user repository
	userRepo := postgres.NewUserRepository(pgConn.DB)

	// Check if user already exists
	ctx := context.Background()
	existingUser, err := userRepo.GetByEmail(ctx, email)
	if err == nil && existingUser != nil {
		return fmt.Errorf("user with email %s already exists", email)
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user entity
	hashedPasswordStr := string(hashedPassword)
	user := &entities.User{
		ID:           idgen.GenerateID(),
		Email:        email,
		PasswordHash: &hashedPasswordStr,
		DisplayName:  name,
		Role:         entities.Role(role),
		UserType:     entities.UserType(userType),
		IsActive:     isActive,
	}

	// If no name provided, use email
	if user.DisplayName == "" {
		user.DisplayName = email
	}

	// Validate role
	if user.Role != entities.RoleUser && user.Role != entities.RoleAdmin {
		return fmt.Errorf("invalid role: %s (must be 'user' or 'admin')", role)
	}

	// Validate user type
	if user.UserType != entities.UserTypeLocal && user.UserType != entities.UserTypeOIDC {
		return fmt.Errorf("invalid user type: %s (must be 'local' or 'oidc')", userType)
	}

	// Save user to database
	if err := userRepo.Create(ctx, user); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	slog.Info("User created successfully",
		"user_id", user.ID,
		"email", user.Email,
		"display_name", user.DisplayName,
		"role", user.Role,
		"user_type", user.UserType,
		"is_active", user.IsActive,
	)

	return nil
}
