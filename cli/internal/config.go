package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Context represents a named configuration context (like kubectl contexts)
type Context struct {
	Server struct {
		Address string `yaml:"address"`
		Name    string `yaml:"name"`
		Port    int    `yaml:"port"`
	} `yaml:"server"`
	Rendering struct {
		Theme string `yaml:"theme"`
	} `yaml:"rendering"`
}

// Config represents the CLI configuration with multiple contexts
type Config struct {
	CurrentContext string              `yaml:"current-context"`
	Contexts       map[string]*Context `yaml:"contexts"`
}

// DefaultConfig returns the default configuration with "dev" and "prod" contexts
func DefaultConfig() *Config {
	devContext := &Context{}
	devContext.Server.Address = "localhost"
	devContext.Server.Name = ""
	devContext.Server.Port = 4153
	devContext.Rendering.Theme = "auto"

	prodContext := &Context{}
	prodContext.Server.Address = "hivemind.dontberu.de"
	prodContext.Server.Name = "hivemind.dontberu.de"
	prodContext.Server.Port = 443
	prodContext.Rendering.Theme = "auto"

	return &Config{
		CurrentContext: "dev",
		Contexts: map[string]*Context{
			"dev":  devContext,
			"prod": prodContext,
		},
	}
}

// GetCurrentContext returns the current active context
func (c *Config) GetCurrentContext() (*Context, error) {
	if c.CurrentContext == "" {
		return nil, fmt.Errorf("no current context set")
	}

	ctx, ok := c.Contexts[c.CurrentContext]
	if !ok {
		return nil, fmt.Errorf("current context %q not found", c.CurrentContext)
	}

	return ctx, nil
}

// SetCurrentContext sets the current active context
func (c *Config) SetCurrentContext(name string) error {
	if _, ok := c.Contexts[name]; !ok {
		return fmt.Errorf("context %q does not exist", name)
	}
	c.CurrentContext = name
	return nil
}

// AddContext adds or updates a context
func (c *Config) AddContext(name string, ctx *Context) {
	if c.Contexts == nil {
		c.Contexts = make(map[string]*Context)
	}
	c.Contexts[name] = ctx
}

// DeleteContext removes a context
func (c *Config) DeleteContext(name string) error {
	if name == c.CurrentContext {
		return fmt.Errorf("cannot delete current context %q", name)
	}
	if _, ok := c.Contexts[name]; !ok {
		return fmt.Errorf("context %q does not exist", name)
	}
	delete(c.Contexts, name)
	return nil
}

// GetConfigPath returns the path to the config file
func GetConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(homeDir, ".hivemind"), nil
}

// LoadConfig loads configuration from ~/.hivemind file
func LoadConfig() (*Config, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	// If config file doesn't exist, create it with defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultConfig := DefaultConfig()
		if err := SaveConfig(defaultConfig); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		return defaultConfig, nil
	}

	// Read existing config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Ensure we have a valid current context
	if config.CurrentContext == "" && len(config.Contexts) > 0 {
		// Pick the first context as default
		for name := range config.Contexts {
			config.CurrentContext = name
			break
		}
	}

	return &config, nil
}

// SaveConfig saves configuration to ~/.hivemind file
func SaveConfig(config *Config) error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// ServerAddress returns the full server address for the current context
func (c *Config) ServerAddress() (string, error) {
	ctx, err := c.GetCurrentContext()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d", ctx.Server.Address, ctx.Server.Port), nil
}

// ServerAddress returns the full server address for this context
func (ctx *Context) ServerAddress() string {
	return fmt.Sprintf("%s:%d", ctx.Server.Address, ctx.Server.Port)
}

// ServerName returns the remote server name for the current context
func (c *Config) ServerName() (string, error) {
	ctx, err := c.GetCurrentContext()
	if err != nil {
		return "", err
	}
	return ctx.Server.Name, nil
}

// ServerName returns the remote server name for this context
func (ctx *Context) ServerName() string {
	return ctx.Server.Name
}
