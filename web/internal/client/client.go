package client

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	authpb "github.com/devilmonastery/hivemind/api/generated/go/authpb"
)

// Client wraps gRPC clients for the web service
type Client struct {
	conn       *grpc.ClientConn
	authClient authpb.AuthServiceClient
}

// NewClient creates a new gRPC client for the web service
func NewClient(target string, opts ...grpc.DialOption) (*Client, error) {
	// Add default options
	defaultOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	// Append any additional options (like interceptors)
	allOpts := append(defaultOpts, opts...)

	conn, err := grpc.NewClient(target, allOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	return &Client{
		conn:       conn,
		authClient: authpb.NewAuthServiceClient(conn),
	}, nil
}

// Close closes the gRPC connection
func (c *Client) Close() error {
	return c.conn.Close()
}

// AuthClient returns the auth service client
func (c *Client) AuthClient() authpb.AuthServiceClient {
	return c.authClient
}

// WithAuth returns a context with authentication metadata
func WithAuth(ctx context.Context, token string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
}
