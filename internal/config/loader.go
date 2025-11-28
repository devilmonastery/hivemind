package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// expandEnvVars expands environment variables in the format ${VAR} or $VAR
// Uses Go's built-in os.ExpandEnv which is the idiomatic way to handle this
func expandEnvVars(data []byte) []byte {
	return []byte(os.ExpandEnv(string(data)))
}

// DefaultConfigPaths defines the default locations to search for configuration files
var DefaultConfigPaths = []string{
	"./config.yaml",
	"./config.yml",
	"./configs/config.yaml",
	"./configs/config.yml",
	"./configs/development.yaml",
	"/etc/hivemind/config.yaml",
	"/etc/hivemind/config.yml",
}

// Load loads the configuration from the specified file or default locations
func Load(configPath string) (*Config, error) {
	// Set default values
	config := &Config{
		Database: DatabaseConfig{
			Postgres: PostgresConfig{
				Host:     "localhost",
				Port:     5432,
					Database: "hivemind",
				User:     "postgres",
				SSLMode:  "disable",
			},
		},
		GRPC: GRPCConfig{
			Host: "localhost",
			Port: 9091,
		},
	}

	// If no config path is provided, search in default locations
	if configPath == "" {
		configPath = findConfigFile()
	}

	// Load configuration from file if it exists
	if configPath != "" && fileExists(configPath) {
		fmt.Printf("[CONFIG] Loading config from: %s\n", configPath)
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		// Expand environment variables in the config
		data = expandEnvVars(data)

		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	} else {
		fmt.Printf("[CONFIG] No config file found, using defaults\n")
	}

	// Validate configuration
	if err := validate(config); err != nil {
		return nil, err
	}

	return config, nil
} // LoadFromFile loads configuration from a specific file
func LoadFromFile(filepath string) (*Config, error) {
	return Load(filepath)
}

// LoadFromDefaults loads configuration using only defaults and environment variables
func LoadFromDefaults() (*Config, error) {
	return Load("")
}

// findConfigFile searches for a configuration file in default locations
func findConfigFile() string {
	for _, path := range DefaultConfigPaths {
		if fileExists(path) {
			return path
		}
	}
	return ""
}

// fileExists checks if a file exists and is not a directory
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// validate performs basic validation on the configuration
func validate(config *Config) error {
	// Validate PostgreSQL configuration
	if config.Database.Postgres.Host == "" {
		return fmt.Errorf("postgres host is required")
	}
	if config.Database.Postgres.Database == "" {
		return fmt.Errorf("postgres database name is required")
	}
	if config.Database.Postgres.User == "" {
		return fmt.Errorf("postgres user is required")
	}

	// Validate GRPC port is reasonable
	if config.GRPC.Port < 1 || config.GRPC.Port > 65535 {
		return fmt.Errorf("grpc.port must be between 1 and 65535")
	}

	return nil
}
