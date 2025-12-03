package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	authpb "github.com/devilmonastery/hivemind/api/generated/go/authpb"
	userpb "github.com/devilmonastery/hivemind/api/generated/go/userpb"
	"github.com/devilmonastery/hivemind/internal/auth"
	"github.com/devilmonastery/hivemind/internal/auth/oidc"
	"github.com/devilmonastery/hivemind/internal/config"
	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
	"github.com/devilmonastery/hivemind/internal/pkg/idgen"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserNotActive      = errors.New("user account is not active")
)

// AuthHandler handles authentication operations
type AuthHandler struct {
	authpb.UnimplementedAuthServiceServer
	userRepo        repositories.UserRepository
	tokenRepo       repositories.TokenRepository
	sessionRepo     repositories.SessionRepository
	discordUserRepo repositories.DiscordUserRepository
	jwtManager      *auth.JWTManager
	config          *config.Config
	log             *slog.Logger
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(
	userRepo repositories.UserRepository,
	tokenRepo repositories.TokenRepository,
	sessionRepo repositories.SessionRepository,
	discordUserRepo repositories.DiscordUserRepository,
	jwtManager *auth.JWTManager,
	cfg *config.Config,
) *AuthHandler {
	return &AuthHandler{
		userRepo:        userRepo,
		tokenRepo:       tokenRepo,
		sessionRepo:     sessionRepo,
		discordUserRepo: discordUserRepo,
		jwtManager:      jwtManager,
		config:          cfg,
		log:             slog.Default().With(slog.String("handler", "auth")),
	}
}

// GetOAuthConfig returns the OAuth provider configuration for clients
func (s *AuthHandler) GetOAuthConfig(
	ctx context.Context,
	req *authpb.GetOAuthConfigRequest,
) (*authpb.GetOAuthConfigResponse, error) {
	s.log.Debug("GetOAuthConfig called", slog.Int("provider_count", len(s.config.Auth.Providers)))
	providers := make([]*authpb.OAuthProvider, 0, len(s.config.Auth.Providers))

	for _, providerConfig := range s.config.Auth.Providers {
		s.log.Debug("adding provider",
			slog.String("name", providerConfig.Name),
			slog.String("client_id", providerConfig.ClientID))

		// Get OIDC discovery document
		discovery, err := oidc.GetDiscoveryForProvider(ctx, providerConfig.Issuer)
		if err != nil {
			s.log.Warn("failed to get discovery for provider",
				slog.String("provider", providerConfig.Name),
				slog.String("error", err.Error()))
			continue
		}

		// Build authorization URL with placeholders for dynamic values
		// The web UI and CLI will substitute {redirect_uri}, {state}, {code_challenge}
		// We manually construct the query string to avoid encoding the placeholder braces

		// Manually build query string with proper encoding for static values only
		authURL := fmt.Sprintf(
			"%s?client_id=%s&redirect_uri={redirect_uri}&response_type=code&scope=%s&state={state}&code_challenge={code_challenge}&code_challenge_method=S256&prompt=consent",
			discovery.AuthorizationEndpoint,
			url.QueryEscape(providerConfig.ClientID),
			url.QueryEscape(strings.Join(providerConfig.Scopes, " ")),
		)

		providers = append(providers, &authpb.OAuthProvider{
			Name:             providerConfig.Name,
			ClientId:         providerConfig.ClientID,
			AuthorizationUrl: authURL,
		})
	}

	return &authpb.GetOAuthConfigResponse{
		Providers: providers,
	}, nil
}

// ExchangeAuthCode exchanges an authorization code for tokens server-side
func (s *AuthHandler) ExchangeAuthCode(
	ctx context.Context,
	req *authpb.ExchangeAuthCodeRequest,
) (*authpb.ExchangeAuthCodeResponse, error) {
	// Find provider config
	var providerConfig *config.ProviderConfig
	for _, pc := range s.config.Auth.Providers {
		if pc.Name == req.Provider {
			providerConfig = &pc
			break
		}
	}

	if providerConfig == nil {
		return nil, status.Errorf(codes.NotFound, "provider %s not configured", req.Provider)
	}

	// Get OIDC discovery document for the provider
	discovery, err := oidc.GetDiscoveryForProvider(ctx, providerConfig.Issuer)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get OIDC discovery: %v", err)
	}

	// Build OAuth2 config from discovered endpoints
	oauth2Config := &oauth2.Config{
		ClientID:     providerConfig.ClientID,
		ClientSecret: providerConfig.ClientSecret,
		RedirectURL:  req.RedirectUri,
		Endpoint: oauth2.Endpoint{
			AuthURL:  discovery.AuthorizationEndpoint,
			TokenURL: discovery.TokenEndpoint,
		},
		Scopes: providerConfig.Scopes,
	}

	// Exchange code for token (with PKCE verifier)
	token, err := oauth2Config.Exchange(ctx, req.Code, oauth2.VerifierOption(req.CodeVerifier))
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "failed to exchange code: %v", err)
	}

	// Extract ID token
	idToken, ok := token.Extra("id_token").(string)
	if !ok || idToken == "" {
		return nil, status.Error(codes.Internal, "no ID token in response")
	}

	// Validate ID token using OIDC provider
	provider, err := oidc.GetProvider(req.Provider)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "OIDC provider %s not registered: %v", req.Provider, err)
	}

	claims, err := provider.ValidateIDToken(ctx, idToken, token.AccessToken, *providerConfig)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid ID token: %v", err)
	}

	// Debug: Log claims
	s.log.Debug("validating claims",
		slog.String("email", claims.Email),
		slog.Bool("email_verified", claims.EmailVerified),
		slog.String("subject", claims.Subject))

	// Verify email is verified (skip for Okta admin users for now)
	if !claims.EmailVerified && req.Provider != "okta" {
		return nil, status.Error(codes.PermissionDenied, "email not verified")
	}

	// Check domain allowlist (domain checking is done in ValidateIDToken for Google)
	// But we'll keep this as a backup
	if len(providerConfig.AllowedDomains) > 0 && !claims.EmailVerified {
		return nil, status.Error(codes.PermissionDenied, "email verification required for domain restrictions")
	}

	// Discord-only user provisioning flow
	// Check if Discord user already exists (bot-first or previous OAuth)
	var user *entities.User
	var isNewUser bool

	log := s.log.With(
		slog.String("flow", "oauth"),
		slog.String("provider", req.Provider),
		slog.String("discord_id", claims.Subject),
		slog.String("discord_username", claims.Name),
	)
	log.Info("starting oauth authentication",
		slog.String("email", claims.Email),
		slog.Bool("email_verified", claims.EmailVerified))

	log.Debug("checking if discord_users record exists")
	discordUser, err := s.discordUserRepo.GetByDiscordID(ctx, claims.Subject)

	if err == nil && discordUser != nil && discordUser.UserID != nil {
		// User exists with linked account - get and potentially update email
		log.Info("found existing discord_users record with linked account",
			slog.String("user_id", *discordUser.UserID),
			slog.Time("linked_at", discordUser.LinkedAt))
		user, err = s.userRepo.GetByID(ctx, *discordUser.UserID)
		if err != nil {
			log.Error("failed to get user by id",
				slog.String("user_id", *discordUser.UserID),
				slog.String("error", err.Error()))
			return nil, status.Errorf(codes.Internal, "failed to get user: %v", err)
		}

		// Update user's email if it's currently empty and we have a verified email
		if user.Email == "" && claims.EmailVerified && claims.Email != "" {
			log.Info("updating user email", slog.String("email", claims.Email))
			user.Email = claims.Email
			if err := s.userRepo.Update(ctx, user); err != nil {
				log.Warn("failed to update user email", slog.String("error", err.Error()))
			}
		}

		// Update last seen
		_ = s.discordUserRepo.UpdateLastSeen(ctx, claims.Subject)
	} else {
		// New user or Discord user without linked account - create user and link
		if err == nil && discordUser != nil && discordUser.UserID == nil {
			log.Info("found discord_users record with no linked account, will provision and link",
				slog.String("discord_username", discordUser.DiscordUsername),
				slog.Time("discord_user_created", discordUser.LinkedAt))
		} else if err != nil {
			log.Info("no existing discord_users record found, will create new user",
				slog.String("lookup_error", err.Error()))
		} else {
			log.Info("no existing discord_users record found, will create new user")
		}

		if !providerConfig.AutoProvision {
			log.Warn("auto-provisioning is disabled, rejecting authentication")
			return nil, status.Error(codes.PermissionDenied, "auto-provisioning not enabled")
		}

		log.Info("creating new user",
			slog.String("email", claims.Email),
			slog.String("name", claims.Name))
		user = &entities.User{
			Email:       claims.Email,
			DisplayName: claims.Name,
			Role:        entities.RoleUser,
			IsActive:    true,
		}

		if err := s.userRepo.Create(ctx, user); err != nil {
			log.Error("failed to create user in database",
				slog.String("email", user.Email),
				slog.String("error", err.Error()))
			return nil, status.Errorf(codes.Internal, "failed to create user: %v", err)
		}
		log.Info("user created successfully",
			slog.String("user_id", user.ID),
			slog.String("email", user.Email),
			slog.String("display_name", user.DisplayName))

		// Create discord_users record
		userIDPtr := user.ID
		newDiscordUser := &entities.DiscordUser{
			DiscordID:       claims.Subject,
			UserID:          &userIDPtr,
			DiscordUsername: claims.Name,
			AvatarHash:      nil, // OIDC provides URL, not hash - will be populated by bot sync
			LinkedAt:        time.Now(),
			LastSeen:        nil,
		}

		if err := s.discordUserRepo.Upsert(ctx, newDiscordUser); err != nil {
			log.Error("failed to upsert discord_users record",
				slog.String("user_id", user.ID),
				slog.String("error", err.Error()))
		} else {
			log.Info("discord_users record linked successfully",
				slog.String("user_id", user.ID))
		}

		isNewUser = true
	}

	// Check if user is active
	if user == nil {
		return nil, status.Error(codes.Internal, "user is nil - this should not happen")
	}
	if !user.IsActive {
		return nil, status.Error(codes.PermissionDenied, "user account is inactive")
	}

	// Update identity last login (best effort - don't fail if it errors)
	// Note: UpdateLastLogin doesn't exist yet, we'll skip this for now
	// if err := s.identityRepo.UpdateLastLogin(ctx, identity.IdentityID); err != nil {
	// 	log.Printf("Warning: failed to update last login: %v", err)
	// }

	// Generate token ID
	tokenID, err := auth.GenerateTokenID()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate token ID: %v", err)
	}

	// Get display name and picture from claims
	displayName := claims.Name
	picture := claims.Picture

	// For Discord users, use Discord username as display name
	if req.Provider == "discord" && discordUser != nil {
		// Prefer global name, fallback to username
		if discordUser.DiscordGlobalName != nil && *discordUser.DiscordGlobalName != "" {
			displayName = *discordUser.DiscordGlobalName
		} else if discordUser.DiscordUsername != "" {
			displayName = discordUser.DiscordUsername
		}
		// Use Discord avatar
		// Note: DiscordUser now stores avatar_hash, not full URL
		// For now, we keep picture from OIDC claims for User.AvatarURL
	}

	// Update user's avatar URL and timezone from this login
	needsUpdate := false

	// Update avatar URL if changed
	if picture != "" && (user.AvatarURL == nil || (user.AvatarURL != nil && *user.AvatarURL != picture)) {
		user.AvatarURL = &picture
		log.Info("updating user avatar URL",
			slog.String("user_id", user.ID),
			slog.String("avatar_url", picture))
		needsUpdate = true
	}

	// Update timezone if provided and different
	if req.Timezone != "" && (user.Timezone == nil || (user.Timezone != nil && *user.Timezone != req.Timezone)) {
		user.Timezone = &req.Timezone
		log.Info("updating user timezone",
			slog.String("user_id", user.ID),
			slog.String("timezone", req.Timezone))
		needsUpdate = true
	}

	if needsUpdate {
		if err := s.userRepo.Update(ctx, user); err != nil {
			log.Warn("failed to update user profile", slog.String("error", err.Error()))
			// Don't fail the login if profile update fails
		}
	}

	// Get timezone for JWT (prefer user profile, fallback to request)
	timezone := ""
	if user.Timezone != nil {
		timezone = *user.Timezone
	} else if req.Timezone != "" {
		timezone = req.Timezone
	}

	// Generate JWT with user profile information
	tokenString, expiresAt, err := s.jwtManager.GenerateTokenWithClaims(
		user.ID,
		user.Email,
		displayName,
		picture,
		timezone,
		string(user.Role),
		tokenID,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate token: %v", err)
	}

	// Store token in database
	// API token record expires much later than JWT to allow refreshing
	apiTokenExpiry := time.Now().Add(30 * 24 * time.Hour) // 30 days
	apiToken := &entities.APIToken{
		ID:         tokenID,
		UserID:     user.ID,
		TokenHash:  tokenString,
		DeviceName: req.DeviceName,
		Scopes:     req.Scopes,
		ExpiresAt:  apiTokenExpiry, // API token valid for 30 days
		LastUsed:   &[]time.Time{time.Now()}[0],
	}

	if err := s.tokenRepo.Create(ctx, apiToken); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to store token: %v", err)
	}

	return &authpb.ExchangeAuthCodeResponse{
		ApiToken: tokenString,
		TokenId:  tokenID,
		User: &userpb.User{
			UserId:   user.ID,
			Email:    user.Email,
			Name:     user.DisplayName,
			Role:     userpb.Role(userpb.Role_value[string(user.Role)]),
			UserType: userpb.UserType(userpb.UserType_value[string(user.UserType)]),
		},
		ExpiresAt: timestamppb.New(expiresAt),
		IsNewUser: isNewUser,
	}, nil
}

