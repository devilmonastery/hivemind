package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
	"github.com/devilmonastery/hivemind/internal/pkg/idgen"
)

// DiscordService handles Discord-related business logic
type DiscordService struct {
	discordUserRepo repositories.DiscordUserRepository
	userRepo        repositories.UserRepository
	logger          *slog.Logger
}

// NewDiscordService creates a new Discord service
func NewDiscordService(
	discordUserRepo repositories.DiscordUserRepository,
	userRepo repositories.UserRepository,
	logger *slog.Logger,
) *DiscordService {
	return &DiscordService{
		discordUserRepo: discordUserRepo,
		userRepo:        userRepo,
		logger:          logger,
	}
}

// GetOrCreateUserFromDiscord gets or creates a Hivemind user from Discord info
// This implements auto-provisioning: first time a Discord user interacts, we create their account
func (s *DiscordService) GetOrCreateUserFromDiscord(
	ctx context.Context,
	discordID string,
	discordUsername string,
	discordGlobalName *string,
	avatarURL *string,
) (*entities.User, error) {
	// Try to find existing Discord user mapping
	discordUser, err := s.discordUserRepo.GetByDiscordID(ctx, discordID)
	if err == nil {
		// Found existing mapping - update last seen and return the Hivemind user
		if err := s.discordUserRepo.UpdateLastSeen(ctx, discordID); err != nil {
			s.logger.Warn("failed to update last seen",
				slog.String("discord_id", discordID),
				slog.String("error", err.Error()))
		}

		// Get the Hivemind user
		user, err := s.userRepo.GetByID(ctx, discordUser.UserID)
		if err != nil {
			return nil, fmt.Errorf("failed to get user: %w", err)
		}

		return user, nil
	}

	// If error is not "not found", return it
	if err != repositories.ErrDiscordUserNotFound {
		return nil, fmt.Errorf("failed to query discord user: %w", err)
	}

	// User doesn't exist - create new Hivemind user and link Discord identity
	s.logger.Info("auto-provisioning new user from Discord",
		slog.String("discord_id", discordID),
		slog.String("discord_username", discordUsername))

	// Create Hivemind user
	displayName := discordUsername
	if discordGlobalName != nil && *discordGlobalName != "" {
		displayName = *discordGlobalName
	}

	user := &entities.User{
		ID:          idgen.GenerateID(),
		Email:       "", // NULL email - will be populated later if user does OAuth
		DisplayName: displayName,
		AvatarURL:   avatarURL,
		Role:        entities.RoleUser,
		UserType:    entities.UserTypeOIDC, // Discord users are like OIDC users
		IsActive:    true,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Create Discord user mapping
	now := time.Now()
	discordUser = &entities.DiscordUser{
		DiscordID:         discordID,
		UserID:            user.ID,
		DiscordUsername:   discordUsername,
		DiscordGlobalName: discordGlobalName,
		AvatarURL:         avatarURL,
		LinkedAt:          now,
		LastSeen:          &now,
	}

	if err := s.discordUserRepo.Create(ctx, discordUser); err != nil {
		// Try to clean up the user we just created
		_ = s.userRepo.Delete(ctx, user.ID)
		return nil, fmt.Errorf("failed to create discord user mapping: %w", err)
	}

	s.logger.Info("user auto-provisioned from Discord",
		slog.String("user_id", user.ID),
		slog.String("discord_id", discordID),
		slog.String("username", discordUsername))

	return user, nil
}

// UpdateDiscordUserInfo updates the cached Discord user information
func (s *DiscordService) UpdateDiscordUserInfo(
	ctx context.Context,
	discordID string,
	discordUsername string,
	discordGlobalName *string,
	avatarURL *string,
) error {
	discordUser, err := s.discordUserRepo.GetByDiscordID(ctx, discordID)
	if err != nil {
		return fmt.Errorf("failed to get discord user: %w", err)
	}

	// Update fields
	discordUser.DiscordUsername = discordUsername
	discordUser.DiscordGlobalName = discordGlobalName
	discordUser.AvatarURL = avatarURL
	now := time.Now()
	discordUser.LastSeen = &now

	if err := s.discordUserRepo.Update(ctx, discordUser); err != nil {
		return fmt.Errorf("failed to update discord user: %w", err)
	}

	return nil
}
