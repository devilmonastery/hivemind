package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
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
	userRepo     repositories.UserRepository
	identityRepo repositories.IdentityRepository
	tokenRepo    repositories.TokenRepository
	sessionRepo  repositories.SessionRepository
	jwtManager   *auth.JWTManager
	config       *config.Config
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(
	userRepo repositories.UserRepository,
	identityRepo repositories.IdentityRepository,
	tokenRepo repositories.TokenRepository,
	sessionRepo repositories.SessionRepository,
	jwtManager *auth.JWTManager,
	cfg *config.Config,
) *AuthHandler {
	return &AuthHandler{
		userRepo:     userRepo,
		identityRepo: identityRepo,
		tokenRepo:    tokenRepo,
		sessionRepo:  sessionRepo,
		jwtManager:   jwtManager,
		config:       cfg,
	}
}

// GetOAuthConfig returns the OAuth provider configuration for clients
func (s *AuthHandler) GetOAuthConfig(
	ctx context.Context,
	req *authpb.GetOAuthConfigRequest,
) (*authpb.GetOAuthConfigResponse, error) {
	log.Printf("[DEBUG] GetOAuthConfig called. Config providers: %d", len(s.config.Auth.Providers))
	providers := make([]*authpb.OAuthProvider, 0, len(s.config.Auth.Providers))

	for _, providerConfig := range s.config.Auth.Providers {
		log.Printf("[DEBUG] Adding provider: %s (client_id: %s)", providerConfig.Name, providerConfig.ClientID)

		// Get OIDC discovery document
		discovery, err := oidc.GetDiscoveryForProvider(ctx, providerConfig.Issuer)
		if err != nil {
			log.Printf("[WARN] Failed to get discovery for provider %s: %v", providerConfig.Name, err)
			continue
		}

		// Build authorization URL with placeholders for dynamic values
		// The web UI and CLI will substitute {redirect_uri}, {state}, {code_challenge}
		scopesStr := strings.Join(providerConfig.Scopes, "%20")
		authURL := fmt.Sprintf("%s?client_id=%s&redirect_uri={redirect_uri}&response_type=code&scope=%s&state={state}&code_challenge={code_challenge}&code_challenge_method=S256&prompt=consent",
			discovery.AuthorizationEndpoint,
			providerConfig.ClientID,
			scopesStr,
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
	log.Printf("[DEBUG] Claims - Email: %s, EmailVerified: %t, Subject: %s", claims.Email, claims.EmailVerified, claims.Subject)

	// Verify email is verified (skip for Okta admin users for now)
	if !claims.EmailVerified && req.Provider != "okta" {
		return nil, status.Error(codes.PermissionDenied, "email not verified")
	}

	// Check domain allowlist (domain checking is done in ValidateIDToken for Google)
	// But we'll keep this as a backup
	if len(providerConfig.AllowedDomains) > 0 && !claims.EmailVerified {
		return nil, status.Error(codes.PermissionDenied, "email verification required for domain restrictions")
	}

	// Try to find existing identity
	log.Printf("[OAuth] Looking up identity: provider=%s, external_id=%s", req.Provider, claims.Subject)
	identity, err := s.identityRepo.GetByProviderAndExternalID(ctx, req.Provider, claims.Subject)

	var user *entities.User
	var isNewUser bool

	if err != nil || identity == nil {
		log.Printf("[OAuth] Identity not found, will create new user/identity for email=%s", claims.Email)
		// Identity doesn't exist - check if we should auto-link by email
		if claims.EmailVerified {
			log.Printf("[OAuth] Checking for existing user with email=%s", claims.Email)
			existingUser, err := s.userRepo.GetByEmail(ctx, claims.Email)
			if err == nil && existingUser != nil {
				log.Printf("[OAuth] Found existing user, linking identity: user_id=%s", existingUser.ID)
				// Found existing user with same email - link identity
				user = existingUser
				identity = &entities.Identity{
					UserID:            user.ID,
					Provider:          req.Provider,
					ExternalID:        claims.Subject,
					Email:             claims.Email,
					EmailVerified:     claims.EmailVerified,
					DisplayName:       claims.Name,
					ProfilePictureURL: claims.Picture,
				}
				if err := s.identityRepo.Create(ctx, identity); err != nil {
					return nil, status.Errorf(codes.Internal, "failed to create identity: %v", err)
				}
				log.Printf("[OAuth] Identity linked successfully")
			} else {
				log.Printf("[OAuth] No existing user found with email=%s (err=%v, user_nil=%v)", claims.Email, err, existingUser == nil)
			}
		}

		// No existing user - create new if auto-provision enabled
		if user == nil {
			log.Printf("[OAuth] No user exists, checking auto-provision for provider=%s", req.Provider)
			if !providerConfig.AutoProvision {
				return nil, status.Error(codes.PermissionDenied, "auto-provisioning not enabled")
			}

			log.Printf("[OAuth] Creating new user: email=%s, name=%s", claims.Email, claims.Name)
			// Create new user
			user = &entities.User{
				Email:       claims.Email,
				DisplayName: claims.Name,
				Role:        entities.RoleUser,
				IsActive:    true,
			}

			if err := s.userRepo.Create(ctx, user); err != nil {
				return nil, status.Errorf(codes.Internal, "failed to create user: %v", err)
			}
			log.Printf("[OAuth] User created successfully: user_id=%s", user.ID)

			// Create identity
			identity = &entities.Identity{
				UserID:            user.ID,
				Provider:          req.Provider,
				ExternalID:        claims.Subject,
				Email:             claims.Email,
				EmailVerified:     claims.EmailVerified,
				DisplayName:       claims.Name,
				ProfilePictureURL: claims.Picture,
			}

			if err := s.identityRepo.Create(ctx, identity); err != nil {
				return nil, status.Errorf(codes.Internal, "failed to create identity: %v", err)
			}
			log.Printf("[OAuth] Identity created successfully: identity_id=%s, user_id=%s", identity.IdentityID, identity.UserID)

			isNewUser = true
		}
	} else {
		log.Printf("[OAuth] Identity found: identity_id=%s, user_id=%s", identity.IdentityID, identity.UserID)
		// Identity exists - get user
		user, err = s.userRepo.GetByID(ctx, identity.UserID)
		if err != nil {
			log.Printf("[OAuth] ERROR: Failed to get user by ID: user_id=%s, error=%v", identity.UserID, err)
			return nil, status.Errorf(codes.Internal, "failed to get user: %v", err)
		}
		if user == nil {
			log.Printf("[OAuth] ERROR: User is nil after GetByID: user_id=%s (orphaned identity?)", identity.UserID)
			return nil, status.Error(codes.Internal, "user not found for identity")
		}
		log.Printf("[OAuth] User found: user_id=%s, email=%s, active=%v", user.ID, user.Email, user.IsActive)
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

	// Get display name and picture from identity or claims
	displayName := ""
	picture := ""
	if identity != nil {
		displayName = identity.DisplayName
		picture = identity.ProfilePictureURL
	}
	// If identity doesn't have them, try claims
	if displayName == "" && claims != nil {
		displayName = claims.Name
	}
	if picture == "" && claims != nil {
		picture = claims.Picture
	}

	// Update user's avatar URL and timezone from this login
	needsUpdate := false

	// Update avatar URL if changed
	if picture != "" && (user.AvatarURL == nil || (user.AvatarURL != nil && *user.AvatarURL != picture)) {
		user.AvatarURL = &picture
		log.Printf("[OAuth] Updating user avatar URL: user_id=%s, avatar_url=%s", user.ID, picture)
		needsUpdate = true
	}

	// Update timezone if provided and different
	if req.Timezone != "" && (user.Timezone == nil || (user.Timezone != nil && *user.Timezone != req.Timezone)) {
		user.Timezone = &req.Timezone
		log.Printf("[OAuth] Updating user timezone: user_id=%s, timezone=%s", user.ID, req.Timezone)
		needsUpdate = true
	}

	if needsUpdate {
		if err := s.userRepo.Update(ctx, user); err != nil {
			log.Printf("Warning: failed to update user profile: %v", err)
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
		log.Printf("Failed to refresh OAuth token: %v", err)
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
		log.Printf("Failed to validate refreshed ID token: %v", err)
		return nil, status.Error(codes.Unauthenticated, "invalid ID token")
	}

	// Get user by external ID
	identity, err := s.identityRepo.GetByProviderAndExternalID(ctx, req.Provider, claims.Subject)
	if err != nil {
		log.Printf("Identity not found for %s/%s: %v", req.Provider, claims.Subject, err)
		return nil, status.Error(codes.Unauthenticated, "user not found")
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, identity.UserID)
	if err != nil {
		log.Printf("User not found for identity %s: %v", identity.UserID, err)
		return nil, status.Error(codes.Unauthenticated, "user not found")
	}
	if user == nil {
		log.Printf("User is nil for identity %s (orphaned identity)", identity.UserID)
		return nil, status.Error(codes.Unauthenticated, "user not found")
	}

	// Update identity last login
	now := time.Now()
	identity.LastLoginAt = &now
	if err := s.identityRepo.Update(ctx, identity); err != nil {
		log.Printf("Failed to update identity last login: %v", err)
		// Non-fatal, continue
	}

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
		log.Printf("Failed to store API token: %v", err)
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
	log.Printf("[DEBUG] RefreshToken called with TokenID: %s", req.TokenId)

	// Get existing token
	log.Printf("[DEBUG] Calling tokenRepo.GetByID for token: %s", req.TokenId)
	existingToken, err := s.tokenRepo.GetByID(ctx, req.TokenId)
	log.Printf("[DEBUG] tokenRepo.GetByID returned: existingToken=%v, err=%v", existingToken != nil, err)

	if err != nil {
		log.Printf("[ERROR] Failed to get token: %v", err)
		return nil, status.Error(codes.NotFound, "token not found")
	}
	if existingToken == nil {
		log.Printf("[ERROR] Token is nil (not found in database)")
		return nil, status.Error(codes.NotFound, "token not found")
	}

	log.Printf("[DEBUG] Token found for user: %s, revoked: %v", existingToken.UserID, existingToken.RevokedAt != nil)

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
		// Get the user's identity to find the provider
		identities, err := s.identityRepo.ListByUserID(ctx, user.ID)
		if err != nil || len(identities) == 0 {
			return nil, status.Error(codes.Internal, "failed to get user identities")
		}

		// Use the first (most recent) identity's provider
		provider := identities[0].Provider

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

		log.Printf("[DEBUG] OAuth2 config for refresh - ClientID: %s, Endpoint: %v", oauth2Config.ClientID, oauth2Config.Endpoint)
		log.Printf("[DEBUG] Refresh token length: %d, starts with: %s", len(*oidcSession.RefreshToken), (*oidcSession.RefreshToken)[:min(20, len(*oidcSession.RefreshToken))])

		// Exchange refresh token for new access token
		token, err := oauth2Config.TokenSource(ctx, &oauth2.Token{
			RefreshToken: *oidcSession.RefreshToken,
		}).Token()
		if err != nil {
			log.Printf("[ERROR] Failed to refresh OAuth token for user %s provider %s: %v", user.ID, provider, err)
			return nil, status.Error(codes.Unauthenticated, "failed to refresh OAuth token - please login again")
		}

		// Update the OIDC session with potentially rotated refresh token
		if token.RefreshToken != "" && token.RefreshToken != *oidcSession.RefreshToken {
			oidcSession.RefreshToken = &token.RefreshToken
			now := time.Now()
			oidcSession.LastRefreshed = &now
			if err := s.sessionRepo.UpdateOIDCSession(ctx, oidcSession); err != nil {
				fmt.Printf("warning: failed to update OIDC session: %v\n", err)
			}
		}

		// Note: For Google, we could extract and validate a new ID token here
		// For now, we trust that the refresh worked and just issue a new JWT
	}

	// Generate new JWT with user profile information (including avatar)
	log.Printf("[DEBUG] Generating new JWT for user %s with tokenID %s", user.ID, existingToken.ID)
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
	log.Printf("[DEBUG] Generated new JWT (preview): %s..., expires at %v", tokenString[:min(30, len(tokenString))], expiresAt)

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

	// Try to get existing identity
	existingIdentity, err := s.identityRepo.GetByProviderAndExternalID(ctx, req.Provider, claims.Subject)

	var user *entities.User
	var isNewUser bool

	if err == nil && existingIdentity != nil {
		// Identity exists, get the associated user
		user, err = s.userRepo.GetByID(ctx, existingIdentity.UserID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get user: %v", err)
		}
		if user == nil {
			return nil, status.Error(codes.NotFound, "user not found")
		}
	} else {
		// Identity doesn't exist, check if we should link to existing user by email
		existingUser, err := s.userRepo.GetByEmail(ctx, claims.Email)

		if err == nil && existingUser != nil {
			// User with this email exists - link the new identity
			user = existingUser

			// Create new identity for this provider
			now := time.Now()
			identity := &entities.Identity{
				IdentityID:        idgen.GenerateID(),
				UserID:            user.ID,
				Provider:          req.Provider,
				ExternalID:        claims.Subject,
				Email:             claims.Email,
				EmailVerified:     claims.EmailVerified,
				DisplayName:       claims.Name,
				ProfilePictureURL: claims.Picture,
				CreatedAt:         time.Now(),
				LastLoginAt:       &now,
			}

			if err := s.identityRepo.Create(ctx, identity); err != nil {
				return nil, status.Errorf(codes.Internal, "failed to create identity: %v", err)
			}
		} else {
			// No user exists - create new user and identity (auto-provisioning)
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
				return nil, status.Errorf(codes.Internal, "failed to create user: %v", err)
			}

			// Create identity
			now := time.Now()
			identity := &entities.Identity{
				IdentityID:        idgen.GenerateID(),
				UserID:            user.ID,
				Provider:          req.Provider,
				ExternalID:        claims.Subject,
				Email:             claims.Email,
				EmailVerified:     claims.EmailVerified,
				DisplayName:       claims.Name,
				ProfilePictureURL: claims.Picture,
				CreatedAt:         time.Now(),
				LastLoginAt:       &now,
			}

			if err := s.identityRepo.Create(ctx, identity); err != nil {
				return nil, status.Errorf(codes.Internal, "failed to create identity: %v", err)
			}

			isNewUser = true
		}
	}

	// Check if user is active
	if !user.IsActive {
		return nil, status.Error(codes.PermissionDenied, "user account is not active")
	}

	// Update last login time for the identity
	if existingIdentity != nil {
		now := time.Now()
		existingIdentity.LastLoginAt = &now
		if err := s.identityRepo.Update(ctx, existingIdentity); err != nil {
			// Log error but don't fail the login
			fmt.Printf("warning: failed to update identity last login: %v\n", err)
		}
	}

	// Generate token ID
	tokenID, err := auth.GenerateTokenID()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate token ID: %v", err)
	}

	// Get display name and picture from identity or claims
	displayName := ""
	picture := ""
	if existingIdentity != nil {
		displayName = existingIdentity.DisplayName
		picture = existingIdentity.ProfilePictureURL
	} else if claims != nil {
		displayName = claims.Name
		picture = claims.Picture
	}

	// Update user's avatar URL with the picture from this login
	if picture != "" && (user.AvatarURL == nil || (user.AvatarURL != nil && *user.AvatarURL != picture)) {
		user.AvatarURL = &picture
		if err := s.userRepo.Update(ctx, user); err != nil {
			log.Printf("Warning: failed to update user avatar URL: %v", err)
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
			fmt.Printf("warning: failed to get existing OIDC session: %v\n", err)
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
				fmt.Printf("warning: failed to update OIDC session: %v\n", err)
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
				fmt.Printf("warning: failed to create OIDC session: %v\n", err)
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
