package bot

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	discordpb "github.com/devilmonastery/hivemind/api/generated/go/discordpb"
)

// StartMemberSync starts a background goroutine that periodically syncs guild members
// Runs every hour, checking which guilds need syncing (based on 24h interval)
func (b *Bot) StartMemberSync(ctx context.Context) {
	// Check every hour instead of every 24h - coordinator handles interval logic
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	b.log.Info("starting member sync background job",
		slog.Duration("check_interval", 1*time.Hour),
		slog.Duration("sync_interval", 24*time.Hour))

	// Run initial sync immediately
	b.syncAllGuildMembers(ctx)

	for {
		select {
		case <-ctx.Done():
			b.log.Info("stopping member sync background job")
			return
		case <-ticker.C:
			b.syncAllGuildMembers(ctx)
		}
	}
}

// syncAllGuildMembers performs a full member sync for all guilds
func (b *Bot) syncAllGuildMembers(ctx context.Context) {
	b.log.Info("starting scheduled member sync check for all guilds")

	discordClient := discordpb.NewDiscordServiceClient(b.grpcClient.Conn())

	// Get all guilds the bot is in
	guilds := b.session.State.Guilds

	successCount := 0
	errorCount := 0
	skippedCount := 0

	for _, guild := range guilds {
		if err := b.syncGuildWithCoordinator(ctx, discordClient, guild.ID); err != nil {
			b.log.Error("failed to sync guild members",
				slog.String("guild_id", guild.ID),
				slog.String("error", err.Error()))
			errorCount++
		} else {
			successCount++
		}
	}

	b.log.Info("completed scheduled member sync",
		slog.Int("success_count", successCount),
		slog.Int("error_count", errorCount),
		slog.Int("skipped_count", skippedCount))
}

// syncGuildWithCoordinator syncs a guild using the coordinator for distributed locking
func (b *Bot) syncGuildWithCoordinator(ctx context.Context, discordClient discordpb.DiscordServiceClient, guildID string) error {
	// Check if sync is needed (last sync > 24h ago)
	needsSync, lastSync, err := b.syncCoordinator.NeedsSyncSince(ctx, guildID, 24*time.Hour)
	if err != nil {
		b.log.Warn("failed to check sync status, skipping",
			slog.String("guild_id", guildID),
			slog.String("error", err.Error()))
		return nil // Don't count as error, just skip
	}

	if !needsSync {
		b.log.Debug("guild does not need sync yet",
			slog.String("guild_id", guildID),
			slog.Any("last_sync", lastSync))
		return nil
	}

	// Try to acquire lease (10 minute expiry is plenty for guild member sync)
	acquired, currentHolder, err := b.syncCoordinator.TryAcquireLease(ctx, guildID, 10*time.Minute)
	if err != nil {
		return fmt.Errorf("failed to acquire lease: %w", err)
	}

	if !acquired {
		// Another instance is syncing this guild
		b.log.Debug("guild sync lease held by another instance",
			slog.String("guild_id", guildID),
			slog.String("holder", currentHolder))
		return nil
	}

	// Do the sync
	syncStart := time.Now()
	memberCount, syncErr := b.syncGuildMembersInternal(ctx, discordClient, guildID)

	// Always release the lease, recording the result
	if err := b.syncCoordinator.ReleaseLease(ctx, guildID, syncErr == nil, memberCount, syncStart); err != nil {
		b.log.Error("failed to release lease",
			slog.String("guild_id", guildID),
			slog.String("error", err.Error()))
	}

	return syncErr
}

// syncGuildMembers syncs all members for a specific guild (legacy method for backward compatibility)
func (b *Bot) syncGuildMembers(ctx context.Context, discordClient discordpb.DiscordServiceClient, guildID string) error {
	_, err := b.syncGuildMembersInternal(ctx, discordClient, guildID)
	return err
}

// syncGuildMembersInternal syncs all members for a specific guild and returns the member count
func (b *Bot) syncGuildMembersInternal(ctx context.Context, discordClient discordpb.DiscordServiceClient, guildID string) (int, error) {
	b.log.Debug("syncing guild members",
		slog.String("guild_id", guildID))

	var allMembers []*discordpb.GuildMember
	after := ""
	totalFetched := 0

	// Paginate through all members (Discord limit is 1000 per request)
	for {
		b.log.Debug("fetching guild members from Discord API",
			slog.String("guild_id", guildID),
			slog.String("after", after),
			slog.Int("limit", 1000))

		members, err := b.session.GuildMembers(guildID, after, 1000)
		if err != nil {
			return 0, err
		}

		if len(members) == 0 {
			break
		}

		// Convert to protobuf messages
		for _, m := range members {
			pbMember := &discordpb.GuildMember{
				GuildId:         guildID,
				DiscordId:       m.User.ID,
				Roles:           m.Roles,
				DiscordUsername: m.User.Username,
			}

			if m.Nick != "" {
				pbMember.GuildNick = m.Nick
			}
			if m.Avatar != "" {
				pbMember.GuildAvatarHash = m.Avatar
			}
			if m.User.GlobalName != "" {
				pbMember.DiscordGlobalName = m.User.GlobalName
			}
			if m.User.Avatar != "" {
				pbMember.AvatarHash = m.User.Avatar
			}

			allMembers = append(allMembers, pbMember)
		}

		totalFetched += len(members)
		b.log.Debug("fetched member batch",
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
		_, err := discordClient.UpsertGuildMembersBatch(ctx, &discordpb.UpsertGuildMembersBatchRequest{
			Members: allMembers,
		})
		if err != nil {
			return 0, err
		}
	}

	b.log.Info("completed guild member sync",
		slog.String("guild_id", guildID),
		slog.Int("total_members", len(allMembers)))

	return len(allMembers), nil
}

// SyncGuildMembersManual triggers a manual sync for a specific guild (for CLI)
func (b *Bot) SyncGuildMembersManual(guildID string) error {
	ctx := context.Background()
	discordClient := discordpb.NewDiscordServiceClient(b.grpcClient.Conn())

	return b.syncGuildMembers(ctx, discordClient, guildID)
}
