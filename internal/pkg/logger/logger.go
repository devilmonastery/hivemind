package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	Level         slog.Level
	LogFile       string
	LogToStderr   bool
	AlsoLogStderr bool
	Format        string // "json" or "text"
}

// SetupLogger creates a configured slog logger
func SetupLogger(cfg Config) (*slog.Logger, error) {
	var writers []io.Writer

	// File output (default)
	if cfg.LogFile != "" {
		// Ensure directory exists
		dir := filepath.Dir(cfg.LogFile)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}

		file, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, err
		}
		writers = append(writers, file)
	}

	// Stderr output
	if cfg.LogToStderr || cfg.AlsoLogStderr {
		writers = append(writers, os.Stderr)
	}

	// Create handler based on format
	var handler slog.Handler
	writer := io.MultiWriter(writers...)

	opts := &slog.HandlerOptions{
		Level:     cfg.Level,
		AddSource: true, // Always add source file and line number
	}

	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(writer, opts)
	} else {
		handler = slog.NewTextHandler(writer, opts)
	}

	return slog.New(handler), nil
}

// ParseLevel converts a string to slog.Level
func ParseLevel(level string) slog.Level {
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

// CLI-specific convenience functions
func WithCommand(logger *slog.Logger, cmd string) *slog.Logger {
	return logger.With("command", cmd)
}

func WithUser(logger *slog.Logger, userID string) *slog.Logger {
	return logger.With("user_id", userID)
}

func WithRequest(logger *slog.Logger, requestID string) *slog.Logger {
	return logger.With("request_id", requestID)
}

// Server-specific convenience functions
func WithGRPCMethod(logger *slog.Logger, method string) *slog.Logger {
	return logger.With("grpc_method", method)
}

func WithHTTPRequest(logger *slog.Logger, method, path string) *slog.Logger {
	return logger.With("http_method", method, "http_path", path)
}

func WithDuration(logger *slog.Logger, duration time.Duration) *slog.Logger {
	return logger.With("duration_ms", duration.Milliseconds())
}

// GetDefaultLogFile returns the default log file path for a component
func GetDefaultLogFile(component string) string {
	configDir, _ := os.UserConfigDir()
	if configDir == "" {
		configDir = "."
	}
	logDir := filepath.Join(configDir, "hivemind")
	return filepath.Join(logDir, component+".log")
}
