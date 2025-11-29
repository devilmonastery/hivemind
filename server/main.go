package main

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	adminpb "github.com/devilmonastery/hivemind/api/generated/go/adminpb"
	authpb "github.com/devilmonastery/hivemind/api/generated/go/authpb"
	notespb "github.com/devilmonastery/hivemind/api/generated/go/notespb"
	quotespb "github.com/devilmonastery/hivemind/api/generated/go/quotespb"
	"github.com/devilmonastery/hivemind/api/generated/go/tokenspb"
	wikipb "github.com/devilmonastery/hivemind/api/generated/go/wikipb"
	"github.com/devilmonastery/hivemind/internal/auth"
	"github.com/devilmonastery/hivemind/internal/auth/oidc"
	"github.com/devilmonastery/hivemind/internal/config"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
	"github.com/devilmonastery/hivemind/internal/domain/services"
	"github.com/devilmonastery/hivemind/internal/infrastructure/database/postgres"
	"github.com/devilmonastery/hivemind/internal/pkg/idgen"
	"github.com/devilmonastery/hivemind/internal/pkg/logger"
	"github.com/devilmonastery/hivemind/migrations"
	"github.com/devilmonastery/hivemind/server/internal/grpc/handlers"
	"github.com/devilmonastery/hivemind/server/internal/grpc/interceptors"
)

func main() {
	rootCmd := newRootCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	var (
		forceVersion  int
		configPath    string
		logLevel      string
		logFile       string
		logToStderr   bool
		alsoLogStderr bool
		logFormat     string
	)

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Hivemind gRPC server",
		Long:  "The gRPC server for the Hivemind service",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return setupServerLogging(logLevel, logFile, logToStderr, alsoLogStderr, logFormat)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer(configPath, forceVersion)
		},
	}

	cmd.Flags().IntVar(&forceVersion, "force-migration", -1, "Force migration version (use to fix dirty migration state)")
	cmd.Flags().StringVar(&configPath, "config", "", "Path to config file (optional)")

	// Add logging flags
	cmd.Flags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	cmd.Flags().StringVar(&logFile, "log-file", "", "Log file path (if specified, logs to file instead of stderr)")
	cmd.Flags().BoolVar(&logToStderr, "logtostderr", false, "Log to stderr (default behavior unless --log-file specified)")
	cmd.Flags().BoolVar(&alsoLogStderr, "alsologtostderr", false, "Log to both file and stderr")
	cmd.Flags().StringVar(&logFormat, "log-format", "json", "Log format (text, json)")

	// Add subcommands
	cmd.AddCommand(newUserCommand())
	cmd.AddCommand(newTokenCommand())

	return cmd
}

// setupServerLogging configures the global logger for the server
func setupServerLogging(logLevel, logFile string, logToStderr, alsoLogStderr bool, logFormat string) error {
	// Default to stderr logging unless file is specified
	if logFile == "" {
		logToStderr = true
	}

	cfg := logger.Config{
		Level:         logger.ParseLevel(logLevel),
		LogFile:       logFile,
		LogToStderr:   logToStderr,
		AlsoLogStderr: alsoLogStderr,
		Format:        logFormat,
	}

	globalLogger, err := logger.SetupLogger(cfg)
	if err != nil {
		return err
	}

	// Set as default logger
	slog.SetDefault(globalLogger)

	return nil
}

