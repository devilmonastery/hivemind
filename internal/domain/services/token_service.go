package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
)

// TokenService provides business logic for API token management
type TokenService struct {
	tokenRepo repositories.TokenRepository
	userRepo  repositories.UserRepository
	auditRepo repositories.AuditRepository
}

// NewTokenService creates a new token service
func NewTokenService(tokenRepo repositories.TokenRepository, userRepo repositories.UserRepository, auditRepo repositories.AuditRepository) *TokenService {
	return &TokenService{
		tokenRepo: tokenRepo,
		userRepo:  userRepo,
		auditRepo: auditRepo,
	}
}

// CreateToken creates a new API token for a user
func (s *TokenService) CreateToken(ctx context.Context, userID, deviceName string, scopes []string, expiresIn time.Duration, createdBy string) (*entities.APIToken, string, error) {
	// Verify user exists and is active
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if IsUserInactive(err) {
			return nil, "", fmt.Errorf("cannot create token for inactive user")
		}
		return nil, "", fmt.Errorf("failed to get user: %w", err)
	}

	// Validate scopes based on user role
	if err := s.validateScopes(user, scopes); err != nil {
		return nil, "", fmt.Errorf("invalid scopes: %w", err)
	}

	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	// Create readable token with prefix
	plainToken := fmt.Sprintf("snip_%s", base64.URLEncoding.EncodeToString(tokenBytes))

	// Hash the token for storage
	hash := sha256.Sum256([]byte(plainToken))
	tokenHash := base64.URLEncoding.EncodeToString(hash[:])

	// Create token entity
	token := &entities.APIToken{
		UserID:     userID,
		TokenHash:  tokenHash,
		DeviceName: deviceName,
		Scopes:     scopes,
		ExpiresAt:  time.Now().Add(expiresIn),
		CreatedAt:  time.Now(),
	}

	// Save token
	if err := s.tokenRepo.Create(ctx, token); err != nil {
		return nil, "", fmt.Errorf("failed to create token: %w", err)
	}

	// Log token creation
	auditLog := entities.NewAuditLog(&createdBy, entities.ActionTokenCreated, entities.ResourceToken).
		WithResourceID(token.ID).
		WithMetadata("user_id", userID).
		WithMetadata("device_name", deviceName).
		WithMetadata("scopes", scopes).
		WithMetadata("expires_at", token.ExpiresAt).
		WithMetadata("created_by", createdBy)

	if err := s.auditRepo.Create(ctx, auditLog); err != nil {
		// Log audit failure but don't fail the operation
	}

	return token, plainToken, nil
}

// ValidateToken validates a token and returns the associated user if valid
func (s *TokenService) ValidateToken(ctx context.Context, plainToken string, requiredScope *entities.TokenScope, ipAddress, userAgent *string) (*entities.User, *entities.APIToken, error) {
	// Hash the provided token
	hash := sha256.Sum256([]byte(plainToken))
	tokenHash := base64.URLEncoding.EncodeToString(hash[:])

	// Get token from repository
	token, err := s.tokenRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid token")
	}

	// Check if token exists
	if token == nil {
		return nil, nil, fmt.Errorf("invalid token")
	}

	// Check if token is valid
	if !token.IsValid() {
		// Log token usage attempt
		auditLog := entities.NewAuditLog(&token.UserID, entities.ActionTokenUsed, entities.ResourceToken).
			WithResourceID(token.ID).
			WithMetadata("valid", false).
			WithMetadata("reason", getTokenInvalidReason(token))
		if ipAddress != nil {
			auditLog = auditLog.WithIPAddress(*ipAddress)
		}
		if userAgent != nil {
			auditLog = auditLog.WithUserAgent(*userAgent)
		}
		s.auditRepo.Create(ctx, auditLog)

		return nil, nil, fmt.Errorf("token is invalid")
	}

	// Check scope if required
	if requiredScope != nil && !token.HasScope(*requiredScope) {
		auditLog := entities.NewAuditLog(&token.UserID, entities.ActionTokenUsed, entities.ResourceToken).
			WithResourceID(token.ID).
			WithMetadata("valid", false).
			WithMetadata("reason", "insufficient_scope").
			WithMetadata("required_scope", string(*requiredScope))
		if ipAddress != nil {
			auditLog = auditLog.WithIPAddress(*ipAddress)
		}
		if userAgent != nil {
			auditLog = auditLog.WithUserAgent(*userAgent)
		}
		s.auditRepo.Create(ctx, auditLog)

		return nil, nil, fmt.Errorf("insufficient token scope")
	}

	// Get user and verify they're active
	user, err := s.userRepo.GetByID(ctx, token.UserID)
	if err != nil {
		reason := GetUserLookupFailureReason(err)

		auditLog := entities.NewAuditLog(&token.UserID, entities.ActionTokenUsed, entities.ResourceToken).
			WithResourceID(token.ID).
			WithMetadata("valid", false).
			WithMetadata("reason", reason)
		if ipAddress != nil {
			auditLog = auditLog.WithIPAddress(*ipAddress)
		}
		if userAgent != nil {
			auditLog = auditLog.WithUserAgent(*userAgent)
		}
		s.auditRepo.Create(ctx, auditLog)

		return nil, nil, fmt.Errorf("failed to get token user: %w", err)
	}

	// Update token last used
	token.UpdateLastUsed()
	if err := s.tokenRepo.UpdateLastUsed(ctx, token.ID, *token.LastUsed); err != nil {
		// Log error but don't fail authentication
	}

	// Log successful token usage
	auditLog := entities.NewAuditLog(&token.UserID, entities.ActionTokenUsed, entities.ResourceToken).
		WithResourceID(token.ID).
		WithMetadata("valid", true)
	if requiredScope != nil {
		auditLog = auditLog.WithMetadata("scope_used", string(*requiredScope))
	}
	if ipAddress != nil {
		auditLog = auditLog.WithIPAddress(*ipAddress)
	}
	if userAgent != nil {
		auditLog = auditLog.WithUserAgent(*userAgent)
	}
	s.auditRepo.Create(ctx, auditLog)

	// Clear password hash for security
	user.PasswordHash = nil
	return user, token, nil
}

