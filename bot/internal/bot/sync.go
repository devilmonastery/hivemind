package bot

import (
	"context"
	"log/slog"
	"time"

	discordpb "github.com/devilmonastery/hivemind/api/generated/go/discordpb"
)

// StartMemberSync starts a background goroutine that periodically syncs guild members
// Runs every 24 hours, syncing all enabled guilds
func (b *Bot) StartMemberSync(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	b.log.Info("starting member sync background job",
		slog.Duration("interval", 24*time.Hour))

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
	b.log.Info("starting scheduled member sync for all guilds")

	discordClient := discordpb.NewDiscordServiceClient(b.grpcClient.Conn())

	// Get all guilds the bot is in
	guilds := b.session.State.Guilds

	successCount := 0
	errorCount := 0

	for _, guild := range guilds {
		if err := b.syncGuildMembers(ctx, discordClient, guild.ID); err != nil {
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
		slog.Int("error_count", errorCount))
}

// syncGuildMembers syncs all members for a specific guild
func (b *Bot) syncGuildMembers(ctx context.Context, discordClient discordpb.DiscordServiceClient, guildID string) error {
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
			return err
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
			return err
		}
	}

	b.log.Info("completed guild member sync",
		slog.String("guild_id", guildID),
		slog.Int("total_members", len(allMembers)))

	return nil
}

// SyncGuildMembersManual triggers a manual sync for a specific guild (for CLI)
func (b *Bot) SyncGuildMembersManual(guildID string) error {
	ctx := context.Background()
	discordClient := discordpb.NewDiscordServiceClient(b.grpcClient.Conn())

	return b.syncGuildMembers(ctx, discordClient, guildID)
}
