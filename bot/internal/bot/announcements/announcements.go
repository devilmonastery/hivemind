package announcements

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	discordpb "github.com/devilmonastery/hivemind/api/generated/go/discordpb"
	"github.com/devilmonastery/hivemind/internal/client"
)

// PostWikiCreated posts an announcement for a newly created wiki page
func PostWikiCreated(s *discordgo.Session, grpcClient *client.Client, guildID, title, authorName, wikiID string, log *slog.Logger) {
	ctx := context.Background()
	discordClient := discordpb.NewDiscordServiceClient(grpcClient.Conn())

	// Fetch guild settings
	settingsResp, err := discordClient.GetGuildSettings(ctx, &discordpb.GetGuildSettingsRequest{
		GuildId: guildID,
	})
	if err != nil {
		log.Debug("Failed to fetch guild settings for announcement", "error", err, "guild_id", guildID)
		return
	}

	// Check if announcements are enabled
	if settingsResp.Settings == nil ||
		settingsResp.Settings.Announcements == nil ||
		!settingsResp.Settings.Announcements.Enabled ||
		!settingsResp.Settings.Announcements.NotifyWikiCreate ||
		settingsResp.Settings.Announcements.ChannelId == "" {
		return
	}

	channelID := settingsResp.Settings.Announcements.ChannelId

	// Build embed
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("📚 New Wiki: %s", title),
		Description: fmt.Sprintf("Created by %s", authorName),
		Color:       0x00D9FF, // Cyan
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Use /wiki view to read it",
		},
	}

	// TODO: Add web link when available
	// embed.URL = fmt.Sprintf("https://hivemind.example.com/wiki/%s", wikiID)

	// Post message
	msg, err := s.ChannelMessageSendEmbed(channelID, embed)
	if err != nil {
		log.Warn("Failed to post wiki announcement",
			"error", err,
			"guild_id", guildID,
			"channel_id", channelID,
			"wiki_title", title)
		return
	}

	log.Info("Posted wiki announcement",
		"guild_id", guildID,
		"channel_id", channelID,
		"message_id", msg.ID,
		"wiki_title", title)
}

// PostQuoteCreated posts an announcement for a newly created quote
func PostQuoteCreated(s *discordgo.Session, grpcClient *client.Client, guildID, quoteBody, authorName, quoteID string, log *slog.Logger) {
	ctx := context.Background()
	discordClient := discordpb.NewDiscordServiceClient(grpcClient.Conn())

	// Fetch guild settings
	settingsResp, err := discordClient.GetGuildSettings(ctx, &discordpb.GetGuildSettingsRequest{
		GuildId: guildID,
	})
	if err != nil {
		log.Debug("Failed to fetch guild settings for announcement", "error", err, "guild_id", guildID)
		return
	}

	// Check if announcements are enabled
	if settingsResp.Settings == nil ||
		settingsResp.Settings.Announcements == nil ||
		!settingsResp.Settings.Announcements.Enabled ||
		!settingsResp.Settings.Announcements.NotifyQuoteCreate ||
		settingsResp.Settings.Announcements.ChannelId == "" {
		return
	}

	channelID := settingsResp.Settings.Announcements.ChannelId

	// Truncate quote if too long
	displayBody := quoteBody
	if len(displayBody) > 300 {
		displayBody = displayBody[:297] + "..."
	}

	// Build embed
	embed := &discordgo.MessageEmbed{
		Title:       "💬 New Quote Added",
		Description: fmt.Sprintf("> %s\n\n*Saved by %s*", displayBody, authorName),
		Color:       0xFF00D9, // Magenta
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Use /quote random to see quotes",
		},
	}

	// Post message
	msg, err := s.ChannelMessageSendEmbed(channelID, embed)
	if err != nil {
		log.Warn("Failed to post quote announcement",
			"error", err,
			"guild_id", guildID,
			"channel_id", channelID)
		return
	}

	log.Info("Posted quote announcement",
		"guild_id", guildID,
		"channel_id", channelID,
		"message_id", msg.ID)
}
