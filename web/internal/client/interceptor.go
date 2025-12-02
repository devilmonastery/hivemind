package client

import (
	"context"
	"log/slog"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authpb "github.com/devilmonastery/hivemind/api/generated/go/authpb"
)

// SessionManager interface for accessing session data
// This avoids circular dependencies with the session package
type SessionManager interface {
	GetToken(r *http.Request) (string, error)
	GetTokenID(r *http.Request) (string, error)
	SetToken(r *http.Request, w http.ResponseWriter, token, tokenID string) error
}

// requestContextKey is the key for storing request/response in context
type contextKey string

const (
	requestKey  contextKey = "http_request"
	responseKey contextKey = "http_response"
)

// WithHTTPContext adds the HTTP request and response writer to the context
// This allows the interceptor to access the session
func WithHTTPContext(ctx context.Context, r *http.Request, w http.ResponseWriter) context.Context {
	ctx = context.WithValue(ctx, requestKey, r)
	ctx = context.WithValue(ctx, responseKey, w)
	return ctx
}

// AutoRefreshInterceptor creates a gRPC unary interceptor that automatically
// refreshes expired tokens, similar to the CLI's AuthInterceptor
type AutoRefreshInterceptor struct {
	authClient     authpb.AuthServiceClient
	sessionManager SessionManager
	log            *slog.Logger
}

// NewAutoRefreshInterceptor creates a new auto-refresh interceptor
func NewAutoRefreshInterceptor(authClient authpb.AuthServiceClient, sessionManager SessionManager) *AutoRefreshInterceptor {
	return &AutoRefreshInterceptor{
		authClient:     authClient,
		sessionManager: sessionManager,
		log:            slog.Default().With(slog.String("component", "auto_refresh_interceptor")),
	}
}

// UnaryInterceptor returns a gRPC unary client interceptor with auto-refresh
func (i *AutoRefreshInterceptor) UnaryInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		// Make the request
		err := invoker(ctx, method, req, reply, cc, opts...)

		// If we get Unauthenticated, try to refresh
		if status.Code(err) == codes.Unauthenticated {
			i.log.Info("authentication failed, attempting token refresh",
				slog.String("method", method))

			// Get HTTP request and response from context
			r, ok := ctx.Value(requestKey).(*http.Request)
			if !ok {
				i.log.Warn("no HTTP request in context, cannot refresh token",
					slog.String("method", method))
				return err
			}

			w, ok := ctx.Value(responseKey).(http.ResponseWriter)
			if !ok {
				i.log.Warn("no HTTP response writer in context, cannot refresh token",
					slog.String("method", method))
				return err
			}

			// Try to refresh the token
			if refreshErr := i.refreshToken(ctx, r, w); refreshErr == nil {
				i.log.Info("token refreshed successfully, retrying request",
					slog.String("method", method))

				// Get the new token and retry with updated auth context
				token, _ := i.sessionManager.GetToken(r)
				retryCtx := WithAuth(ctx, token)

				// Retry the request with new token
				err = invoker(retryCtx, method, req, reply, cc, opts...)
			} else {
				i.log.Error("token refresh failed",
					slog.String("method", method),
					slog.String("error", refreshErr.Error()))
				return err
			}
		}

		return err
	}
}

// refreshToken attempts to refresh the JWT using the stored token_id
func (i *AutoRefreshInterceptor) refreshToken(ctx context.Context, r *http.Request, w http.ResponseWriter) error {
	tokenID, err := i.sessionManager.GetTokenID(r)
	if err != nil || tokenID == "" {
		i.log.Warn("no token ID available for refresh",
			slog.String("error", err.Error()))
		return err
	}

	// Call RefreshToken RPC
	resp, err := i.authClient.RefreshToken(ctx, &authpb.RefreshTokenRequest{
		TokenId: tokenID,
	})
	if err != nil {
		i.log.Error("RefreshToken RPC failed",
			slog.String("error", err.Error()))
		return err
	}

	// Update session with new token (keep same token ID)
	if err := i.sessionManager.SetToken(r, w, resp.ApiToken, tokenID); err != nil {
		i.log.Error("failed to save refreshed token",
			slog.String("error", err.Error()))
		return err
	}

	i.log.Info("successfully refreshed token")
	return nil
}