func runServer(configPath string, forceVersion int) error {
	logger := slog.Default().With("component", "server")
	logger.Info("Starting server initialization")

	// Initialize Snowflake ID generator
	if err := idgen.Initialize(1); err != nil {
		return fmt.Errorf("failed to initialize ID generator: %w", err)
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Debug: log loaded providers
	logger.Info("Loaded OAuth providers", "count", len(cfg.Auth.Providers))
	for _, p := range cfg.Auth.Providers {
		logger.Info("OAuth provider configured",
			"name", p.Name,
			"client_id", p.ClientID,
			"auto_provision", p.AutoProvision)
	}

	// Initialize OIDC providers from config
	if err := oidc.InitializeProviders(cfg.Auth.Providers); err != nil {
		return fmt.Errorf("failed to initialize OIDC providers: %w", err)
	}
	logger.Info("OIDC providers initialized")

	// Initialize database based on type
	var userRepo repositories.UserRepository
	var tokenRepo repositories.TokenRepository
	var sessionRepo repositories.SessionRepository
	var auditRepo repositories.AuditRepository

	// Initialize PostgreSQL database
	logger.Info("Initializing PostgreSQL database",
		"user", cfg.Database.Postgres.User,
		"host", cfg.Database.Postgres.Host,
		"database", cfg.Database.Postgres.Database)

	// Get connection string from config
	connString := cfg.Database.Postgres.ConnectionString()

	// Connect to PostgreSQL with retries (for Kubernetes startup)
	var pgConn *postgres.Connection
	maxRetries := 10
	retryDelay := 2 * time.Second

	for i := 0; i < maxRetries; i++ {
		var err error
		pgConn, err = postgres.NewConnection(connString)
		if err == nil {
			logger.Info("Successfully connected to PostgreSQL")
			break
		}

		if i < maxRetries-1 {
			logger.Warn("Failed to connect to PostgreSQL",
				"attempt", i+1,
				"max_retries", maxRetries,
				"error", err,
				"retry_delay", retryDelay)
			time.Sleep(retryDelay)
			retryDelay *= 2 // Exponential backoff
			if retryDelay > 30*time.Second {
				retryDelay = 30 * time.Second
			}
		} else {
			return fmt.Errorf("failed to connect to PostgreSQL after %d attempts: %w", maxRetries, err)
		}
	}
	defer pgConn.Close()

	// Handle force migration if requested
	if forceVersion >= 0 {
		logger.Info("Force setting migration version", "version", forceVersion)
		if err := pgConn.ForceMigrationVersion(migrations.FS, forceVersion); err != nil {
			return fmt.Errorf("failed to force migration version: %w", err)
		}
		logger.Info("Migration version forced, exiting", "version", forceVersion)
		return nil
	}

	// Run migrations
	if err := pgConn.RunMigrations(migrations.FS); err != nil {
		return fmt.Errorf("failed to run PostgreSQL migrations: %w", err)
	}

	// Initialize PostgreSQL repositories
	userRepo = postgres.NewUserRepository(pgConn.DB)
	tokenRepo = postgres.NewTokenRepository(pgConn.DB)
	sessionRepo = postgres.NewSessionRepository(pgConn.DB)
	auditRepo = postgres.NewAuditRepository(pgConn.DB)
	discordUserRepo := postgres.NewDiscordUserRepository(pgConn.DB)
	wikiPageRepo := postgres.NewWikiPageRepository(pgConn.DB.DB)
	noteRepo := postgres.NewNoteRepository(pgConn.DB.DB)
	quoteRepo := postgres.NewQuoteRepository(pgConn.DB.DB)

	// Initialize JWT manager from config
	if cfg.Auth.JWT.SigningKey == "" {
		logger.Error("JWT signing key not configured")
		os.Exit(1)
	}
	if cfg.Auth.JWT.Lifetime == 0 {
		logger.Error("JWT lifetime not configured")
		os.Exit(1)
	}
	jwtManager := auth.NewJWTManager(cfg.Auth.JWT.SigningKey, cfg.Auth.JWT.Lifetime)

	// Initialize services
	userService := services.NewUserService(userRepo, auditRepo)
	tokenService := services.NewTokenService(tokenRepo, userRepo, auditRepo)
	discordService := services.NewDiscordService(discordUserRepo, userRepo, logger)
	wikiService := services.NewWikiService(wikiPageRepo)
	noteService := services.NewNoteService(noteRepo)
	quoteService := services.NewQuoteService(quoteRepo)
	authHandler := handlers.NewAuthHandler(userRepo, tokenRepo, sessionRepo, discordUserRepo, jwtManager, cfg)

	// Initialize auth interceptor
	authInterceptor := interceptors.NewAuthInterceptor(jwtManager, tokenRepo, discordService, cfg.Auth.DevBotToken)

	// Initialize gRPC handlers
	adminHandler := handlers.NewAdminHandler(userService)
	tokenHandler := handlers.NewTokenHandler(tokenService)
	wikiHandler := handlers.NewWikiHandler(wikiService)
	noteHandler := handlers.NewNoteHandler(noteService)
	quoteHandler := handlers.NewQuoteHandler(quoteService)

	// Create gRPC server with interceptors and keepalive
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(authInterceptor.Unary()),
		grpc.StreamInterceptor(authInterceptor.Stream()),
		// Keepalive settings to prevent connections from being dropped
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     15 * time.Minute, // Close idle connections after 15 min
			MaxConnectionAge:      30 * time.Minute, // Close connections after 30 min
			MaxConnectionAgeGrace: 5 * time.Second,  // Grace period for active RPCs
			Time:                  5 * time.Second,  // Send keepalive ping every 5 seconds if idle
			Timeout:               1 * time.Second,  // Wait 1 second for ping ack
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second, // Minimum time between client pings
			PermitWithoutStream: true,            // Allow pings when no active streams
		}),
	)

	// Register services
	adminpb.RegisterAdminServiceServer(grpcServer, adminHandler)
	tokenspb.RegisterTokenServiceServer(grpcServer, tokenHandler)
	authpb.RegisterAuthServiceServer(grpcServer, authHandler)
	wikipb.RegisterWikiServiceServer(grpcServer, wikiHandler)
	notespb.RegisterNoteServiceServer(grpcServer, noteHandler)
	quotespb.RegisterQuoteServiceServer(grpcServer, quoteHandler)

	// Register health check service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	// Set all services to SERVING status
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// Enable reflection
	reflection.Register(grpcServer)

	// Start listening
	address := cfg.GRPC.Host + ":" + fmt.Sprintf("%d", cfg.GRPC.Port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", address, err)
	}

	logger.Info("Starting gRPC server", "address", address)

	// Log database info
	logger.Info("Database configuration",
		"type", "PostgreSQL",
		"host", cfg.Database.Postgres.Host,
		"port", cfg.Database.Postgres.Port,
		"database", cfg.Database.Postgres.Database,
		"user", cfg.Database.Postgres.User)

	// Start health server for snoodev probes
	// Port can be configured via ADMIN_PORT environment variable (default: 6060)
	go func() {
		adminPort := os.Getenv("ADMIN_PORT")
		if adminPort == "" {
			adminPort = "6060"
		}

		healthMux := http.NewServeMux()
		healthMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})
		healthMux.HandleFunc("/readiness", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("READY"))
		})

		logger.Info("Starting health server", "port", adminPort)
		if err := http.ListenAndServe(":"+adminPort, healthMux); err != nil {
			logger.Error("Health server failed", "error", err)
		}
	}()

	logger.Info("gRPC server starting", "address", listener.Addr().String())
	if err := grpcServer.Serve(listener); err != nil {
		return fmt.Errorf("failed to serve gRPC server: %w", err)
	}

	return nil
}
