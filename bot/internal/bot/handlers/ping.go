package handlers

import (
	"log/slog"

	"github.com/bwmarrin/discordgo"
)

// handlePing responds to the /ping command
func handlePing(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "🏓 Pong!",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Error("failed to respond to ping",
			slog.String("error", err.Error()),
			slog.String("user_id", i.Member.User.ID),
		)
	}
}
