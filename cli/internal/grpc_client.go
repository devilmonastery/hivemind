package cli

import (
	"fmt"

	"github.com/devilmonastery/hivemind/internal/client"
)

// NewGRPCClient creates a shared gRPC client with automatic token refresh
func NewGRPCClient() (*client.Client, error) {
	// Load config for server address
	config, err := LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Get server address from current context
	serverAddress, err := config.ServerAddress()
	if err != nil {
		return nil, fmt.Errorf("failed to get server address: %w", err)
	}

	serverName, err := config.ServerName()
	if err != nil {
		return nil, fmt.Errorf("failed to get server name: %w", err)
	}

	// Create file-based token manager
	tokenManager := NewFileCredentials()

	// Create shared client with interceptor
	grpcClient, err := client.NewClient(serverAddress, serverName, tokenManager)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}

	return grpcClient, nil
}

// NewUnauthenticatedClient creates a client without authentication (for login/register)
func NewUnauthenticatedClient() (*client.Client, error) {
	// Load config for server address
	config, err := LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Get server address from current context
	serverAddress, err := config.ServerAddress()
	if err != nil {
		return nil, fmt.Errorf("failed to get server address: %w", err)
	}

	serverName, err := config.ServerName()
	if err != nil {
		return nil, fmt.Errorf("failed to get server name: %w", err)
	}

	// Create client without token manager (nil = no auth)
	grpcClient, err := client.NewClient(serverAddress, serverName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}

	return grpcClient, nil
}
