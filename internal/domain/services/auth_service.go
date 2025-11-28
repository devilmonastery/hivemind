package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
)

// AuthService provides business logic for authentication workflows
type AuthService struct {
	userRepo    repositories.UserRepository
	tokenRepo   repositories.TokenRepository
	sessionRepo repositories.SessionRepository
	auditRepo   repositories.AuditRepository
	tokenSvc    *TokenService
}

// NewAuthService creates a new auth service
func NewAuthService(
	userRepo repositories.UserRepository,
	tokenRepo repositories.TokenRepository,
	sessionRepo repositories.SessionRepository,
	auditRepo repositories.AuditRepository,
	tokenSvc *TokenService,
) *AuthService {
	return &AuthService{
		userRepo:    userRepo,
		tokenRepo:   tokenRepo,
		sessionRepo: sessionRepo,
		auditRepo:   auditRepo,
		tokenSvc:    tokenSvc,
	}
}

// StartOIDCFlow initiates an OIDC authentication flow
func (s *AuthService) StartOIDCFlow(ctx context.Context, redirectURI string, scopes []string) (*entities.OIDCSession, error) {
	// Generate state and nonce
	state, err := s.generateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}

	nonce, err := s.generateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Generate PKCE code verifier
	codeVerifier, err := s.generateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate code verifier: %w", err)
	}

	// Create session
	session := &entities.OIDCSession{
		State:        state,
		Nonce:        nonce,
		CodeVerifier: codeVerifier,
		RedirectURI:  redirectURI,
		Scopes:       scopes,
		ExpiresAt:    time.Now().Add(30 * time.Minute), // 30 minute expiry
		CreatedAt:    time.Now(),
	}

	if err := s.sessionRepo.CreateOIDCSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create OIDC session: %w", err)
	}

	// Log OIDC start
	auditLog := entities.NewAuditLog(nil, entities.ActionOIDCStart, entities.ResourceOIDCSession).
		WithResourceID(session.ID).
		WithMetadata("redirect_uri", redirectURI).
		WithMetadata("scopes", scopes)

	if err := s.auditRepo.Create(ctx, auditLog); err != nil {
		// Log audit failure but don't fail the operation
	}

	return session, nil
}

// CompleteOIDCFlow completes an OIDC flow with tokens
func (s *AuthService) CompleteOIDCFlow(ctx context.Context, state string, userID string, idToken, accessToken, refreshToken *string, ipAddress, userAgent *string) error {
	session, err := s.sessionRepo.GetOIDCSessionByState(ctx, state)
	if err != nil {
		return fmt.Errorf("invalid state parameter")
	}

	if session.IsExpired() {
		auditLog := entities.NewAuditLog(&userID, entities.ActionOIDCFailed, entities.ResourceOIDCSession).
			WithResourceID(session.ID).
			WithMetadata("user_id", userID).
			WithMetadata("reason", "expired")
		if ipAddress != nil {
			auditLog = auditLog.WithIPAddress(*ipAddress)
		}
		if userAgent != nil {
			auditLog = auditLog.WithUserAgent(*userAgent)
		}
		s.auditRepo.Create(ctx, auditLog)

		return fmt.Errorf("OIDC session has expired")
	}

	if session.IsComplete() {
		return fmt.Errorf("OIDC session already completed")
	}

	// Complete the session
	session.Complete(userID, idToken, accessToken, refreshToken)
	if err := s.sessionRepo.UpdateOIDCSession(ctx, session); err != nil {
		return fmt.Errorf("failed to complete OIDC session: %w", err)
	}

	// Log completion
	auditLog := entities.NewAuditLog(&userID, entities.ActionOIDCComplete, entities.ResourceOIDCSession).
		WithResourceID(session.ID).
		WithMetadata("user_id", userID).
		WithMetadata("success", true)
	if ipAddress != nil {
		auditLog = auditLog.WithIPAddress(*ipAddress)
	}
	if userAgent != nil {
		auditLog = auditLog.WithUserAgent(*userAgent)
	}
	s.auditRepo.Create(ctx, auditLog)

	return nil
}

// CleanupExpiredSessions removes expired authentication sessions
func (s *AuthService) CleanupExpiredSessions(ctx context.Context, olderThan time.Duration) (int64, int64, error) {
	before := time.Now().Add(-olderThan)
	oidcDeleted, err := s.sessionRepo.CleanupExpiredSessions(ctx, before)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}

	if oidcDeleted > 0 {
		// Log cleanup
		auditLog := entities.NewAuditLog(nil, entities.ActionSystemStartup, entities.ResourceSystem).
			WithMetadata("action", "cleanup_expired_sessions").
			WithMetadata("oidc_deleted", oidcDeleted).
			WithMetadata("older_than", olderThan.String())

		s.auditRepo.Create(ctx, auditLog)
	}

	return 0, oidcDeleted, nil
}

// generateSecureToken generates a cryptographically secure random token
func (s *AuthService) generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}
