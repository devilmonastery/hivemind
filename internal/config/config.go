package config

import (
	"fmt"
	"time"
)

// Config represents the application configuration
type Config struct {
	Database    DatabaseConfig `yaml:"database"`
	GRPC        GRPCConfig     `yaml:"grpc"`
	Auth        AuthConfig     `yaml:"auth"`
	Environment string         `yaml:"environment" default:"local"`       // local, dev, prod
	VaultPath   string         `yaml:"vault_path" default:"/mnt/secrets"` // Path where Vault secrets are mounted
}

// ServerConfig holds general server configuration
type ServerConfig struct {
	Host string `yaml:"host" default:"localhost"`
	Port int    `yaml:"port" default:"8080"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Postgres PostgresConfig `yaml:"postgres"`
}

// PostgresConfig holds PostgreSQL-specific configuration
type PostgresConfig struct {
	Host            string `yaml:"host" default:"localhost"`
	Port            int    `yaml:"port" default:"5432"`
	Database        string `yaml:"database" default:"hivemind"`
	User            string `yaml:"user" default:"postgres"`
	Password        string `yaml:"password"`
	SSLMode         string `yaml:"sslmode" default:"disable"` // disable, require, verify-ca, verify-full
	UseVaultSecrets bool   `yaml:"use_vault_secrets"`         // If true, use Vault for credentials
	VaultSecretPath string `yaml:"vault_secret_path"`         // Path to Vault secret (e.g., secret/hivemind/pga-hivemind/service_hivemind_rw)
}

// GRPCConfig holds gRPC server configuration
type GRPCConfig struct {
	Host string `yaml:"host" default:"localhost"`
	Port int    `yaml:"port" default:"9091"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWT           JWTConfig        `yaml:"jwt"`
	EncryptionKey string           `yaml:"encryption_key"` // AES-256 key for encrypting refresh tokens (32 bytes base64)
	DevBotToken   string           `yaml:"dev_bot_token"`  // Optional: Static bot token for development only (DO NOT USE IN PRODUCTION)
	Providers     []ProviderConfig `yaml:"providers"`
}

// JWTConfig holds JWT token configuration
type JWTConfig struct {
	SigningKey string        `yaml:"signing_key"`             // Secret key for signing JWTs
	Lifetime   time.Duration `yaml:"lifetime" default:"168h"` // Default 7 days
}

// ProviderConfig holds OIDC provider configuration
type ProviderConfig struct {
	Name           string   `yaml:"name"`                            // "google", "github", "okta", etc.
	Type           string   `yaml:"type"`                            // Same as name for now, extensible
	ClientID       string   `yaml:"client_id"`                       // OAuth client ID (required)
	ClientSecret   string   `yaml:"client_secret,omitempty"`         // OAuth client secret
	Issuer         string   `yaml:"issuer,omitempty"`                // OIDC issuer URL (for discovery)
	Scopes         []string `yaml:"scopes,omitempty"`                // OAuth scopes (e.g., ["openid", "email", "profile"])
	AllowedDomains []string `yaml:"allowed_domains,omitempty"`       // Email domain allowlist
	AllowedUsers   []string `yaml:"allowed_users,omitempty"`         // Individual user email allowlist
	AllowedOrgs    []string `yaml:"allowed_organizations,omitempty"` // GitHub orgs, etc.
	AutoProvision  bool     `yaml:"auto_provision" default:"true"`   // Auto-create users on first login
}

// ConnectionString returns the PostgreSQL connection string
func (p *PostgresConfig) ConnectionString() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		p.Host, p.Port, p.User, p.Password, p.Database, p.SSLMode)
}
