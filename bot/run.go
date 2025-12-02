package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"

	"github.com/devilmonastery/hivemind/bot/internal/bot"
	"github.com/devilmonastery/hivemind/bot/internal/config"
)

func newRunCommand() *cobra.Command {
	var (
		configPath  string
		logLevel    string
		logFormat   string
		metricsPort int
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

			// Set as global default logger so slog.Default() returns this configured logger
			slog.SetDefault(log)

			// Start metrics server on port 9100
			go func() {
				metricsMux := http.NewServeMux()
				metricsMux.Handle("/metrics", promhttp.Handler())
				metricsMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("OK"))
				})

				metricsAddr := ":9100"
				log.Info("starting metrics server", "address", metricsAddr)
				if metricsErr := http.ListenAndServe(metricsAddr, metricsMux); metricsErr != nil {
					log.Error("metrics server failed", "error", metricsErr)
				}
			}() // Validate required configuration
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

	cmd.Flags().StringVar(&configPath, "config", "", "Path to config file")
	cmd.Flags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	cmd.Flags().StringVar(&logFormat, "log-format", "json", "Log format (text, json)")
	cmd.Flags().IntVar(&metricsPort, "metrics-port", 0, "Metrics server port (default: 9100)")

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
