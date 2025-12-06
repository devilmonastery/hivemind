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
	guildMemberRepo  repositories.GuildMemberRepository
	userRepo         repositories.UserRepository
	logger           *slog.Logger
}

// NewDiscordService creates a new Discord service
func NewDiscordService(
	discordUserRepo repositories.DiscordUserRepository,
	discordGuildRepo repositories.DiscordGuildRepository,
	guildMemberRepo repositories.GuildMemberRepository,
	userRepo repositories.UserRepository,
	logger *slog.Logger,
) *DiscordService {
	return &DiscordService{
		discordUserRepo:  discordUserRepo,
		discordGuildRepo: discordGuildRepo,
		guildMemberRepo:  guildMemberRepo,
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
	avatarURL *string, // Deprecated: Use avatarHash instead, keeping for backwards compatibility
) (*entities.User, error) {
	// Try to find existing Discord user mapping
	s.logger.Debug("GetOrCreateUserFromDiscord called",
		slog.String("discord_id", discordID),
		slog.String("discord_username", discordUsername))

	discordUser, err := s.discordUserRepo.GetByDiscordID(ctx, discordID)
	if err == nil {
		// Found existing mapping
		s.logger.Debug("found existing discord_user record",
			slog.String("discord_id", discordID),
			slog.Any("user_id", discordUser.UserID))

		if updateErr := s.discordUserRepo.UpdateLastSeen(ctx, discordID); updateErr != nil {
			s.logger.Warn("failed to update last seen",
				slog.String("discord_id", discordID),
				slog.String("error", updateErr.Error()))
		}

		// If already linked to a Hivemind user, return it
		if discordUser.UserID != nil {
			s.logger.Debug("discord_user already linked, fetching existing user",
				slog.String("user_id", *discordUser.UserID))
			user, getUserErr := s.userRepo.GetByID(ctx, *discordUser.UserID)
			if getUserErr != nil {
				return nil, fmt.Errorf("failed to get user: %w", getUserErr)
			}
			s.logger.Debug("returning existing user",
				slog.String("user_id", user.ID),
				slog.String("display_name", user.DisplayName))
			return user, nil
		}

		// Discord user exists but not linked - fall through to create and link Hivemind user
		s.logger.Info("linking existing Discord user to new Hivemind account",
			slog.String("discord_id", discordID),
			slog.String("discord_username", discordUsername))
	} else if err != repositories.ErrDiscordUserNotFound {
		// If error is not "not found", return it
		s.logger.Error("error querying discord_users",
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to query discord user: %w", err)
	} else {
		s.logger.Debug("discord_user not found, will create new",
			slog.String("discord_id", discordID))
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

	s.logger.Debug("creating new user",
		slog.String("user_id", user.ID),
		slog.String("display_name", displayName))

	if err := s.userRepo.Create(ctx, user); err != nil {
		s.logger.Error("failed to create user in database",
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	s.logger.Debug("user created successfully", slog.String("user_id", user.ID))

	// Create or update Discord user mapping
	now := time.Now()
	userIDPtr := user.ID

	if discordUser != nil {
		// Update existing discord_users record with the new UserID link
		s.logger.Debug("updating existing discord_users record",
			slog.String("discord_id", discordID),
			slog.String("new_user_id", userIDPtr),
			slog.Any("old_user_id", discordUser.UserID))

		discordUser.UserID = &userIDPtr
		discordUser.LastSeen = &now
		if err := s.discordUserRepo.Update(ctx, discordUser); err != nil {
			s.logger.Error("failed to update discord_users record",
				slog.String("error", err.Error()))
			// Try to clean up the user we just created
			_ = s.userRepo.Delete(ctx, user.ID)
			return nil, fmt.Errorf("failed to update discord user mapping: %w", err)
		}
		s.logger.Debug("discord_users record updated successfully")
	} else {
		// Create new discord_users record
		s.logger.Debug("creating new discord_users record",
			slog.String("discord_id", discordID),
			slog.String("user_id", userIDPtr))

		discordUser = &entities.DiscordUser{
			DiscordID:         discordID,
			UserID:            &userIDPtr,
			DiscordUsername:   discordUsername,
			DiscordGlobalName: discordGlobalName,
			AvatarHash:        nil, // Auto-provisioning doesn't have avatar hash
			LinkedAt:          now,
			LastSeen:          &now,
		}

		if err := s.discordUserRepo.Create(ctx, discordUser); err != nil {
			s.logger.Error("failed to create discord_users record",
				slog.String("error", err.Error()))
			// Try to clean up the user we just created
			_ = s.userRepo.Delete(ctx, user.ID)
			return nil, fmt.Errorf("failed to create discord user mapping: %w", err)
		}
		s.logger.Debug("discord_users record created successfully")
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
	avatarHash *string,
) error {
	discordUser, err := s.discordUserRepo.GetByDiscordID(ctx, discordID)
	if err != nil {
		return fmt.Errorf("failed to get discord user: %w", err)
	}

	// Update fields
	discordUser.DiscordUsername = discordUsername
	discordUser.DiscordGlobalName = discordGlobalName
	discordUser.AvatarHash = avatarHash
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

// UpsertGuildMember creates or updates a guild member record
func (s *DiscordService) UpsertGuildMember(
	ctx context.Context,
	member *entities.GuildMember,
) error {
	if err := s.guildMemberRepo.Upsert(ctx, member); err != nil {
		return err
	}

	// Refresh display names for this specific guild member
	if err := s.guildMemberRepo.RefreshDisplayNames(ctx, member.GuildID); err != nil {
		return fmt.Errorf("failed to refresh display names: %w", err)
	}

	return nil
}

// UpsertDiscordUser creates or updates a discord user record
func (s *DiscordService) UpsertDiscordUser(
	ctx context.Context,
	discordUser *entities.DiscordUser,
) error {
	return s.discordUserRepo.Upsert(ctx, discordUser)
}

// UpsertDiscordUsersBatch efficiently upserts multiple Discord users
func (s *DiscordService) UpsertDiscordUsersBatch(
	ctx context.Context,
	discordUsers []*entities.DiscordUser,
) error {
	if len(discordUsers) == 0 {
		return nil
	}

	for _, du := range discordUsers {
		if err := s.discordUserRepo.Upsert(ctx, du); err != nil {
			return fmt.Errorf("failed to upsert discord user %s: %w", du.DiscordID, err)
		}
	}

	s.logger.Info("batch upserted discord users",
		slog.Int("count", len(discordUsers)))

	return nil
}

// UpsertGuildMembersBatch efficiently upserts multiple members
func (s *DiscordService) UpsertGuildMembersBatch(
	ctx context.Context,
	members []*entities.GuildMember,
) error {
	if len(members) == 0 {
		return nil
	}

	err := s.guildMemberRepo.UpsertBatch(ctx, members)
	if err != nil {
		return fmt.Errorf("failed to batch upsert members: %w", err)
	}

	s.logger.Info("batch upserted guild members",
		slog.Int("count", len(members)),
		slog.String("guild_id", members[0].GuildID))

	// Refresh the denormalized display names table for this guild
	if err := s.guildMemberRepo.RefreshDisplayNames(ctx, members[0].GuildID); err != nil {
		return fmt.Errorf("failed to refresh display names: %w", err)
	}

	return nil
}

// RemoveGuildMember removes a member record
func (s *DiscordService) RemoveGuildMember(
	ctx context.Context,
	guildID string,
	discordID string,
) error {
	err := s.guildMemberRepo.DeleteMember(ctx, guildID, discordID)
	if err != nil && err != repositories.ErrGuildMemberNotFound {
		return fmt.Errorf("failed to remove guild member: %w", err)
	}

	s.logger.Info("guild member removed",
		slog.String("guild_id", guildID),
		slog.String("discord_id", discordID))

	return nil
}

// CheckGuildMembership checks if a user is a member of a guild
func (s *DiscordService) CheckGuildMembership(
	ctx context.Context,
	guildID string,
	discordID string,
) (bool, error) {
	return s.guildMemberRepo.IsMember(ctx, guildID, discordID)
}

// ListUserGuilds returns all guild IDs a user is a member of
func (s *DiscordService) ListUserGuilds(
	ctx context.Context,
	discordID string,
) ([]string, error) {
	return s.guildMemberRepo.ListUserGuilds(ctx, discordID)
}

// UpdateMemberLastSeen updates the last_seen timestamp for a guild member
func (s *DiscordService) UpdateMemberLastSeen(
	ctx context.Context,
	guildID string,
	discordID string,
) error {
	err := s.guildMemberRepo.UpdateLastSeen(ctx, guildID, discordID)
	if err != nil && err != repositories.ErrGuildMemberNotFound {
		// Log warning but don't fail - member might not be synced yet
		s.logger.Warn("failed to update member last seen",
			slog.String("guild_id", guildID),
			slog.String("discord_id", discordID),
			slog.String("error", err.Error()))
	}
	return nil
}

// UpdateGuildSettings updates guild-specific settings
func (s *DiscordService) UpdateGuildSettings(ctx context.Context, guildID string, settings map[string]interface{}) error {
	// Validate guild exists
	_, err := s.discordGuildRepo.GetByID(ctx, guildID)
	if err != nil {
		return fmt.Errorf("guild not found: %w", err)
	}

	// Add version if not present
	if _, ok := settings["version"]; !ok {
		settings["version"] = 1
	}

	err = s.discordGuildRepo.UpdateSettings(ctx, guildID, settings)
	if err != nil {
		return err
	}

	s.logger.Info("guild settings updated",
		slog.String("component", "discord_service"),
		slog.String("guild_id", guildID))

	return nil
}

// GetGuildSettings retrieves guild settings
func (s *DiscordService) GetGuildSettings(ctx context.Context, guildID string) (map[string]interface{}, error) {
	settings, err := s.discordGuildRepo.GetSettings(ctx, guildID)
	if err != nil {
		return nil, err
	}

	// Return empty settings if none configured
	if settings == nil {
		settings = map[string]interface{}{
			"version": 1,
		}
	}

	return settings, nil
}
