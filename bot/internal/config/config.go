package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the bot configuration
type Config struct {
	Bot      BotConfig      `yaml:"bot"`
	Backend  BackendConfig  `yaml:"backend"`
	Logging  LoggingConfig  `yaml:"logging"`
	Features FeaturesConfig `yaml:"features"`
}

// BotConfig holds Discord bot specific configuration
type BotConfig struct {
	Token         string `yaml:"token"`
	ApplicationID string `yaml:"application_id"`
}

// BackendConfig holds backend server connection details
type BackendConfig struct {
	GRPCHost   string `yaml:"grpc_host"`
	GRPCPort   int    `yaml:"grpc_port"`
	TLSEnabled bool   `yaml:"tls_enabled"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

// FeaturesConfig holds feature flags
type FeaturesConfig struct {
	EnableQuotes bool `yaml:"enable_quotes"`
	EnableWiki   bool `yaml:"enable_wiki"`
	EnableNotes  bool `yaml:"enable_notes"`
	MaxNoteSize  int  `yaml:"max_note_size"`
	MaxWikiSize  int  `yaml:"max_wiki_size"`
}

// Load reads the configuration from a YAML file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables
	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate required fields
	if cfg.Bot.Token == "" {
		return nil, fmt.Errorf("bot.token is required")
	}
	if cfg.Bot.ApplicationID == "" {
		return nil, fmt.Errorf("bot.application_id is required")
	}

	// Set defaults
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "json"
	}
	if cfg.Features.MaxNoteSize == 0 {
		cfg.Features.MaxNoteSize = 10000
	}
	if cfg.Features.MaxWikiSize == 0 {
		cfg.Features.MaxWikiSize = 50000
	}

	return &cfg, nil
}
