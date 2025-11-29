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
	"github.com/devilmonastery/hivemind/internal/domain/services"
)

// UserContextKey is the key for storing user info in context
type contextKey string

const UserContextKey contextKey = "user"

// Metadata keys for Discord bot requests
const (
	MetadataKeyDiscordUserID   = "x-discord-user-id"
	MetadataKeyDiscordGuildID  = "x-discord-guild-id"
	MetadataKeyDiscordUsername = "x-discord-username"
)

// Special role identifiers
const (
	RoleBot = "bot" // Service account role for bots
)

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
	jwtManager     *auth.JWTManager
	tokenRepo      repositories.TokenRepository
	discordService *services.DiscordService
	devBotToken    string // Optional dev-only bot token (not for production)
	// Methods that don't require authentication
	publicMethods map[string]bool
	// Method prefixes that don't require authentication (e.g., "/grpc." for infrastructure)
	publicPrefixes []string
}

// NewAuthInterceptor creates a new auth interceptor
func NewAuthInterceptor(
	jwtManager *auth.JWTManager,
	tokenRepo repositories.TokenRepository,
	discordService *services.DiscordService,
	devBotToken string, // Optional: dev-only bot token
) *AuthInterceptor {
	return &AuthInterceptor{
		jwtManager:     jwtManager,
		tokenRepo:      tokenRepo,
		discordService: discordService,
		devBotToken:    devBotToken,
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

		// Add user context using both keys for compatibility
		ctx = auth.SetUserInContext(ctx, &auth.UserContext{
			UserID:      userCtx.UserID,
			Username:    userCtx.Username,
			DisplayName: userCtx.DisplayName,
			Picture:     userCtx.Picture,
			Timezone:    userCtx.Timezone,
			Role:        userCtx.Role,
			TokenID:     userCtx.TokenID,
		})

		// Also set using the interceptors' context key
		ctx = context.WithValue(ctx, UserContextKey, userCtx)

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

		// Wrap stream with authenticated context (both keys for compatibility)
		ctx := auth.SetUserInContext(stream.Context(), &auth.UserContext{
			UserID:      userCtx.UserID,
			Username:    userCtx.Username,
			DisplayName: userCtx.DisplayName,
			Picture:     userCtx.Picture,
			Timezone:    userCtx.Timezone,
			Role:        userCtx.Role,
			TokenID:     userCtx.TokenID,
		})
		ctx = context.WithValue(ctx, UserContextKey, userCtx)

		wrappedStream := &authenticatedStream{
			ServerStream: stream,
			ctx:          ctx,
		}

		return handler(srv, wrappedStream)
	}
}

// authenticate extracts and validates the JWT token or Discord user context
func (i *AuthInterceptor) authenticate(ctx context.Context) (*UserContext, error) {
	// Extract metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	// Check if this is a bot request with Discord user context
	// Bot requests have x-discord-user-id metadata AND a valid bot service token
	discordUserID := md.Get(MetadataKeyDiscordUserID)
	if len(discordUserID) > 0 && discordUserID[0] != "" {
		return i.authenticateDiscordUser(ctx, md)
	}

	// Fall back to standard JWT authentication for web/CLI
	return i.authenticateJWT(ctx, md)
}

// authenticateDiscordUser handles authentication for bot requests acting on behalf of Discord users
func (i *AuthInterceptor) authenticateDiscordUser(ctx context.Context, md metadata.MD) (*UserContext, error) {
	// SECURITY: First, authenticate the bot itself using its service token
	// Only authenticated bots can act on behalf of Discord users
	botContext, err := i.authenticateJWT(ctx, md)
	if err != nil {
		log.Printf("[AUTH] Bot authentication failed: %v", err)
		return nil, status.Error(codes.Unauthenticated, "bot must authenticate with service token")
	}

	// Verify the authenticated entity is actually a bot (has role "bot" or "service_account")
	if botContext.Role != RoleBot && botContext.Role != "service_account" {
		log.Printf("[AUTH] Non-bot user attempted to use Discord context: user_id=%s, role=%s",
			botContext.UserID, botContext.Role)
		return nil, status.Error(codes.PermissionDenied, "only bots can act on behalf of Discord users")
	}

	// Extract Discord user ID
	discordUserIDs := md.Get(MetadataKeyDiscordUserID)
	if len(discordUserIDs) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing discord user id")
	}
	discordUserID := discordUserIDs[0]

	// Extract optional guild ID and username for logging
	var guildID, username string
	if guildIDs := md.Get(MetadataKeyDiscordGuildID); len(guildIDs) > 0 {
		guildID = guildIDs[0]
	}
	if usernames := md.Get(MetadataKeyDiscordUsername); len(usernames) > 0 {
		username = usernames[0]
	}

	// Get or create Hivemind user from Discord identity
	user, err := i.discordService.GetOrCreateUserFromDiscord(
		ctx,
		discordUserID,
		username,
		nil, // TODO: Get discord_global_name from metadata if available
		nil, // TODO: Get avatar_url from metadata if available
	)
	if err != nil {
		log.Printf("[AUTH] Failed to get/create user from Discord: %v", err)
		return nil, status.Error(codes.Internal, "failed to provision user")
	}

	log.Printf("[AUTH] Bot %s acting on behalf of Discord user: discord_id=%s, user_id=%s, username=%s, guild_id=%s",
		botContext.UserID, discordUserID, user.ID, username, guildID)

	// Return UserContext with mapped Hivemind user
	return &UserContext{
		UserID:      user.ID,
		Username:    username,
		DisplayName: user.DisplayName,
		Picture:     stringPtrToString(user.AvatarURL),
		Timezone:    stringPtrToString(user.Timezone),
		Role:        string(user.Role),
		TokenID:     "", // Bot requests don't have token IDs
	}, nil
}

// authenticateJWT handles standard JWT token authentication for web/CLI
func (i *AuthInterceptor) authenticateJWT(ctx context.Context, md metadata.MD) (*UserContext, error) {
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

	// Check dev bot token FIRST (before JWT validation)
	// This allows simple string tokens for development without JWT complexity
	if i.devBotToken != "" && token == i.devBotToken {
		log.Printf("[AUTH] ⚠️  DEV MODE: Using config-based bot token (DO NOT USE IN PRODUCTION)")
		return &UserContext{
			UserID:      "bot-dev",
			Username:    "dev-bot",
			DisplayName: "Development Bot",
			Picture:     "",
			Timezone:    "UTC",
			Role:        RoleBot, // Important: Must be bot role to use Discord context
			TokenID:     "",
		}, nil
	}

	// Validate JWT (production path)
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

// stringPtrToString safely dereferences a string pointer
func stringPtrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// GetUserFromContext extracts user context from the request context
func GetUserFromContext(ctx context.Context) (*UserContext, error) {
	userCtx, ok := ctx.Value(UserContextKey).(*UserContext)
	if !ok {
		return nil, status.Error(codes.Internal, "user context not found")
	}
	return userCtx, nil
}
