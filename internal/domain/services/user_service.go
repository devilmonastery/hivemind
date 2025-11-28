package services

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
)

// UserService provides business logic for user management
type UserService struct {
	userRepo  repositories.UserRepository
	auditRepo repositories.AuditRepository
}

// NewUserService creates a new user service
func NewUserService(userRepo repositories.UserRepository, auditRepo repositories.AuditRepository) *UserService {
	return &UserService{
		userRepo:  userRepo,
		auditRepo: auditRepo,
	}
}

// NewUserServiceMinimal creates a new user service without audit logging (for initial wiring)
func NewUserServiceMinimal(userRepo repositories.UserRepository) *UserService {
	return &UserService{
		userRepo:  userRepo,
		auditRepo: nil, // Will be wired later
	}
}

// auditLog is a helper method that logs audit events if auditRepo is available
func (s *UserService) auditLog(ctx context.Context, auditLog *entities.AuditLog) error {
	if s.auditRepo == nil {
		return nil // Skip audit logging if not configured
	}
	return s.auditRepo.Create(ctx, auditLog)
}

// CreateLocalUser creates a new local user with password
func (s *UserService) CreateLocalUser(ctx context.Context, email, displayName, password string, role entities.Role) (*entities.User, error) {
	// Check if user already exists
	exists, err := s.userRepo.ExistsByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("failed to check if user exists: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("user with email %s already exists", email)
	}

	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user entity
	user := &entities.User{
		Email:        email,
		DisplayName:  displayName,
		Role:         role,
		UserType:     entities.UserTypeLocal,
		IsActive:     true,
		PasswordHash: (*string)(&[]string{string(passwordHash)}[0]),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Create user in repository
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Log user creation
	auditLog := entities.NewAuditLog(nil, entities.ActionUserCreated, entities.ResourceUser).
		WithResourceID(user.ID).
		WithMetadata("email", email).
		WithMetadata("display_name", displayName).
		WithMetadata("role", string(role)).
		WithMetadata("user_type", string(entities.UserTypeLocal))

	if err := s.auditLog(ctx, auditLog); err != nil {
		// Log audit failure but don't fail the operation
		// In production, you might want to log this error
	}

	// Clear password hash from returned user for security
	user.PasswordHash = nil
	return user, nil
}

// CreateOIDCUser creates a new OIDC user
func (s *UserService) CreateOIDCUser(ctx context.Context, email, displayName, oidcSubject, oidcProvider string) (*entities.User, error) {
	// Check if user already exists by email
	exists, err := s.userRepo.ExistsByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("failed to check if user exists: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("user with email %s already exists", email)
	}

	// Check if OIDC subject already exists
	existingUser, err := s.userRepo.GetByOIDCSubject(ctx, oidcSubject)
	if err == nil && existingUser != nil {
		return nil, fmt.Errorf("user with OIDC subject already exists")
	}

	// Create user entity
	user := &entities.User{
		Email:        email,
		DisplayName:  displayName,
		Role:         entities.RoleUser, // Default role for OIDC users
		UserType:     entities.UserTypeOIDC,
		IsActive:     true,
		OIDCSubject:  &oidcSubject,
		OIDCProvider: &oidcProvider,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Create user in repository
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Log user creation
	auditLog := entities.NewAuditLog(nil, entities.ActionUserCreated, entities.ResourceUser).
		WithResourceID(user.ID).
		WithMetadata("email", email).
		WithMetadata("display_name", displayName).
		WithMetadata("oidc_subject", oidcSubject).
		WithMetadata("oidc_provider", oidcProvider).
		WithMetadata("user_type", string(entities.UserTypeOIDC))

	if err := s.auditRepo.Create(ctx, auditLog); err != nil {
		// Log audit failure but don't fail the operation
	}

	return user, nil
}

// AuthenticateLocalUser authenticates a local user with password
func (s *UserService) AuthenticateLocalUser(ctx context.Context, email, password string, ipAddress, userAgent *string) (*entities.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		reason := GetUserLookupFailureReason(err)
		var userID *string

		// Log failed login attempt
		auditLog := entities.NewAuditLog(userID, entities.ActionUserLoginFailed, entities.ResourceUser).
			WithMetadata("email", email).
			WithMetadata("reason", reason)
		if ipAddress != nil {
			auditLog = auditLog.WithIPAddress(*ipAddress)
		}
		if userAgent != nil {
			auditLog = auditLog.WithUserAgent(*userAgent)
		}
		s.auditRepo.Create(ctx, auditLog)

		if IsUserInactive(err) {
			return nil, fmt.Errorf("user account is inactive")
		}
		return nil, fmt.Errorf("invalid credentials")
	} // Check if user is local type
	if !user.IsLocalUser() {
		auditLog := entities.NewAuditLog(&user.ID, entities.ActionUserLoginFailed, entities.ResourceUser).
			WithResourceID(user.ID).
			WithMetadata("email", email).
			WithMetadata("reason", "not_local_user")
		if ipAddress != nil {
			auditLog = auditLog.WithIPAddress(*ipAddress)
		}
		if userAgent != nil {
			auditLog = auditLog.WithUserAgent(*userAgent)
		}
		s.auditRepo.Create(ctx, auditLog)

		return nil, fmt.Errorf("invalid credentials")
	}

	// Check password
	if user.PasswordHash == nil {
		auditLog := entities.NewAuditLog(&user.ID, entities.ActionUserLoginFailed, entities.ResourceUser).
			WithResourceID(user.ID).
			WithMetadata("email", email).
			WithMetadata("reason", "no_password_set")
		if ipAddress != nil {
			auditLog = auditLog.WithIPAddress(*ipAddress)
		}
		if userAgent != nil {
			auditLog = auditLog.WithUserAgent(*userAgent)
		}
		s.auditRepo.Create(ctx, auditLog)

		return nil, fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password)); err != nil {
		auditLog := entities.NewAuditLog(&user.ID, entities.ActionUserLoginFailed, entities.ResourceUser).
			WithResourceID(user.ID).
			WithMetadata("email", email).
			WithMetadata("reason", "invalid_password")
		if ipAddress != nil {
			auditLog = auditLog.WithIPAddress(*ipAddress)
		}
		if userAgent != nil {
			auditLog = auditLog.WithUserAgent(*userAgent)
		}
		s.auditRepo.Create(ctx, auditLog)

		return nil, fmt.Errorf("invalid credentials")
	}

	// Update last login
	now := time.Now()
	if err := s.userRepo.UpdateLastLogin(ctx, user.ID, now); err != nil {
		// Log error but don't fail authentication
	}

	// Log successful login
	auditLog := entities.NewAuditLog(&user.ID, entities.ActionUserLogin, entities.ResourceUser).
		WithResourceID(user.ID).
		WithMetadata("email", email).
		WithMetadata("auth_method", "local")
	if ipAddress != nil {
		auditLog = auditLog.WithIPAddress(*ipAddress)
	}
	if userAgent != nil {
		auditLog = auditLog.WithUserAgent(*userAgent)
	}
	s.auditRepo.Create(ctx, auditLog)

	// Clear password hash from returned user for security
	user.PasswordHash = nil
	user.LastLogin = &now
	return user, nil
}

