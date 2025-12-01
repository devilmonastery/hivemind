package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/devilmonastery/hivemind/bot/internal/bot"
	"github.com/devilmonastery/hivemind/bot/internal/config"
)

func newRunCommand() *cobra.Command {
	var (
		configPath string
		logLevel   string
		logFormat  string
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start the Discord bot",
		Long:  `Start the Discord bot and connect to Discord API. The bot will listen for commands and interactions.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration first
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Use config file logging settings if not overridden by flags
			if !cmd.Flags().Changed("log-level") && cfg.Logging.Level != "" {
				logLevel = cfg.Logging.Level
			}
			if !cmd.Flags().Changed("log-format") && cfg.Logging.Format != "" {
				logFormat = cfg.Logging.Format
			}

			// Initialize logger
			log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
				Level: parseLogLevel(logLevel),
			}))
			if logFormat == "text" {
				log = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
					Level: parseLogLevel(logLevel),
				}))
			}

			// Validate required configuration
			if cfg.Backend.ServiceToken == "" {
				return fmt.Errorf("backend.service_token is required - the bot cannot authenticate with the server without it. See example_bot.yaml for instructions on generating a service token")
			}

			log.Info("starting hivemind bot",
				slog.String("version", "0.1.0"),
				slog.String("config", configPath),
			)

			// Create bot instance
			b, err := bot.New(cfg, log)
			if err != nil {
				return fmt.Errorf("failed to create bot: %w", err)
			}

			// Start the bot
			if err := b.Start(); err != nil {
				return fmt.Errorf("failed to start bot: %w", err)
			}

			log.Info("bot started successfully")

			// Wait for interrupt signal
			stop := make(chan os.Signal, 1)
			signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
			<-stop

			log.Info("shutting down bot")

			// Graceful shutdown
			if err := b.Stop(context.Background()); err != nil {
				log.Error("error during shutdown", slog.String("error", err.Error()))
				return err
			}

			log.Info("bot stopped")
			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "configs/dev-bot.yaml", "path to configuration file")
	cmd.Flags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	cmd.Flags().StringVar(&logFormat, "log-format", "json", "log format (json, text)")

	return cmd
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