// RefreshOAuthToken handles OAuth token refresh
func (s *AuthHandler) RefreshOAuthToken(
	ctx context.Context,
	req *authpb.RefreshOAuthTokenRequest,
) (*authpb.RefreshOAuthTokenResponse, error) {
	// Validate input
	if req.Provider == "" {
		return nil, status.Error(codes.InvalidArgument, "provider is required")
	}
	if req.RefreshToken == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh_token is required")
	}

	// Get provider config
	var providerConfig *config.ProviderConfig
	for _, p := range s.config.Auth.Providers {
		if p.Name == req.Provider {
			providerConfig = &p
			break
		}
	}
	if providerConfig == nil {
		return nil, status.Errorf(codes.InvalidArgument, "unsupported provider: %s", req.Provider)
	}

	// Get OIDC discovery document for the provider
	discovery, err := oidc.GetDiscoveryForProvider(ctx, providerConfig.Issuer)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get OIDC discovery: %v", err)
	}

	// Create OAuth2 config with client secret (server-side only)
	oauth2Config := &oauth2.Config{
		ClientID:     providerConfig.ClientID,
		ClientSecret: providerConfig.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  discovery.AuthorizationEndpoint,
			TokenURL: discovery.TokenEndpoint,
		},
	}

	// Exchange refresh token for new access token
	token := &oauth2.Token{
		RefreshToken: req.RefreshToken,
	}
	tokenSource := oauth2Config.TokenSource(ctx, token)
	newToken, err := tokenSource.Token()
	if err != nil {
		s.log.Error("failed to refresh OAuth token", slog.String("error", err.Error()))
		return nil, status.Error(codes.Unauthenticated, "failed to refresh OAuth token")
	}

	// Extract ID token
	idToken, ok := newToken.Extra("id_token").(string)
	if !ok || idToken == "" {
		return nil, status.Error(codes.Internal, "no id_token in refresh response")
	}

	// Validate ID token via OIDC provider
	provider, err := oidc.GetProvider(req.Provider)
	if err != nil || provider == nil {
		return nil, status.Errorf(codes.InvalidArgument, "OIDC provider not configured: %s", req.Provider)
	}

	claims, err := provider.ValidateIDToken(ctx, idToken, newToken.AccessToken, *providerConfig)
	if err != nil {
		s.log.Error("failed to validate refreshed ID token", slog.String("error", err.Error()))
		return nil, status.Error(codes.Unauthenticated, "invalid ID token")
	}

	// Get user by Discord ID (Discord-only app)
	discordUser, err := s.discordUserRepo.GetByDiscordID(ctx, claims.Subject)
	if err != nil {
		s.log.Error("discord user not found",
			slog.String("subject", claims.Subject),
			slog.String("error", err.Error()))
		return nil, status.Error(codes.Unauthenticated, "user not found")
	}

	// Get user
	if discordUser.UserID == nil {
		s.log.Error("discord user has no linked account", slog.String("discord_id", claims.Subject))
		return nil, status.Error(codes.Unauthenticated, "user not found")
	}
	user, err := s.userRepo.GetByID(ctx, *discordUser.UserID)
	if err != nil {
		s.log.Error("user not found for discord user",
			slog.String("discord_user_id", *discordUser.UserID),
			slog.String("error", err.Error()))
		return nil, status.Error(codes.Unauthenticated, "user not found")
	}
	if user == nil {
		s.log.Error("user is nil", slog.String("discord_user_id", *discordUser.UserID))
		return nil, status.Error(codes.Unauthenticated, "user not found")
	}

	// Update last seen
	_ = s.discordUserRepo.UpdateLastSeen(ctx, claims.Subject)

	// Generate new JWT token with user profile information
	tokenID, err := auth.GenerateTokenID()
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate token ID")
	}

	displayName := user.DisplayName
	if displayName == "" {
		displayName = user.Email // fallback to email if no display name
	}
	avatarURL := ""
	if user.AvatarURL != nil {
		avatarURL = *user.AvatarURL
	}
	timezone := ""
	if user.Timezone != nil {
		timezone = *user.Timezone
	}
	tokenString, expiresAt, err := s.jwtManager.GenerateTokenWithClaims(
		user.ID,
		user.Email,
		displayName,
		avatarURL,
		timezone,
		string(user.Role),
		tokenID,
	)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate token")
	}

	// Store token in database
	deviceName := req.DeviceName
	if deviceName == "" {
		deviceName = "refreshed-device"
	}
	// API token record expires much later than JWT to allow refreshing
	apiTokenExpiry := time.Now().Add(30 * 24 * time.Hour) // 30 days
	apiToken := &entities.APIToken{
		ID:         tokenID,
		UserID:     user.ID,
		DeviceName: deviceName,
		Scopes:     []string{"hivemind:read", "hivemind:write"},
		ExpiresAt:  apiTokenExpiry, // API token valid for 30 days
		LastUsed:   &[]time.Time{time.Now()}[0],
	}
	if err := s.tokenRepo.Create(ctx, apiToken); err != nil {
		s.log.Warn("failed to store API token", slog.String("error", err.Error()))
		// Non-fatal, token is still valid
	}

	// Determine which refresh token to return
	responseRefreshToken := req.RefreshToken
	if newToken.RefreshToken != "" {
		// Provider rotated the refresh token
		responseRefreshToken = newToken.RefreshToken
	}

	return &authpb.RefreshOAuthTokenResponse{
		ApiToken:     tokenString,
		TokenId:      tokenID,
		ExpiresAt:    timestamppb.New(expiresAt),
		RefreshToken: responseRefreshToken,
	}, nil
}

