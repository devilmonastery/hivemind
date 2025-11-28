package client

import (
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	authpb "github.com/devilmonastery/hivemind/api/generated/go/authpb"
)

// Client wraps gRPC clients with automatic token refresh
type Client struct {
	conn         *grpc.ClientConn
	tokenManager TokenManager
	authClient   authpb.AuthServiceClient
}

// NewClient creates a new gRPC client with automatic token refresh
// If tokenManager is nil, no auth interceptor will be added (useful for creating a base connection).
// if serverName is not empty, it is used as the remote peer name.
func NewClient(serverAddress string, serverName string, tokenManager TokenManager) (*Client, error) {
	opts := []grpc.DialOption{
		// Keep connections alive to prevent EOF errors
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second, // Send keepalive ping every 10 seconds
			Timeout:             3 * time.Second,  // Wait 3 seconds for ping ack
			PermitWithoutStream: true,             // Allow pings when no active streams
		}),
	}

	// Auto-detect TLS based on server address
	// Use TLS for production hosts, insecure for localhost
	if isLocalhost(serverAddress) {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		// Extract server name for SNI (remove port if present)
		if serverName == "" {
			if idx := strings.LastIndex(serverAddress, ":"); idx != -1 {
				serverName = serverAddress[:idx]
			}
		}

		// Use system certificates for TLS with proper SNI
		creds := credentials.NewTLS(&tls.Config{
			ServerName: serverName, // Required for SNI with virtual hosting
			MinVersion: tls.VersionTLS12,
		})
		opts = append(opts, grpc.WithTransportCredentials(creds))
	}

	// Only add interceptor if we have a token manager
	if tokenManager != nil {
		interceptor := NewAuthInterceptor(tokenManager, serverAddress)
		opts = append(opts,
			grpc.WithUnaryInterceptor(interceptor.Unary()),
			grpc.WithStreamInterceptor(interceptor.Stream()),
		)
	}

	// Create connection with options
	// Note: gRPC internally manages connection pooling, so creating multiple
	// clients to the same address will reuse underlying connections
	conn, err := grpc.NewClient(serverAddress, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	return &Client{
		conn:         conn,
		tokenManager: tokenManager,
		authClient:   authpb.NewAuthServiceClient(conn),
	}, nil
}

// isLocalhost checks if an address is localhost/127.0.0.1 or a cluster-internal address
func isLocalhost(address string) bool {
	lower := strings.ToLower(address)
	return strings.Contains(lower, "localhost") ||
		strings.Contains(lower, "127.0.0.1") ||
		strings.HasPrefix(lower, "::1") ||
		strings.HasPrefix(lower, "[::1]") ||
		// Kubernetes service names (no dots = internal)
		!strings.Contains(address, ".")
}

// Close closes the gRPC connection
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// AuthClient returns the auth service client
func (c *Client) AuthClient() authpb.AuthServiceClient {
	return c.authClient
}

// TokenManager returns the token manager (useful for handlers)
func (c *Client) TokenManager() TokenManager {
	return c.tokenManager
}

// Conn returns the underlying gRPC connection
func (c *Client) Conn() *grpc.ClientConn {
	return c.conn
}
