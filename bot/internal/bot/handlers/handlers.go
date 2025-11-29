package handlers

import (
	"log/slog"

	"github.com/bwmarrin/discordgo"

	"github.com/devilmonastery/hivemind/bot/internal/config"
	"github.com/devilmonastery/hivemind/internal/client"
)

// HandleInteraction routes interactions to the appropriate handler
func HandleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *config.Config, log *slog.Logger, grpcClient *client.Client) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		handleCommand(s, i, cfg, log, grpcClient)
	case discordgo.InteractionMessageComponent:
		handleComponent(s, i, cfg, log, grpcClient)
	case discordgo.InteractionModalSubmit:
		handleModal(s, i, cfg, log, grpcClient)
	case discordgo.InteractionApplicationCommandAutocomplete:
		handleAutocomplete(s, i, cfg, log, grpcClient)
	}
}

func handleCommand(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *config.Config, log *slog.Logger, grpcClient *client.Client) {
	commandName := i.ApplicationCommandData().Name

	log.Info("command received",
		slog.String("command", commandName),
		slog.String("user_id", i.Member.User.ID),
		slog.String("guild_id", i.GuildID),
	)

	switch commandName {
	case "ping":
		handlePing(s, i, log, grpcClient)
	case "wiki":
		handleWiki(s, i, log, grpcClient)
	case "note":
		handleNote(s, i, log, grpcClient)
	default:
		respondError(s, i, "Unknown command", log)
	}
}

func handleComponent(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *config.Config, log *slog.Logger, grpcClient *client.Client) {
	// TODO: Handle button clicks, select menus, etc.
	log.Info("component interaction received",
		slog.String("custom_id", i.MessageComponentData().CustomID),
		slog.String("user_id", i.Member.User.ID),
	)
}

func handleModal(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *config.Config, log *slog.Logger, grpcClient *client.Client) {
	customID := i.ModalSubmitData().CustomID

	log.Info("modal submission received",
		slog.String("custom_id", customID),
		slog.String("user_id", i.Member.User.ID),
	)

	switch customID {
	case "wiki_create_modal":
		handleWikiCreateModal(s, i, log, grpcClient)
	case "note_create_modal":
		handleNoteCreateModal(s, i, log, grpcClient)
	default:
		respondError(s, i, "Unknown modal", log)
	}
}

func handleAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *config.Config, log *slog.Logger, grpcClient *client.Client) {
	// TODO: Handle autocomplete requests
	log.Debug("autocomplete request received",
		slog.String("command", i.ApplicationCommandData().Name),
		slog.String("user_id", i.Member.User.ID),
	)
}

// respondError sends an error message to the user
func respondError(s *discordgo.Session, i *discordgo.InteractionCreate, message string, log *slog.Logger) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "❌ " + message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Error("failed to send error response", slog.String("error", err.Error()))
	}
}
