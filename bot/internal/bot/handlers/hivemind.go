package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
	discordpb "github.com/devilmonastery/hivemind/api/generated/go/discordpb"
	"github.com/devilmonastery/hivemind/internal/client"
)

func handleHivemind(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger, grpcClient *client.Client) {
	if i.Member == nil {
		respondError(s, i, "This command can only be used in servers", log)
		return
	}

	// Get subcommand
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		respondError(s, i, "No subcommand specified", log)
		return
	}

	switch options[0].Name {
	case "setup-announcements":
		handleSetupAnnouncements(s, i, options[0], log, grpcClient)
	case "show":
		handleShowConfig(s, i, log, grpcClient)
	default:
		respondError(s, i, "Unknown subcommand", log)
	}
}

func handleSetupAnnouncements(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption, log *slog.Logger, grpcClient *client.Client) {
	// Acknowledge immediately
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Error("Failed to acknowledge interaction", "error", err)
		return
	}

	ctx := context.Background()
	discordClient := discordpb.NewDiscordServiceClient(grpcClient.Conn())

	var channelID string
	var channelName string
	var enabled bool

	// Check if channel was provided
	if len(subcommand.Options) > 0 {
		channel := subcommand.Options[0].ChannelValue(s)
		channelID = channel.ID
		channelName = channel.Name
		enabled = true
	} else {
		// No channel = disable announcements
		enabled = false
	}

	// Update guild settings via gRPC
	_, err = discordClient.UpdateGuildSettings(ctx, &discordpb.UpdateGuildSettingsRequest{
		GuildId: i.GuildID,
		Settings: &discordpb.GuildSettings{
			Announcements: &discordpb.AnnouncementSettings{
				Enabled:           enabled,
				ChannelId:         channelID,
				NotifyWikiCreate:  true,
				NotifyWikiEdit:    false,
				NotifyQuoteCreate: true,
			},
		},
	})
	if err != nil {
		log.Error("Failed to update guild settings", "error", err, "guild_id", i.GuildID)
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "❌ Failed to update settings. Please try again.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	// Success response
	var content string
	if enabled {
		content = fmt.Sprintf("✅ Announcements enabled!\n\nNew wikis and quotes will be posted to <#%s>", channelID)
	} else {
		content = "✅ Announcements disabled"
	}

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: content,
		Flags:   discordgo.MessageFlagsEphemeral,
	})
	if err != nil {
		log.Error("Failed to send followup", "error", err)
	}

	log.Info("Updated guild announcement settings",
		"guild_id", i.GuildID,
		"enabled", enabled,
		"channel_id", channelID,
		"channel_name", channelName,
		"admin_id", i.Member.User.ID,
	)
}

func handleShowConfig(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger, grpcClient *client.Client) {
	// Acknowledge immediately
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Error("Failed to acknowledge interaction", "error", err)
		return
	}

	ctx := context.Background()
	discordClient := discordpb.NewDiscordServiceClient(grpcClient.Conn())

	// Fetch guild info for name
	guildResp, err := discordClient.GetGuild(ctx, &discordpb.GetGuildRequest{
		GuildId: i.GuildID,
	})
	if err != nil {
		log.Error("Failed to fetch guild info", "error", err, "guild_id", i.GuildID)
	}

	guildName := i.GuildID
	if guildResp != nil && guildResp.Guild != nil {
		guildName = guildResp.Guild.GuildName
	}

	// Fetch guild settings
	resp, err := discordClient.GetGuildSettings(ctx, &discordpb.GetGuildSettingsRequest{
		GuildId: i.GuildID,
	})
	if err != nil {
		log.Error("Failed to fetch guild settings", "error", err, "guild_id", i.GuildID)
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "❌ Failed to fetch settings. Please try again.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	// Build embed
	embed := &discordgo.MessageEmbed{
		Title:       "📋 Hivemind Configuration",
		Description: fmt.Sprintf("Settings for **%s**", guildName),
		Color:       0x00D9FF, // Cyan
		Fields:      []*discordgo.MessageEmbedField{},
	}

	// Announcements section
	if resp.Settings != nil && resp.Settings.Announcements != nil {
		ann := resp.Settings.Announcements
		var status string
		if ann.Enabled && ann.ChannelId != "" {
			status = fmt.Sprintf("✅ Enabled\nChannel: <#%s>", ann.ChannelId)
		} else {
			status = "❌ Disabled"
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "🔔 Announcements",
			Value:  status,
			Inline: false,
		})

		if ann.Enabled && ann.ChannelId != "" {
			var notifications []string
			if ann.NotifyWikiCreate {
				notifications = append(notifications, "• 📚 New wikis")
			}
			if ann.NotifyWikiEdit {
				notifications = append(notifications, "• ✏️ Wiki edits")
			}
			if ann.NotifyQuoteCreate {
				notifications = append(notifications, "• 💬 New quotes")
			}

			if len(notifications) > 0 {
				embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
					Name:   "Notification Types",
					Value:  strings.Join(notifications, "\n"),
					Inline: false,
				})
			}
		}
	} else {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "🔔 Announcements",
			Value:  "❌ Not configured",
			Inline: false,
		})
	}

	embed.Footer = &discordgo.MessageEmbedFooter{
		Text: "Use /hivemind setup-announcements to configure",
	}

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embed},
		Flags:  discordgo.MessageFlagsEphemeral,
	})
	if err != nil {
		log.Error("Failed to send followup", "error", err)
	}
}
