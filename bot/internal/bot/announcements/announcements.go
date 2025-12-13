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
func PostWikiCreated(s *discordgo.Session, grpcClient *client.Client, guildID, title, authorName, wikiID, webBaseURL string, log *slog.Logger) {
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

	// Add web link if web base URL is configured
	if webBaseURL != "" {
		embed.URL = fmt.Sprintf("%s/wiki/%s", webBaseURL, wikiID)
	}

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
func PostQuoteCreated(s *discordgo.Session, grpcClient *client.Client, guildID, quoteBody, quoteAuthorName, saverName, quoteID, sourceChannelID, sourceMessageID string, log *slog.Logger) {
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
	var description string
	if quoteAuthorName != "" {
		description = fmt.Sprintf("%s said:\n> %s\n\n*Saved by %s*", quoteAuthorName, displayBody, saverName)
	} else {
		description = fmt.Sprintf("> %s\n\n*Saved by %s*", displayBody, saverName)
	}

	embed := &discordgo.MessageEmbed{
		Title:       "💬 New Quote Added",
		Description: description,
		Color:       0xFF00D9, // Magenta
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Use /quote random to see quotes",
		},
	}

	// Add link to original message if available
	if sourceChannelID != "" && sourceMessageID != "" {
		messageURL := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", guildID, sourceChannelID, sourceMessageID)
		embed.Description += fmt.Sprintf("\n\n[Jump to original message](%s)", messageURL)
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
