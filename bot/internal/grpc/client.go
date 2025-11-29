package grpc

import (
	"fmt"

	"github.com/devilmonastery/hivemind/bot/internal/config"
	"github.com/devilmonastery/hivemind/internal/client"
)

// NewClient creates a gRPC client for the bot to communicate with the backend
func NewClient(cfg *config.Config) (*client.Client, error) {
	// Build server address from config
	serverAddress := fmt.Sprintf("%s:%d", cfg.Backend.GRPCHost, cfg.Backend.GRPCPort)

	// Create token manager if service token is provided
	var tokenManager *StaticTokenManager
	if cfg.Backend.ServiceToken != "" {
		tokenManager = NewStaticTokenManager(cfg.Backend.ServiceToken)
	}

	// Create gRPC client with or without authentication
	grpcClient, err := client.NewClient(serverAddress, cfg.Backend.GRPCHost, tokenManager)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}

	return grpcClient, nil
}