// GetOrCreateOIDCUser gets an existing OIDC user or creates a new one
func (s *UserService) GetOrCreateOIDCUser(ctx context.Context, email, displayName, oidcSubject, oidcProvider string) (*entities.User, error) {
	// Try to get existing user by OIDC subject first
	user, err := s.userRepo.GetByOIDCSubject(ctx, oidcSubject)
	if err == nil && user != nil {
		// Update last login
		now := time.Now()
		if err := s.userRepo.UpdateLastLogin(ctx, user.ID, now); err != nil {
			// Log error but don't fail operation
		}
		user.LastLogin = &now
		return user, nil
	}

	// Try to get existing user by email
	user, err = s.userRepo.GetByEmail(ctx, email)
	if err == nil && user != nil {
		// User exists with this email but different OIDC subject
		// This could be a security issue - log it
		auditLog := entities.NewAuditLog(&user.ID, entities.ActionOIDCFailed, entities.ResourceUser).
			WithResourceID(user.ID).
			WithMetadata("email", email).
			WithMetadata("oidc_subject", oidcSubject).
			WithMetadata("oidc_provider", oidcProvider).
			WithMetadata("reason", "email_already_exists")
		s.auditRepo.Create(ctx, auditLog)

		return nil, fmt.Errorf("user with email %s already exists with different authentication method", email)
	}

	// User doesn't exist, create new OIDC user
	return s.CreateOIDCUser(ctx, email, displayName, oidcSubject, oidcProvider)
}

// GetUserByID retrieves a user by ID
func (s *UserService) GetUserByID(ctx context.Context, userID string) (*entities.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Clear password hash for security
	user.PasswordHash = nil
	return user, nil
}

// ListUsers lists users with filtering and pagination
func (s *UserService) ListUsers(ctx context.Context, opts repositories.ListUsersOptions) ([]*entities.User, int64, error) {
	users, total, err := s.userRepo.List(ctx, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}

	// Clear password hashes for security
	for _, user := range users {
		user.PasswordHash = nil
	}

	return users, total, nil
}

// UpdateUser updates user information
func (s *UserService) UpdateUser(ctx context.Context, userID string, updates map[string]interface{}, updatedBy string) (*entities.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	originalRole := user.Role
	updated := false

	// Apply updates
	if displayName, ok := updates["display_name"].(string); ok && displayName != user.DisplayName {
		user.DisplayName = displayName
		updated = true
	}

	if role, ok := updates["role"].(entities.Role); ok && role != user.Role {
		user.Role = role
		updated = true
	}

	if isActive, ok := updates["is_active"].(bool); ok && isActive != user.IsActive {
		user.IsActive = isActive
		updated = true
	}

	if !updated {
		user.PasswordHash = nil
		return user, nil
	}

	user.UpdatedAt = time.Now()
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Log user update
	auditLog := entities.NewAuditLog(&updatedBy, entities.ActionUserUpdated, entities.ResourceUser).
		WithResourceID(user.ID).
		WithMetadata("updated_by", updatedBy)

	if originalRole != user.Role {
		auditLog = auditLog.WithMetadata("role_changed", map[string]string{
			"from": string(originalRole),
			"to":   string(user.Role),
		})
	}

	if err := s.auditRepo.Create(ctx, auditLog); err != nil {
		// Log audit failure but don't fail the operation
	}

	// Clear password hash for security
	user.PasswordHash = nil
	return user, nil
}

// DeactivateUser deactivates a user account
func (s *UserService) DeactivateUser(ctx context.Context, userID, deactivatedBy string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if !user.Active() {
		return nil // Already inactive
	}

	user.IsActive = false
	user.UpdatedAt = time.Now()

	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to deactivate user: %w", err)
	}

	// Log user deactivation
	auditLog := entities.NewAuditLog(&deactivatedBy, entities.ActionUserUpdated, entities.ResourceUser).
		WithResourceID(user.ID).
		WithMetadata("deactivated_by", deactivatedBy).
		WithMetadata("action", "deactivated")

	if err := s.auditRepo.Create(ctx, auditLog); err != nil {
		// Log audit failure but don't fail the operation
	}

	return nil
}
