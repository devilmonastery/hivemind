package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
)

// DiscordSyncService handles bulk synchronization of Discord data
type DiscordSyncService struct {
	session         *discordgo.Session
	guildRepo       repositories.DiscordGuildRepository
	guildMemberRepo repositories.GuildMemberRepository
	discordUserRepo repositories.DiscordUserRepository
	log             *slog.Logger
}

// NewDiscordSyncService creates a new sync service
func NewDiscordSyncService(
	session *discordgo.Session,
	guildRepo repositories.DiscordGuildRepository,
	guildMemberRepo repositories.GuildMemberRepository,
	discordUserRepo repositories.DiscordUserRepository,
	log *slog.Logger,
) *DiscordSyncService {
	return &DiscordSyncService{
		session:         session,
		guildRepo:       guildRepo,
		guildMemberRepo: guildMemberRepo,
		discordUserRepo: discordUserRepo,
		log:             log,
	}
}

// SyncGuildMembers synchronizes all members for a specific guild
func (s *DiscordSyncService) SyncGuildMembers(ctx context.Context, guildID string) error {
	s.log.Info("starting member sync for guild",
		slog.String("guild_id", guildID))

	var allMembers []*entities.GuildMember
	var after string
	totalFetched := 0

	// Paginate through all guild members (Discord limit is 1000 per request)
	for {
		members, err := s.session.GuildMembers(guildID, after, 1000)
		if err != nil {
			return fmt.Errorf("failed to fetch guild members: %w", err)
		}

		if len(members) == 0 {
			break
		}

		// Convert Discord members to domain entities
		for _, m := range members {
			var guildNick *string
			if m.Nick != "" {
				guildNick = &m.Nick
			}
			var guildAvatar *string
			if m.Avatar != "" {
				guildAvatar = &m.Avatar
			}

			entity := &entities.GuildMember{
				GuildID:         guildID,
				DiscordID:       m.User.ID,
				JoinedAt:        m.JoinedAt,
				Roles:           m.Roles,
				GuildNick:       guildNick,
				GuildAvatarHash: guildAvatar,
				SyncedAt:        time.Now(),
			}
			allMembers = append(allMembers, entity)
		}

		totalFetched += len(members)
		s.log.Debug("fetched member batch",
			slog.String("guild_id", guildID),
			slog.Int("batch_size", len(members)),
			slog.Int("total_fetched", totalFetched))

		// If we got less than 1000, we're done
		if len(members) < 1000 {
			break
		}

		// Use last member's ID for pagination
		after = members[len(members)-1].User.ID
	}

	// Batch upsert all members
	if len(allMembers) > 0 {
		if err := s.guildMemberRepo.UpsertBatch(ctx, allMembers); err != nil {
			return fmt.Errorf("failed to upsert members: %w", err)
		}
	}

	// Update guild's last_member_sync timestamp
	if err := s.guildRepo.UpdateMemberSyncTime(ctx, guildID); err != nil {
		s.log.Warn("failed to update guild member sync time",
			slog.String("guild_id", guildID),
			slog.String("error", err.Error()))
	}

	s.log.Info("completed member sync for guild",
		slog.String("guild_id", guildID),
		slog.Int("total_members", len(allMembers)))

	return nil
}

// SyncAllGuildMembers synchronizes members for all active guilds
func (s *DiscordSyncService) SyncAllGuildMembers(ctx context.Context) error {
	s.log.Info("starting member sync for all guilds")

	// Get all active guilds from database
	guilds, err := s.guildRepo.List(ctx, true) // enabledOnly = true
	if err != nil {
		return fmt.Errorf("failed to list guilds: %w", err)
	}

	successCount := 0
	errorCount := 0

	for _, guild := range guilds {
		if err := s.SyncGuildMembers(ctx, guild.GuildID); err != nil {
			s.log.Error("failed to sync guild members",
				slog.String("guild_id", guild.GuildID),
				slog.String("error", err.Error()))
			errorCount++
		} else {
			successCount++
		}
	}

	s.log.Info("completed member sync for all guilds",
		slog.Int("success_count", successCount),
		slog.Int("error_count", errorCount))

	if errorCount > 0 {
		return fmt.Errorf("sync completed with %d errors", errorCount)
	}

	return nil
}
