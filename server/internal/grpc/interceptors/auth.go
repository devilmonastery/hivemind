package interceptors

import (
	"context"
	"errors"
	"log"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/devilmonastery/hivemind/internal/auth"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
)

// UserContextKey is the key for storing user info in context
type contextKey string

const UserContextKey contextKey = "user"

// UserContext contains authenticated user information
type UserContext struct {
	UserID      string
	Username    string
	DisplayName string
	Picture     string
	Timezone    string
	Role        string
	TokenID     string
}

// AuthInterceptor handles authentication for gRPC requests
type AuthInterceptor struct {
	jwtManager *auth.JWTManager
	tokenRepo  repositories.TokenRepository
	// Methods that don't require authentication
	publicMethods map[string]bool
	// Method prefixes that don't require authentication (e.g., "/grpc." for infrastructure)
	publicPrefixes []string
}

// NewAuthInterceptor creates a new auth interceptor
func NewAuthInterceptor(
	jwtManager *auth.JWTManager,
	tokenRepo repositories.TokenRepository,
) *AuthInterceptor {
	return &AuthInterceptor{
		jwtManager: jwtManager,
		tokenRepo:  tokenRepo,
		publicMethods: map[string]bool{
			"/hivemind.auth.v1.AuthService/AuthenticateLocal": true,
			"/hivemind.auth.v1.AuthService/GetOAuthConfig":    true,
			"/hivemind.auth.v1.AuthService/ExchangeAuthCode":  true,
			"/hivemind.auth.v1.AuthService/LoginWithOIDC":     true,
			"/hivemind.auth.v1.AuthService/RefreshToken":      true, // Allow refresh with expired token
			"/hivemind.auth.v1.AuthService/RefreshOAuthToken": true, // Deprecated but kept for compatibility
		},
		publicPrefixes: []string{
			"/grpc.", // All standard gRPC infrastructure methods (health, reflection, etc.)
		},
	}
}

// isPublicMethod checks if a method is publicly accessible
func (i *AuthInterceptor) isPublicMethod(method string) bool {
	// Check exact match first
	if i.publicMethods[method] {
		return true
	}

	// Check prefixes
	for _, prefix := range i.publicPrefixes {
		if strings.HasPrefix(method, prefix) {
			return true
		}
	}

	return false
}

// Unary returns a server interceptor for unary RPCs
func (i *AuthInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Skip auth for public methods
		if i.isPublicMethod(info.FullMethod) {
			return handler(ctx, req)
		}

		// Authenticate
		userCtx, err := i.authenticate(ctx)
		if err != nil {
			return nil, err
		}

		// Add user context using the auth package helper
		ctx = auth.SetUserInContext(ctx, &auth.UserContext{
			UserID:      userCtx.UserID,
			Username:    userCtx.Username,
			DisplayName: userCtx.DisplayName,
			Picture:     userCtx.Picture,
			Timezone:    userCtx.Timezone,
			Role:        userCtx.Role,
			TokenID:     userCtx.TokenID,
		})

		return handler(ctx, req)
	}
}

// Stream returns a server interceptor for streaming RPCs
func (i *AuthInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// Skip auth for public methods
		if i.isPublicMethod(info.FullMethod) {
			return handler(srv, stream)
		}

		// Authenticate
		userCtx, err := i.authenticate(stream.Context())
		if err != nil {
			return err
		}

		// Wrap stream with authenticated context
		ctx := auth.SetUserInContext(stream.Context(), &auth.UserContext{
			UserID:      userCtx.UserID,
			Username:    userCtx.Username,
			DisplayName: userCtx.DisplayName,
			Picture:     userCtx.Picture,
			Timezone:    userCtx.Timezone,
			Role:        userCtx.Role,
			TokenID:     userCtx.TokenID,
		})
		wrappedStream := &authenticatedStream{
			ServerStream: stream,
			ctx:          ctx,
		}

		return handler(srv, wrappedStream)
	}
}

// authenticate extracts and validates the JWT token
func (i *AuthInterceptor) authenticate(ctx context.Context) (*UserContext, error) {
	// Extract metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	// Get authorization header
	values := md.Get("authorization")
	if len(values) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing authorization token")
	}

	// Extract token (format: "Bearer <token>")
	authHeader := values[0]
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization format")
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")

	// Validate JWT
	claims, err := i.jwtManager.ValidateToken(token)
	if err != nil {
		log.Printf("[AUTH] Token validation failed: %v (token preview: %s...)", err, token[:min(30, len(token))])
		if errors.Is(err, auth.ErrExpiredToken) {
			return nil, status.Error(codes.Unauthenticated, "token expired")
		}
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	// Check if token is revoked
	dbToken, err := i.tokenRepo.GetByID(ctx, claims.TokenID)
	if err != nil {
		if errors.Is(err, repositories.ErrTokenNotFound) {
			return nil, status.Error(codes.Unauthenticated, "token not found")
		}
		return nil, status.Error(codes.Internal, "token lookup failed")
	}

	if dbToken == nil {
		return nil, status.Error(codes.Unauthenticated, "token not found")
	}

	if dbToken.RevokedAt != nil {
		return nil, status.Error(codes.Unauthenticated, "token has been revoked")
	}

	return &UserContext{
		UserID:      claims.UserID,
		Username:    claims.Username,
		DisplayName: claims.DisplayName,
		Picture:     claims.Picture,
		Timezone:    claims.Timezone,
		Role:        claims.Role,
		TokenID:     claims.TokenID,
	}, nil
}

// authenticatedStream wraps a grpc.ServerStream with an authenticated context
type authenticatedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *authenticatedStream) Context() context.Context {
	return s.ctx
}

// GetUserFromContext extracts user context from the request context
func GetUserFromContext(ctx context.Context) (*UserContext, error) {
	userCtx, ok := ctx.Value(UserContextKey).(*UserContext)
	if !ok {
		return nil, status.Error(codes.Internal, "user context not found")
	}
	return userCtx, nil
}
