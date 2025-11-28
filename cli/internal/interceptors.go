package cli

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	authpb "github.com/devilmonastery/hivemind/api/generated/go/authpb"
)

// AuthInterceptor handles authentication and automatic token refresh
type AuthInterceptor struct {
	credentials *Credentials
}

// NewAuthInterceptor creates a new auth interceptor
func NewAuthInterceptor(creds *Credentials) *AuthInterceptor {
	return &AuthInterceptor{
		credentials: creds,
	}
}

// UnaryInterceptor returns a gRPC unary client interceptor with auto-refresh
func (a *AuthInterceptor) UnaryInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		// Add authorization header
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+a.credentials.AccessToken)

		// Make the request
		err := invoker(ctx, method, req, reply, cc, opts...)

		// If we get Unauthenticated, try to refresh
		if status.Code(err) == codes.Unauthenticated {
			// Try to refresh (server has the OAuth refresh token)
			if refreshErr := a.refreshToken(ctx); refreshErr == nil {
				// Refresh succeeded - retry the request with new token
				// Create a fresh context with the new authorization header
				retryCtx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+a.credentials.AccessToken)
				err = invoker(retryCtx, method, req, reply, cc, opts...)
			} else {
				// Refresh failed - return a clear message to re-login with debug info
				msg := fmt.Sprintf("authentication failed: your session has expired\n\nRefresh error: %v\n\n", refreshErr)
				if a.credentials.Provider != "" {
					msg += fmt.Sprintf("Please run 'hivemind auth login --provider %s' to re-authenticate", a.credentials.Provider)
				} else {
					msg += "Please run 'hivemind auth login' to authenticate"
				}
				return fmt.Errorf("%s", msg)
			}
		}

		// If still unauthenticated after refresh attempt, give clear message
		if status.Code(err) == codes.Unauthenticated {
			msg := "authentication failed\n\n"
			if a.credentials.Provider != "" {
				msg += fmt.Sprintf("Please run 'hivemind auth login --provider %s' to authenticate", a.credentials.Provider)
			} else {
				msg += "Please run 'hivemind auth login' to authenticate"
			}
			return fmt.Errorf("%s", msg)
		}

		return err
	}
}

// StreamInterceptor returns a gRPC stream client interceptor.
// TODO: This currently does NOT handle token refresh on stream errors
func (a *AuthInterceptor) StreamInterceptor() grpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *grpc.StreamDesc,
		cc *grpc.ClientConn,
		method string,
		streamer grpc.Streamer,
		opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		// Add authorization header
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+a.credentials.AccessToken)

		return streamer(ctx, desc, cc, method, opts...)
	}
}

// refreshToken refreshes the access token using the server-side stored OAuth refresh token
func (a *AuthInterceptor) refreshToken(ctx context.Context) error {
	// Create an unauthenticated connection to call RefreshToken
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	serverAddress, err := config.ServerAddress()
	if err != nil {
		return fmt.Errorf("failed to get server address: %w", err)
	}

	conn, err := grpc.NewClient(
		serverAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer conn.Close()

	// Call RefreshToken RPC (server looks up OAuth refresh token by token_id)
	authClient := authpb.NewAuthServiceClient(conn)

	resp, err := authClient.RefreshToken(ctx, &authpb.RefreshTokenRequest{
		TokenId: a.credentials.TokenID,
	})
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}

	// Update credentials with new token
	a.credentials.AccessToken = resp.ApiToken
	a.credentials.ExpiresAt = resp.ExpiresAt.AsTime()

	// Save updated credentials
	if err := SaveCredentials(a.credentials); err != nil {
		return fmt.Errorf("failed to save refreshed credentials: %w", err)
	}

	return nil
}
