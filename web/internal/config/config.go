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

// WebServerConfig represents the web server configuration
type WebServerConfig struct {
	Server    HTTPServer      `yaml:"server"`
	GRPC      GRPCTarget      `yaml:"grpc"`
	OAuth     OAuthConfig     `yaml:"oauth"`
	Session   SessionConfig   `yaml:"session"`
	Templates TemplatesConfig `yaml:"templates"`
	Logging   LoggingConfig   `yaml:"logging"`
}

// HTTPServer holds HTTP server configuration
type HTTPServer struct {
	Host        string `yaml:"host" default:"localhost"`
	Port        int    `yaml:"port" default:"8080"`
	MetricsPort int    `yaml:"metrics_port" default:"0"` // 0 means Port+10
}

// GRPCTarget holds gRPC backend connection info
type GRPCTarget struct {
	Address string `yaml:"address" default:"localhost:9091"`
}

// OAuthConfig holds OAuth redirect configuration
type OAuthConfig struct {
	RedirectURI string `yaml:"redirect_uri" default:"http://localhost:8080/auth/callback"`
}

// SessionConfig holds session configuration
type SessionConfig struct {
	Secret string `yaml:"secret"` // 32-byte base64-encoded or hex string
}

// TemplatesConfig holds template loading configuration
type TemplatesConfig struct {
	Path string `yaml:"path" default:"web/templates"` // Path to templates directory
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level" default:"info"`  // Log level: debug, info, warn, error
	Format string `yaml:"format" default:"json"` // Log format: json, text
}

// DefaultConfigPaths defines the default locations to search for web configuration files
var DefaultConfigPaths = []string{
	"./config.yaml",
	"./config.yml",
	"./configs/web.yaml",
	"./configs/web.yml",
	"./configs/development.yaml",
	"/etc/hivemind/config.yaml",
	"/etc/hivemind/config.yml",
}

// Load loads the web server configuration from the specified file or default locations
func Load(configPath string) (*WebServerConfig, error) {
	// Set default values
	config := &WebServerConfig{
		Server: HTTPServer{
			Host: "localhost",
			Port: 8080,
		},
		GRPC: GRPCTarget{
			Address: "localhost:9091",
		},
		OAuth: OAuthConfig{
			RedirectURI: "http://localhost:8080/auth/callback",
		},
		Templates: TemplatesConfig{
			Path: "web/templates",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	// If no config path is provided, search in default locations
	if configPath == "" {
		configPath = findConfigFile()
	}

	// Load configuration from file if it exists
	if configPath != "" && fileExists(configPath) {
		fmt.Printf("[CONFIG] Loading web config from: %s\n", configPath)
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		// Expand environment variables in the config
		data = expandEnvVars(data)

		// Parse YAML
		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	} else {
		fmt.Printf("[CONFIG] No web config file found, using defaults\n")
	}

	// Environment variables take precedence (infrared best practice)
	if grpcAddr := os.Getenv("GRPC_ADDRESS"); grpcAddr != "" {
		config.GRPC.Address = grpcAddr
		fmt.Printf("[CONFIG] Using GRPC address from environment: %s\n", grpcAddr)
	}

	// Validate
	if err := validate(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
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

// validate performs basic validation on the web configuration
func validate(config *WebServerConfig) error {
	// Validate HTTP port is reasonable
	if config.Server.Port < 1 || config.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535")
	}

	// Validate gRPC address is not empty
	if config.GRPC.Address == "" {
		return fmt.Errorf("grpc.address cannot be empty")
	}

	return nil
}
