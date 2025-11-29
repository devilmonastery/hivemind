package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/cobra"

	"github.com/devilmonastery/hivemind/internal/auth"
	"github.com/devilmonastery/hivemind/internal/config"
	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
	"github.com/devilmonastery/hivemind/internal/infrastructure/database/postgres"
	"github.com/devilmonastery/hivemind/internal/pkg/idgen"
	"github.com/devilmonastery/hivemind/migrations"
)

func newTokenCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Token management commands",
		Long:  "Commands for managing authentication tokens in the Hivemind system",
	}

	cmd.AddCommand(newCreateServiceTokenCommand())
	cmd.AddCommand(newListTokensCommand())
	cmd.AddCommand(newRevokeTokenCommand())

	return cmd
}

func newCreateServiceTokenCommand() *cobra.Command {
	var (
		name       string
		role       string
		expiryDays int
		configPath string
	)

	cmd := &cobra.Command{
		Use:   "create-service",
		Short: "Create a service account token",
		Long: `Create a JWT token for service accounts (bots, automation).

Service tokens are designed for non-interactive authentication and can be used
by the Discord bot or other automated systems to communicate with the backend.

The token will be stored in the database for audit purposes and can be revoked.

Examples:
  # Create a bot service token (1 year expiry)
  server token create-service --name "discord-bot-prod" --role bot --expiry-days 365

  # Create an admin service token for automation (90 days expiry)
  server token create-service --name "ci-automation" --role admin --expiry-days 90`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return createServiceToken(configPath, name, role, expiryDays)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Service name/description (required)")
	cmd.Flags().StringVar(&role, "role", "bot", "Service role: 'bot' or 'admin'")
	cmd.Flags().IntVar(&expiryDays, "expiry-days", 365, "Token expiry in days")
	cmd.Flags().StringVar(&configPath, "config", "", "Path to config file")

	cmd.MarkFlagRequired("name")

	return cmd
}

func newListTokensCommand() *cobra.Command {
	var (
		userID     string
		configPath string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tokens",
		Long:  "List API tokens in the system (optionally filtered by user)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listTokens(configPath, userID)
		},
	}

	cmd.Flags().StringVar(&userID, "user-id", "", "Filter by user ID")
	cmd.Flags().StringVar(&configPath, "config", "", "Path to config file")

	return cmd
}

func newRevokeTokenCommand() *cobra.Command {
	var (
		tokenID    string
		configPath string
	)

	cmd := &cobra.Command{
		Use:   "revoke",
		Short: "Revoke a token",
		Long:  "Revoke an API or service token by its ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			return revokeToken(configPath, tokenID)
		},
	}

	cmd.Flags().StringVar(&tokenID, "token-id", "", "Token ID to revoke (required)")
	cmd.Flags().StringVar(&configPath, "config", "", "Path to config file")

	cmd.MarkFlagRequired("token-id")

	return cmd
}

