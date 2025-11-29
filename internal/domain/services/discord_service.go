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
	discordUserRepo  repositories.DiscordUserRepository
	discordGuildRepo repositories.DiscordGuildRepository
	userRepo         repositories.UserRepository
	logger           *slog.Logger
}

// NewDiscordService creates a new Discord service
func NewDiscordService(
	discordUserRepo repositories.DiscordUserRepository,
	discordGuildRepo repositories.DiscordGuildRepository,
	userRepo repositories.UserRepository,
	logger *slog.Logger,
) *DiscordService {
	return &DiscordService{
		discordUserRepo:  discordUserRepo,
		discordGuildRepo: discordGuildRepo,
		userRepo:         userRepo,
		logger:           logger,
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
		if updateErr := s.discordUserRepo.UpdateLastSeen(ctx, discordID); updateErr != nil {
			s.logger.Warn("failed to update last seen",
				slog.String("discord_id", discordID),
				slog.String("error", updateErr.Error()))
		}

		// Get the Hivemind user
		user, getUserErr := s.userRepo.GetByID(ctx, discordUser.UserID)
		if getUserErr != nil {
			return nil, fmt.Errorf("failed to get user: %w", getUserErr)
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

// GetOrCreateGuild gets or creates a Discord guild record
func (s *DiscordService) GetOrCreateGuild(
	ctx context.Context,
	guildID string,
	guildName string,
) (*entities.DiscordGuild, error) {
	// Try to find existing guild
	guild, err := s.discordGuildRepo.GetByID(ctx, guildID)
	if err == nil {
		// Found existing guild - update last activity
		now := time.Now()
		guild.LastActivity = &now
		if updateErr := s.discordGuildRepo.Update(ctx, guild); updateErr != nil {
			s.logger.Warn("failed to update guild last activity",
				slog.String("guild_id", guildID),
				slog.String("error", updateErr.Error()))
		}
		return guild, nil
	}

	// If error is not "not found", return it
	if err != repositories.ErrDiscordGuildNotFound {
		return nil, fmt.Errorf("failed to query discord guild: %w", err)
	}

	// Guild doesn't exist - create new one
	s.logger.Info("auto-registering new Discord guild",
		slog.String("guild_id", guildID),
		slog.String("guild_name", guildName))

	now := time.Now()
	guild = &entities.DiscordGuild{
		GuildID:      guildID,
		GuildName:    guildName,
		Enabled:      true,
		Settings:     "{}",
		AddedAt:      now,
		LastActivity: &now,
	}

	if err := s.discordGuildRepo.Create(ctx, guild); err != nil {
		return nil, fmt.Errorf("failed to create discord guild: %w", err)
	}

	s.logger.Info("guild auto-registered",
		slog.String("guild_id", guildID),
		slog.String("guild_name", guildName))

	return guild, nil
}

// UpsertGuild creates or updates a Discord guild with full information
func (s *DiscordService) UpsertGuild(
	ctx context.Context,
	guildID string,
	guildName string,
	iconURL string,
	ownerDiscordID string,
) (*entities.DiscordGuild, error) {
	// Try to find existing guild
	guild, err := s.discordGuildRepo.GetByID(ctx, guildID)
	if err == nil {
		// Update existing guild
		guild.GuildName = guildName
		if iconURL != "" {
			guild.IconURL = &iconURL
		}
		if ownerDiscordID != "" {
			guild.OwnerID = &ownerDiscordID
		}
		now := time.Now()
		guild.LastActivity = &now

		if updateErr := s.discordGuildRepo.Update(ctx, guild); updateErr != nil {
			return nil, fmt.Errorf("failed to update guild: %w", updateErr)
		}

		s.logger.Info("guild updated",
			slog.String("guild_id", guildID),
			slog.String("guild_name", guildName))

		return guild, nil
	}

	// If error is not "not found", return it
	if err != repositories.ErrDiscordGuildNotFound {
		return nil, fmt.Errorf("failed to query discord guild: %w", err)
	}

	// Create new guild
	s.logger.Info("registering new Discord guild",
		slog.String("guild_id", guildID),
		slog.String("guild_name", guildName))

	now := time.Now()
	guild = &entities.DiscordGuild{
		GuildID:      guildID,
		GuildName:    guildName,
		Enabled:      true,
		Settings:     "{}",
		AddedAt:      now,
		LastActivity: &now,
	}

	if iconURL != "" {
		guild.IconURL = &iconURL
	}
	if ownerDiscordID != "" {
		guild.OwnerID = &ownerDiscordID
	}

	if err := s.discordGuildRepo.Create(ctx, guild); err != nil {
		return nil, fmt.Errorf("failed to create discord guild: %w", err)
	}

	s.logger.Info("guild registered",
		slog.String("guild_id", guildID),
		slog.String("guild_name", guildName))

	return guild, nil
}

// DisableGuild marks a guild as disabled (when bot is removed)
func (s *DiscordService) DisableGuild(ctx context.Context, guildID string) error {
	guild, err := s.discordGuildRepo.GetByID(ctx, guildID)
	if err != nil {
		return fmt.Errorf("failed to get guild: %w", err)
	}

	guild.Enabled = false
	if err := s.discordGuildRepo.Update(ctx, guild); err != nil {
		return fmt.Errorf("failed to disable guild: %w", err)
	}

	s.logger.Info("guild disabled",
		slog.String("guild_id", guildID))

	return nil
}

// GetGuild retrieves a guild by ID
func (s *DiscordService) GetGuild(ctx context.Context, guildID string) (*entities.DiscordGuild, error) {
	return s.discordGuildRepo.GetByID(ctx, guildID)
}

// GetDiscordUserByHivemindID gets a Discord user by their Hivemind user ID
func (s *DiscordService) GetDiscordUserByHivemindID(ctx context.Context, userID string) (*entities.DiscordUser, error) {
	// Query by user_id field (the reverse lookup from GetOrCreateUserFromDiscord)
	discordUser, err := s.discordUserRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get discord user: %w", err)
	}
	return discordUser, nil
}
