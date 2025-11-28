package cli

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/devilmonastery/hivemind/internal/client"
	"github.com/devilmonastery/hivemind/internal/pkg/logger"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const cliContextKey contextKey = "cliContext"

// CliContext holds shared CLI context
type CliContext struct {
	Config     *Config
	GRPCClient *client.Client
	Logger     *slog.Logger
}

// Global logging flags
var (
	logLevel      string
	logFile       string
	logToStderr   bool
	alsoLogStderr bool
	logFormat     string
)

// NewRootCommand creates the root cobra command
func NewRootCommand() *cobra.Command {
	var ctx CliContext

	rootCmd := &cobra.Command{
		Use:           "hivemind",
		Short:         "CLI for managing knowledge with Hivemind",
		Long:          `A command line interface for managing knowledge via the Hivemind gRPC API.`,
		SilenceUsage:  true, // Don't print usage on errors
		SilenceErrors: true, // Don't print errors (main.go handles it)
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Setup logging first
			if err := setupLogging(); err != nil {
				return fmt.Errorf("failed to setup logging: %w", err)
			}

			// Create logger for this context
			ctx.Logger = slog.Default().With("component", "cli")
			ctx.Logger.Debug("CLI started", "command", cmd.Name())

			// Skip connection setup for auth and config commands
			requiresAuth := cmd.Name() != "auth" && cmd.Parent().Name() != "auth" &&
				cmd.Name() != "config" && cmd.Parent().Name() != "config"

			// Connect to gRPC server with or without authentication using new client architecture
			var grpcClient *client.Client
			var err error

			if requiresAuth {
				// Use authenticated client with token manager
				grpcClient, err = NewGRPCClient()
				if err != nil {
					return fmt.Errorf("authentication required: %w\nPlease run 'hivemind auth login' first", err)
				}
			} else {
				// Use unauthenticated client (nil token manager)
				config, err := LoadConfig()
				if err != nil {
					return fmt.Errorf("failed to load config: %w", err)
				}
				serverAddress, err := config.ServerAddress()
				if err != nil {
					return fmt.Errorf("failed to get server address: %w", err)
				}
				serverName, err := config.ServerName()
				if err != nil {
					return fmt.Errorf("failed to get server name: %w", err)
				}

				grpcClient, err = client.NewClient(serverAddress, serverName, nil)
				if err != nil {
					return fmt.Errorf("failed to create client: %w", err)
				}
			}

			ctx.GRPCClient = grpcClient // Store context in command
			cmd.SetContext(context.WithValue(cmd.Context(), cliContextKey, &ctx))

			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			// Clean up connection
			if ctx.GRPCClient != nil {
				return ctx.GRPCClient.Close()
			}
			return nil
		},
	}

	// Add subcommands
	rootCmd.AddCommand(newAuthCommand())
	rootCmd.AddCommand(newConfigCommand())

	// Add logging flags
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "warn",
		"Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "",
		"Log file path (if specified, logs to file instead of stderr)")
	rootCmd.PersistentFlags().BoolVar(&logToStderr, "logtostderr", false,
		"Log to stderr (default behavior unless --log-file specified)")
	rootCmd.PersistentFlags().BoolVar(&alsoLogStderr, "alsologtostderr", false,
		"Log to both file and stderr")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "text",
		"Log format (text, json)")

	return rootCmd
}

// setupLogging configures the global logger based on CLI flags
func setupLogging() error {
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

// getCliContext extracts the CLI context from the command context
func getCliContext(cmd *cobra.Command) *CliContext {
	return cmd.Context().Value(cliContextKey).(*CliContext)
}