// AuthenticateLocal handles local username/password authentication
func (s *AuthHandler) AuthenticateLocal(
	ctx context.Context,
	req *authpb.AuthenticateLocalRequest,
) (*authpb.AuthenticateLocalResponse, error) {
	// Validate input
	if req.Username == "" || req.Password == "" {
		return nil, ErrInvalidCredentials
	}

	// Get user by email (we use email as the login identifier)
	user, err := s.userRepo.GetByEmail(ctx, req.Username)
	if err != nil || user == nil {
		// User not found or other error - don't reveal which
		return nil, ErrInvalidCredentials
	}

	// Verify password
	if user.PasswordHash == nil || !user.VerifyPassword(req.Password) {
		return nil, ErrInvalidCredentials
	}

	// Check if user is active
	if !user.IsActive {
		return nil, ErrUserNotActive
	}

	// Generate token ID
	tokenID, err := auth.GenerateTokenID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token ID: %w", err)
	}

	// Generate JWT with user profile information
	displayName := user.DisplayName
	if displayName == "" {
		displayName = user.Email // fallback to email if no display name
	}
	avatarURL := ""
	if user.AvatarURL != nil {
		avatarURL = *user.AvatarURL
	}
	timezone := ""
	if user.Timezone != nil {
		timezone = *user.Timezone
	}
	tokenString, expiresAt, err := s.jwtManager.GenerateTokenWithClaims(
		user.ID,
		user.Email, // use email as username
		displayName,
		avatarURL,
		timezone,
		string(user.Role),
		tokenID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Store token in database for tracking/revocation
	// Note: We store the raw JWT as the token hash for now
	// In production, you'd hash it before storage
	// API token record expires much later than JWT to allow refreshing
	apiTokenExpiry := time.Now().Add(30 * 24 * time.Hour) // 30 days
	token := &entities.APIToken{
		ID:         tokenID,
		UserID:     user.ID,
		TokenHash:  tokenString, // TODO: hash this in production
		DeviceName: req.DeviceName,
		Scopes:     req.Scopes,
		ExpiresAt:  apiTokenExpiry, // API token valid for 30 days
		CreatedAt:  time.Now(),
	}

	if err := s.tokenRepo.Create(ctx, token); err != nil {
		return nil, fmt.Errorf("failed to store token: %w", err)
	}

	// Build response
	return &authpb.AuthenticateLocalResponse{
		ApiToken: tokenString,
		TokenId:  tokenID,
		User: &userpb.User{
			UserId:   user.ID,
			Email:    user.Email,
			Name:     user.DisplayName,
			Role:     userpb.Role(userpb.Role_value[string(user.Role)]),
			UserType: userpb.UserType(userpb.UserType_value[string(user.UserType)]),
		},
		ExpiresAt: timestamppb.New(expiresAt),
	}, nil
}

