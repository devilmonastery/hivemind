package handlers

import (
	"log/slog"

	"github.com/bwmarrin/discordgo"
	"github.com/devilmonastery/hivemind/bot/internal/config"
)

// addReactionIfEnabled adds a reaction to a message if reactions are enabled
// Does not return error - logs and continues to avoid breaking main operations
func addReactionIfEnabled(
	s *discordgo.Session,
	cfg *config.Config,
	channelID string,
	messageID string,
	emojiID string,
	contentType string, // "quote", "wiki", or "note"
	log *slog.Logger,
) {
	// Check if reactions are enabled
	if !cfg.Features.Reactions.Enabled {
		log.Debug("reactions disabled, skipping",
			slog.String("content_type", contentType),
			slog.String("message_id", messageID))
		return
	}

	// Check if emoji ID is configured
	if emojiID == "" {
		log.Warn("emoji ID not configured for content type",
			slog.String("content_type", contentType))
		return
	}

	// Format emoji for reaction (application emoji uses just the ID)
	// Discord format: name:id or just id for application emoji
	emojiFormat := emojiID

	log.Debug("adding reaction to message",
		slog.String("content_type", contentType),
		slog.String("channel_id", channelID),
		slog.String("message_id", messageID),
		slog.String("emoji_id", emojiID))

	err := s.MessageReactionAdd(channelID, messageID, emojiFormat)
	if err != nil {
		// Log but don't fail - reaction is nice-to-have
		log.Warn("failed to add reaction to message",
			slog.String("content_type", contentType),
			slog.String("channel_id", channelID),
			slog.String("message_id", messageID),
			slog.String("error", err.Error()))
		return
	}

	log.Debug("successfully added reaction",
		slog.String("content_type", contentType),
		slog.String("message_id", messageID))
}

// Convenience wrappers for each content type

func addQuoteReaction(s *discordgo.Session, cfg *config.Config, channelID, messageID string, log *slog.Logger) {
	addReactionIfEnabled(s, cfg, channelID, messageID, cfg.Features.Reactions.QuoteEmojiID, "quote", log)
}

func addWikiReaction(s *discordgo.Session, cfg *config.Config, channelID, messageID string, log *slog.Logger) {
	addReactionIfEnabled(s, cfg, channelID, messageID, cfg.Features.Reactions.WikiEmojiID, "wiki", log)
}

func addNoteReaction(s *discordgo.Session, cfg *config.Config, channelID, messageID string, log *slog.Logger) {
	addReactionIfEnabled(s, cfg, channelID, messageID, cfg.Features.Reactions.HivemindEmojiID, "note", log)
}
