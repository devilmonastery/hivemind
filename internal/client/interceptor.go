package client

import (
	"context"
	"crypto/tls"
	"log/slog"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	authpb "github.com/devilmonastery/hivemind/api/generated/go/authpb"
)

// AuthInterceptor handles automatic token refresh for gRPC calls
type AuthInterceptor struct {
	tokenManager  TokenManager
	serverAddress string // For creating unauthenticated connection to refresh
}

// NewAuthInterceptor creates a new auth interceptor
func NewAuthInterceptor(tokenManager TokenManager, serverAddress string) *AuthInterceptor {
	return &AuthInterceptor{
		tokenManager:  tokenManager,
		serverAddress: serverAddress,
	}
}

// Unary returns a gRPC unary client interceptor with auto-refresh
func (a *AuthInterceptor) Unary() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		// Get current token
		token, err := a.tokenManager.GetToken()
		if err != nil {
			return err
		}

		// Add authorization header
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)

		// Try the call
		err = invoker(ctx, method, req, reply, cc, opts...)

		// If unauthenticated, try to refresh and retry
		if status.Code(err) == codes.Unauthenticated {
			slog.Info("token expired, attempting refresh")
			slog.Debug("old token info", slog.String("token_prefix", token[:min(30, len(token))]))

			refreshErr := a.refreshToken(ctx)
			if refreshErr != nil {
				slog.Error("token refresh failed", slog.String("error", refreshErr.Error()))
				return err // Return original error
			}

			// Get the new token
			newToken, tokenErr := a.tokenManager.GetToken()
			if tokenErr != nil {
				return tokenErr
			}
			slog.Debug("new token info", slog.String("token_prefix", newToken[:min(30, len(newToken))]))

			// Retry with new token (use NewOutgoingContext to replace the old auth header)
			md := metadata.Pairs("authorization", "Bearer "+newToken)
			retryCtx := metadata.NewOutgoingContext(context.Background(), md)
			slog.Debug("retrying request with refreshed token")
			err = invoker(retryCtx, method, req, reply, cc, opts...)
			slog.Debug("retry result", slog.Any("error", err))
		}

		return err
	}
}

// Stream returns a gRPC stream client interceptor
// TODO: This currently does NOT handle token refresh on stream errors
func (a *AuthInterceptor) Stream() grpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *grpc.StreamDesc,
		cc *grpc.ClientConn,
		method string,
		streamer grpc.Streamer,
		opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		// Get current token
		token, err := a.tokenManager.GetToken()
		if err != nil {
			return nil, err
		}

		// Add authorization header
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)

		return streamer(ctx, desc, cc, method, opts...)
	}
}

// refreshToken refreshes the access token using the server-side stored OAuth refresh token
func (a *AuthInterceptor) refreshToken(ctx context.Context) error {
	// Get token ID for refresh
	tokenID, err := a.tokenManager.GetTokenID()
	if err != nil || tokenID == "" {
		return err
	}

	// Create connection options with proper TLS (same logic as NewClient)
	opts := []grpc.DialOption{
		// Keep connections alive to prevent EOF errors
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             3 * time.Second,
			PermitWithoutStream: true,
		}),
	}

	// Auto-detect TLS based on server address
	if isLocalhost(a.serverAddress) {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		// Extract server name for SNI (remove port if present)
		serverName := a.serverAddress
		if idx := strings.LastIndex(a.serverAddress, ":"); idx != -1 {
			serverName = a.serverAddress[:idx]
		}

		// Use system certificates for TLS with proper SNI
		creds := credentials.NewTLS(&tls.Config{
			ServerName: serverName,
			MinVersion: tls.VersionTLS12,
		})
		opts = append(opts, grpc.WithTransportCredentials(creds))
	}

	// Create an unauthenticated connection to call RefreshToken
	conn, err := grpc.NewClient(a.serverAddress, opts...)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Call RefreshToken RPC
	authClient := authpb.NewAuthServiceClient(conn)
	resp, err := authClient.RefreshToken(ctx, &authpb.RefreshTokenRequest{
		TokenId: tokenID,
	})
	if err != nil {
		return err
	}

	// Save the new token (keep same token ID)
	err = a.tokenManager.SaveToken(resp.ApiToken, tokenID)
	if err != nil {
		return err
	}

	slog.Info("successfully refreshed token")
	return nil
}