// RefreshToken handles token refresh
// For OIDC users with expired JWTs, this will use the server-side OAuth refresh token
func (s *AuthHandler) RefreshToken(
	ctx context.Context,
	req *authpb.RefreshTokenRequest,
) (*authpb.RefreshTokenResponse, error) {
	s.log.Debug("RefreshToken called", slog.String("token_id", req.TokenId))

	// Get existing token
	s.log.Debug("getting token from repository", slog.String("token_id", req.TokenId))
	existingToken, err := s.tokenRepo.GetByID(ctx, req.TokenId)
	s.log.Debug("token repository result",
		slog.Bool("found", existingToken != nil),
		slog.Bool("has_error", err != nil))

	if err != nil {
		s.log.Error("failed to get token", slog.String("error", err.Error()))
		return nil, status.Error(codes.NotFound, "token not found")
	}
	if existingToken == nil {
		s.log.Error("token is nil (not found in database)")
		return nil, status.Error(codes.NotFound, "token not found")
	}

	s.log.Debug("token found",
		slog.String("user_id", existingToken.UserID),
		slog.Bool("revoked", existingToken.RevokedAt != nil))

	// Check if token is revoked
	if existingToken.RevokedAt != nil {
		return nil, status.Error(codes.Unauthenticated, "token has been revoked")
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, existingToken.UserID)
	if err != nil || user == nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}

	// Check if user is active
	if !user.IsActive {
		return nil, status.Error(codes.PermissionDenied, "user account is not active")
	}

	// If token is expired and user is OIDC type, try to refresh via OAuth
	if time.Now().After(existingToken.ExpiresAt) && user.UserType == entities.UserTypeOIDC {
		// For Discord-only app, provider is always "discord"
		provider := "discord"

		// Get OIDC session with refresh token
		oidcSession, err := s.sessionRepo.GetOIDCSessionByUserAndProvider(ctx, user.ID, provider)
		if err != nil || oidcSession == nil || oidcSession.RefreshToken == nil {
			return nil, status.Error(codes.Unauthenticated, "no refresh token available - please login again")
		}

		// Get provider config
		var providerConfig *config.ProviderConfig
		for _, pc := range s.config.Auth.Providers {
			if pc.Name == provider {
				providerConfig = &pc
				break
			}
		}
		if providerConfig == nil {
			return nil, status.Errorf(codes.Internal, "provider %s not configured", provider)
		}

		// Get OIDC discovery document for the provider
		discovery, err := oidc.GetDiscoveryForProvider(ctx, providerConfig.Issuer)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get OIDC discovery: %v", err)
		}

		// Build OAuth2 config
		// Note: RedirectURL is not needed for refresh token flow per OAuth2 spec
		oauth2Config := &oauth2.Config{
			ClientID:     providerConfig.ClientID,
			ClientSecret: providerConfig.ClientSecret,
			Endpoint: oauth2.Endpoint{
				TokenURL: discovery.TokenEndpoint,
			},
		}

		s.log.Debug("OAuth2 config for refresh",
			slog.String("client_id", oauth2Config.ClientID),
			slog.String("token_url", oauth2Config.Endpoint.TokenURL))
		s.log.Debug("refresh token info",
			slog.Int("length", len(*oidcSession.RefreshToken)),
			slog.String("prefix", (*oidcSession.RefreshToken)[:min(20, len(*oidcSession.RefreshToken))]))

		// Exchange refresh token for new access token
		token, err := oauth2Config.TokenSource(ctx, &oauth2.Token{
			RefreshToken: *oidcSession.RefreshToken,
		}).Token()
		if err != nil {
			s.log.Error("failed to refresh OAuth token",
				slog.String("user_id", user.ID),
				slog.String("provider", provider),
				slog.String("error", err.Error()))
			return nil, status.Error(codes.Unauthenticated, "failed to refresh OAuth token - please login again")
		}

		// Update the OIDC session with potentially rotated refresh token
		if token.RefreshToken != "" && token.RefreshToken != *oidcSession.RefreshToken {
			oidcSession.RefreshToken = &token.RefreshToken
			now := time.Now()
			oidcSession.LastRefreshed = &now
			if err := s.sessionRepo.UpdateOIDCSession(ctx, oidcSession); err != nil {
				s.log.Warn("failed to update OIDC session", slog.String("error", err.Error()))
			}
		}

		// Note: For Google, we could extract and validate a new ID token here
		// For now, we trust that the refresh worked and just issue a new JWT
	}

	// Generate new JWT with user profile information (including avatar)
	s.log.Debug("generating new JWT",
		slog.String("user_id", user.ID),
		slog.String("token_id", existingToken.ID))
	displayName := user.DisplayName
	if displayName == "" {
		displayName = user.Email // fallback to email if no display name
	}
	avatarURL := ""
	if user.AvatarURL != nil {
		avatarURL = *user.AvatarURL
	}
	timezone := ""
	if user.Timezone != nil {
		timezone = *user.Timezone
	}
	tokenString, expiresAt, err := s.jwtManager.GenerateTokenWithClaims(
		user.ID,
		user.Email,
		displayName,
		avatarURL, // Include avatar URL from database
		timezone,  // Include timezone from database
		string(user.Role),
		existingToken.ID,
	)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate token")
	}
	s.log.Debug("generated new JWT",
		slog.String("token_prefix", tokenString[:min(30, len(tokenString))]),
		slog.Time("expires_at", expiresAt))

	// Update token in database
	existingToken.TokenHash = tokenString // TODO: hash this in production
	existingToken.ExpiresAt = expiresAt
	now := time.Now()
	existingToken.LastUsed = &now

	if err := s.tokenRepo.Update(ctx, existingToken); err != nil {
		return nil, status.Error(codes.Internal, "failed to update token")
	}

	return &authpb.RefreshTokenResponse{
		ApiToken:  tokenString,
		ExpiresAt: timestamppb.New(expiresAt),
	}, nil
}