func createServiceToken(configPath, name, role string, expiryDays int) error {
	// Initialize ID generator
	if err := idgen.Initialize(1); err != nil {
		return fmt.Errorf("failed to initialize ID generator: %w", err)
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Validate role
	if role != "bot" && role != "admin" {
		return fmt.Errorf("invalid role: %s (must be 'bot' or 'admin')", role)
	}

	// Initialize database
	pgConn, err := postgres.NewConnection(cfg.Database.Postgres.ConnectionString())
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pgConn.Close()

	// Run migrations
	if err := pgConn.RunMigrations(migrations.FS); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Initialize repositories
	userRepo := postgres.NewUserRepository(pgConn.DB)
	tokenRepo := postgres.NewTokenRepository(pgConn.DB)

	ctx := context.Background()

	// Create or get service user
	serviceUserID := fmt.Sprintf("service-%s", role)
	serviceUsername := fmt.Sprintf("service/%s", name)

	_, err = userRepo.GetByID(ctx, serviceUserID)
	if err != nil {
		// Create service user if doesn't exist
		serviceUser := &entities.User{
			ID:          serviceUserID,
			Email:       fmt.Sprintf("%s@service.hivemind", name),
			DisplayName: fmt.Sprintf("Service: %s", name),
			Role:        entities.Role(role),
			UserType:    entities.UserTypeLocal,
			IsActive:    true,
		}
		if err := userRepo.Create(ctx, serviceUser); err != nil {
			return fmt.Errorf("failed to create service user: %w", err)
		}
		slog.Info("Created service user", "user_id", serviceUserID)
	}

	// Generate token ID
	tokenID, err := auth.GenerateTokenID()
	if err != nil {
		return fmt.Errorf("failed to generate token ID: %w", err)
	}

	// Create JWT token
	tokenDuration := time.Duration(expiryDays) * 24 * time.Hour
	jwtManager := auth.NewJWTManager(cfg.Auth.JWT.SigningKey, tokenDuration)

	jwtToken, expiresAt, err := jwtManager.GenerateToken(
		serviceUserID,
		serviceUsername,
		role,
		tokenID,
	)
	if err != nil {
		return fmt.Errorf("failed to generate JWT: %w", err)
	}

	// Hash the JWT for storage
	hash := sha256.Sum256([]byte(jwtToken))
	tokenHash := base64.StdEncoding.EncodeToString(hash[:])

	// Store token in database (for audit/revocation)
	apiToken := &entities.APIToken{
		ID:         tokenID,
		UserID:     serviceUserID,
		TokenHash:  tokenHash,
		DeviceName: fmt.Sprintf("service:%s", name),
		Scopes:     []string{"service:all"}, // Service tokens get all scopes
		ExpiresAt:  expiresAt,
		CreatedAt:  time.Now(),
	}

	if err := tokenRepo.Create(ctx, apiToken); err != nil {
		return fmt.Errorf("failed to store token: %w", err)
	}

	// Display success message with the token
	fmt.Println("\n✅ Service token created successfully!")
	fmt.Println("\n⚠️  IMPORTANT: Save this token securely. It will not be shown again.")
	fmt.Println()
	fmt.Printf("Service Name:  %s\n", name)
	fmt.Printf("Role:          %s\n", role)
	fmt.Printf("User ID:       %s\n", serviceUserID)
	fmt.Printf("Token ID:      %s\n", tokenID)
	fmt.Printf("Expires At:    %s\n", expiresAt.Format(time.RFC3339))
	fmt.Println()
	fmt.Println("JWT Token:")
	fmt.Println(jwtToken)
	fmt.Println()
	fmt.Println("Configuration Examples:")
	fmt.Printf("  Bot config:     service_token: \"%s\"\n", jwtToken)
	fmt.Printf("  Environment:    export SERVICE_TOKEN=\"%s\"\n", jwtToken)
	fmt.Println()
	fmt.Println("Security Notes:")
	fmt.Println("  • Store this token in a secure secret management system")
	fmt.Println("  • For production, use environment variables or secret managers")
	fmt.Println("  • Never commit tokens to version control")
	fmt.Println("  • Rotate tokens regularly")
	fmt.Printf("  • Revoke with: server token revoke --token-id %s\n", tokenID)
	fmt.Println()

	slog.Info("Service token created",
		"name", name,
		"role", role,
		"user_id", serviceUserID,
		"token_id", tokenID,
		"expires_at", expiresAt)

	return nil
}

func listTokens(configPath, userID string) error {
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
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pgConn.Close()

	tokenRepo := postgres.NewTokenRepository(pgConn.DB)
	ctx := context.Background()

	var tokens []*entities.APIToken
	if userID != "" {
		tokens, _, err = tokenRepo.ListByUser(ctx, userID, repositories.ListTokensOptions{})
	} else {
		tokens, _, err = tokenRepo.List(ctx, repositories.ListTokensOptions{})
	}

	if err != nil {
		return fmt.Errorf("failed to list tokens: %w", err)
	}

	if len(tokens) == 0 {
		fmt.Println("No tokens found")
		return nil
	}

	fmt.Printf("\nFound %d token(s):\n\n", len(tokens))
	for _, token := range tokens {
		status := "Active"
		if token.RevokedAt != nil {
			status = fmt.Sprintf("Revoked (%s)", token.RevokedAt.Format(time.RFC3339))
		} else if time.Now().After(token.ExpiresAt) {
			status = "Expired"
		}

		fmt.Printf("Token ID:      %s\n", token.ID)
		fmt.Printf("User ID:       %s\n", token.UserID)
		fmt.Printf("Device/Name:   %s\n", token.DeviceName)
		fmt.Printf("Scopes:        %v\n", token.Scopes)
		fmt.Printf("Status:        %s\n", status)
		fmt.Printf("Created:       %s\n", token.CreatedAt.Format(time.RFC3339))
		fmt.Printf("Expires:       %s\n", token.ExpiresAt.Format(time.RFC3339))
		if token.LastUsed != nil {
			fmt.Printf("Last Used:     %s\n", token.LastUsed.Format(time.RFC3339))
		}
		fmt.Println()
	}

	return nil
}

func revokeToken(configPath, tokenID string) error {
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
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pgConn.Close()

	tokenRepo := postgres.NewTokenRepository(pgConn.DB)
	ctx := context.Background()

	// Get token
	token, err := tokenRepo.GetByID(ctx, tokenID)
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	if token.RevokedAt != nil {
		return fmt.Errorf("token is already revoked")
	}

	// Revoke token
	now := time.Now()
	token.RevokedAt = &now
	if err := tokenRepo.Update(ctx, token); err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}

	fmt.Printf("✅ Token %s has been revoked\n", tokenID)
	slog.Info("Token revoked", "token_id", tokenID)

	return nil
}