// RevokeToken revokes a specific token
func (s *TokenService) RevokeToken(ctx context.Context, tokenID, revokedBy string) error {
	token, err := s.tokenRepo.GetByID(ctx, tokenID)
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	if token.IsRevoked() {
		return nil // Already revoked
	}

	if err := s.tokenRepo.Revoke(ctx, tokenID); err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}

	// Log token revocation
	auditLog := entities.NewAuditLog(&revokedBy, entities.ActionTokenRevoked, entities.ResourceToken).
		WithResourceID(tokenID).
		WithMetadata("revoked_by", revokedBy).
		WithMetadata("user_id", token.UserID)

	if err := s.auditRepo.Create(ctx, auditLog); err != nil {
		// Log audit failure but don't fail the operation
	}

	return nil
}

// RevokeAllUserTokens revokes all tokens for a user
func (s *TokenService) RevokeAllUserTokens(ctx context.Context, userID, revokedBy string) error {
	if err := s.tokenRepo.RevokeAllForUser(ctx, userID); err != nil {
		return fmt.Errorf("failed to revoke user tokens: %w", err)
	}

	// Log bulk token revocation
	auditLog := entities.NewAuditLog(&revokedBy, entities.ActionTokenRevoked, entities.ResourceToken).
		WithMetadata("revoked_by", revokedBy).
		WithMetadata("user_id", userID).
		WithMetadata("action", "revoke_all_user_tokens")

	if err := s.auditRepo.Create(ctx, auditLog); err != nil {
		// Log audit failure but don't fail the operation
	}

	return nil
}

// ListUserTokens lists tokens for a user
func (s *TokenService) ListUserTokens(ctx context.Context, userID string, opts repositories.ListTokensOptions) ([]*entities.APIToken, int64, error) {
	opts.UserID = &userID
	return s.tokenRepo.ListByUser(ctx, userID, opts)
}

// GetTokenByID gets a token by its ID
func (s *TokenService) GetTokenByID(ctx context.Context, tokenID string) (*entities.APIToken, error) {
	return s.tokenRepo.GetByID(ctx, tokenID)
}

// UpdateToken updates an existing token
func (s *TokenService) UpdateToken(ctx context.Context, token *entities.APIToken) error {
	return s.tokenRepo.Update(ctx, token)
}

// ListAllTokens lists all tokens (admin only)
func (s *TokenService) ListAllTokens(ctx context.Context, opts repositories.ListTokensOptions) ([]*entities.APIToken, int64, error) {
	return s.tokenRepo.List(ctx, opts)
}

// CleanupExpiredTokens removes expired tokens older than the specified duration
func (s *TokenService) CleanupExpiredTokens(ctx context.Context, olderThan time.Duration) (int64, error) {
	before := time.Now().Add(-olderThan)
	deleted, err := s.tokenRepo.DeleteExpired(ctx, before)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired tokens: %w", err)
	}

	if deleted > 0 {
		// Log cleanup
		auditLog := entities.NewAuditLog(nil, entities.ActionSystemStartup, entities.ResourceSystem).
			WithMetadata("action", "cleanup_expired_tokens").
			WithMetadata("deleted_count", deleted).
			WithMetadata("older_than", olderThan.String())

		s.auditRepo.Create(ctx, auditLog)
	}

	return deleted, nil
}

// CleanupRevokedTokens removes revoked tokens older than the specified duration
func (s *TokenService) CleanupRevokedTokens(ctx context.Context, olderThan time.Duration) (int64, error) {
	before := time.Now().Add(-olderThan)
	deleted, err := s.tokenRepo.DeleteRevokedBefore(ctx, before)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup revoked tokens: %w", err)
	}

	if deleted > 0 {
		// Log cleanup
		auditLog := entities.NewAuditLog(nil, entities.ActionSystemStartup, entities.ResourceSystem).
			WithMetadata("action", "cleanup_revoked_tokens").
			WithMetadata("deleted_count", deleted).
			WithMetadata("older_than", olderThan.String())

		s.auditRepo.Create(ctx, auditLog)
	}

	return deleted, nil
}

// validateScopes validates that the user is allowed to have the requested scopes
func (s *TokenService) validateScopes(user *entities.User, scopes []string) error {
	allowedScopes := entities.DefaultUserScopes()
	if user.IsAdmin() {
		allowedScopes = entities.AdminScopes()
	}

	// Create a map for faster lookup
	allowed := make(map[string]bool)
	for _, scope := range allowedScopes {
		allowed[scope] = true
	}

	// Check each requested scope
	for _, scope := range scopes {
		if !allowed[scope] {
			return fmt.Errorf("scope %s not allowed for user role %s", scope, user.Role)
		}
	}

	return nil
}

// getTokenInvalidReason returns a reason why a token is invalid
func getTokenInvalidReason(token *entities.APIToken) string {
	if token.IsRevoked() {
		return "revoked"
	}
	if token.IsExpired() {
		return "expired"
	}
	return "unknown"
}