// RevokeToken handles token revocation
func (s *AuthHandler) RevokeToken(
	ctx context.Context,
	req *authpb.RevokeTokenRequest,
) (*authpb.RevokeTokenResponse, error) {
	err := s.tokenRepo.Revoke(ctx, req.TokenId)
	if err != nil {
		return nil, fmt.Errorf("failed to revoke token: %w", err)
	}

	return &authpb.RevokeTokenResponse{
		Success: true,
	}, nil
}

// LoginWithOIDC handles OIDC authentication with auto-provisioning and identity linking
func (s *AuthHandler) LoginWithOIDC(
	ctx context.Context,
	req *authpb.LoginWithOIDCRequest,
) (*authpb.LoginWithOIDCResponse, error) {
	// Validate request
	if req.Provider == "" {
		return nil, status.Error(codes.InvalidArgument, "provider is required")
	}
	if req.IdToken == "" {
		return nil, status.Error(codes.InvalidArgument, "id_token is required")
	}

	// Get the OIDC provider
	provider, err := oidc.GetProvider(req.Provider)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "unsupported provider: %s", req.Provider)
	}

	// Find provider config
	var providerConfig *config.ProviderConfig
	for _, pc := range s.config.Auth.Providers {
		if pc.Name == req.Provider {
			providerConfig = &pc
			break
		}
	}
	if providerConfig == nil {
		return nil, status.Errorf(codes.NotFound, "provider %s not configured", req.Provider)
	}

	// Validate ID token and extract claims
	// For ValidateToken, we don't have the access token, so pass empty string
	claims, err := provider.ValidateIDToken(ctx, req.IdToken, "", *providerConfig)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid ID token: %v", err)
	}

	// Check email verified
	if !claims.EmailVerified {
		return nil, status.Error(codes.PermissionDenied, "email not verified by provider")
	}

	// Check if provider config allows auto-provisioning
	if !providerConfig.AutoProvision {
		return nil, status.Error(codes.PermissionDenied, "auto-provisioning is disabled for this provider")
	}

	// Try to get existing Discord user (Discord-only app)
	log := s.log.With(
		slog.String("flow", "oidc_login"),
		slog.String("provider", req.Provider),
		slog.String("discord_id", claims.Subject),
		slog.String("discord_username", claims.Name),
	)
	log.Info("starting OIDC authentication",
		slog.String("email", claims.Email),
		slog.Bool("email_verified", claims.EmailVerified))

	discordUser, err := s.discordUserRepo.GetByDiscordID(ctx, claims.Subject)

	var user *entities.User
	var isNewUser bool

	if err == nil && discordUser != nil && discordUser.UserID != nil {
		// Discord user exists with linked account, get the associated user
		log.Info("found existing discord_users record with linked account",
			slog.String("user_id", *discordUser.UserID))
		user, err = s.userRepo.GetByID(ctx, *discordUser.UserID)
		if err != nil {
			log.Error("failed to get user by id",
				slog.String("user_id", *discordUser.UserID),
				slog.String("error", err.Error()))
			return nil, status.Errorf(codes.Internal, "failed to get user: %v", err)
		}
		if user == nil {
			log.Error("user not found in database despite discord_users link",
				slog.String("user_id", *discordUser.UserID))
			return nil, status.Error(codes.NotFound, "user not found")
		}

		// Update email if needed
		if user.Email == "" && claims.EmailVerified && claims.Email != "" {
			user.Email = claims.Email
			_ = s.userRepo.Update(ctx, user)
		}

		// Update last seen
		_ = s.discordUserRepo.UpdateLastSeen(ctx, claims.Subject)
	} else {
		// Discord user doesn't exist or has no linked account - create new user and link
		if err == nil && discordUser != nil && discordUser.UserID == nil {
			log.Info("found discord_users record with no linked account, will provision and link",
				slog.String("discord_username", discordUser.DiscordUsername),
				slog.Time("discord_user_created", discordUser.LinkedAt))
		} else if err != nil {
			log.Info("no existing discord_users record found, will create new user",
				slog.String("lookup_error", err.Error()))
		} else {
			log.Info("no existing discord_users record found, will create new user")
		}

		user = &entities.User{
			ID:          idgen.GenerateID(),
			Email:       claims.Email,
			DisplayName: claims.Name,
			Role:        entities.RoleUser,
			UserType:    entities.UserTypeOIDC,
			IsActive:    true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		if err := s.userRepo.Create(ctx, user); err != nil {
			log.Error("failed to create user in database",
				slog.String("email", user.Email),
				slog.String("error", err.Error()))
			return nil, status.Errorf(codes.Internal, "failed to create user: %v", err)
		}
		log.Info("user created successfully",
			slog.String("user_id", user.ID),
			slog.String("email", user.Email),
			slog.String("display_name", user.DisplayName))

		// Create discord_users record
		userIDPtr := user.ID
		newDiscordUser := &entities.DiscordUser{
			DiscordID:       claims.Subject,
			UserID:          &userIDPtr,
			DiscordUsername: claims.Name,
			AvatarHash:      nil, // OIDC provides URL, not hash - will be populated by bot sync
			LinkedAt:        time.Now(),
			LastSeen:        nil,
		}

		if err := s.discordUserRepo.Upsert(ctx, newDiscordUser); err != nil {
			log.Error("failed to upsert discord_users record",
				slog.String("user_id", user.ID),
				slog.String("error", err.Error()))
			return nil, status.Errorf(codes.Internal, "failed to create discord user: %v", err)
		}
		log.Info("discord_users record linked successfully",
			slog.String("user_id", user.ID))

		isNewUser = true
	}

	// Check if user is active
	if !user.IsActive {
		log.Warn("authentication rejected - user account is inactive",
			slog.String("user_id", user.ID))
		return nil, status.Error(codes.PermissionDenied, "user account is not active")
	}

	log.Info("authentication successful",
		slog.String("user_id", user.ID),
		slog.Bool("is_new_user", isNewUser))

	// Generate token ID
	tokenID, err := auth.GenerateTokenID()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate token ID: %v", err)
	}

	// Get display name and picture from claims
	displayName := claims.Name
	picture := claims.Picture

	// Update user's avatar URL with the picture from this login
	if picture != "" && (user.AvatarURL == nil || (user.AvatarURL != nil && *user.AvatarURL != picture)) {
		user.AvatarURL = &picture
		if err := s.userRepo.Update(ctx, user); err != nil {
			s.log.Warn("failed to update user avatar URL", slog.String("error", err.Error()))
			// Don't fail the login if avatar update fails
		}
	}

	// Get timezone from user profile
	timezone := ""
	if user.Timezone != nil {
		timezone = *user.Timezone
	}

	// Generate JWT with user profile information
	tokenString, expiresAt, err := s.jwtManager.GenerateTokenWithClaims(
		user.ID,
		user.Email,
		displayName,
		picture,
		timezone,
		string(user.Role),
		tokenID,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate token: %v", err)
	}

	// Store token in database
	// API token record expires much later than JWT to allow refreshing
	apiTokenExpiry := time.Now().Add(30 * 24 * time.Hour) // 30 days
	token := &entities.APIToken{
		ID:         tokenID,
		UserID:     user.ID,
		TokenHash:  tokenString, // TODO: hash this in production
		DeviceName: req.DeviceName,
		Scopes:     req.Scopes,
		ExpiresAt:  apiTokenExpiry, // API token valid for 30 days
		CreatedAt:  time.Now(),
	}

	if err := s.tokenRepo.Create(ctx, token); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to store token: %v", err)
	}

	// Store OAuth refresh token server-side if provided
	if req.RefreshToken != "" {
		// Check if we already have an OIDC session for this user+provider
		existingSession, err := s.sessionRepo.GetOIDCSessionByUserAndProvider(ctx, user.ID, req.Provider)
		if err != nil {
			// Log error but continue - this is not critical
			s.log.Warn("failed to get existing OIDC session", slog.String("error", err.Error()))
		}

		if existingSession != nil {
			// Update existing session with new refresh token
			existingSession.RefreshToken = &req.RefreshToken
			now := time.Now()
			existingSession.LastRefreshed = &now
			// Refresh tokens typically don't expire for months, but we'll set a reasonable default
			expiresIn := 90 * 24 * time.Hour // 90 days
			newExpiry := now.Add(expiresIn)
			existingSession.ExpiresAt = newExpiry

			if err := s.sessionRepo.UpdateOIDCSession(ctx, existingSession); err != nil {
				// Log error but don't fail the login
				s.log.Warn("failed to update OIDC session", slog.String("error", err.Error()))
			}
		} else {
			// Create new OIDC session with refresh token
			now := time.Now()
			expiresIn := 90 * 24 * time.Hour // 90 days default
			oidcSession := &entities.OIDCSession{
				ID:            idgen.GenerateID(),
				UserID:        &user.ID,
				Provider:      req.Provider,
				RefreshToken:  &req.RefreshToken,
				ExpiresAt:     now.Add(expiresIn),
				CreatedAt:     now,
				LastRefreshed: &now,
			}

			if err := s.sessionRepo.CreateOIDCSession(ctx, oidcSession); err != nil {
				// Log error but don't fail the login
				s.log.Warn("failed to create OIDC session", slog.String("error", err.Error()))
			}
		}
	}

	// Build response
	return &authpb.LoginWithOIDCResponse{
		ApiToken: tokenString,
		TokenId:  tokenID,
		User: &userpb.User{
			UserId:   user.ID,
			Email:    user.Email,
			Name:     user.DisplayName,
			Role:     userpb.Role(userpb.Role_value[string(user.Role)]),
			UserType: userpb.UserType(userpb.UserType_value[string(user.UserType)]),
		},
		ExpiresAt: timestamppb.New(expiresAt),
		IsNewUser: isNewUser,
	}, nil
}
