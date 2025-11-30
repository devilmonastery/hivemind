package handlers

import (
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"

	"github.com/devilmonastery/hivemind/bot/internal/config"
	"github.com/devilmonastery/hivemind/internal/client"
)

// HandleInteraction routes interactions to the appropriate handler
func HandleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *config.Config, log *slog.Logger, grpcClient *client.Client, cache *TitlesCache) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		handleCommand(s, i, cfg, log, grpcClient)
	case discordgo.InteractionMessageComponent:
		handleComponent(s, i, cfg, log, grpcClient)
	case discordgo.InteractionModalSubmit:
		handleModal(s, i, cfg, log, grpcClient)
	case discordgo.InteractionApplicationCommandAutocomplete:
		handleAutocomplete(s, i, cfg, log, grpcClient, cache)
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
		handleWiki(s, i, cfg, log, grpcClient)
	case "note":
		handleNote(s, i, cfg, log, grpcClient)
	case "quote":
		handleQuote(s, i, log, grpcClient)
	// Context menu commands
	case "Save as Quote":
		handleContextMenuQuote(s, i, log, grpcClient)
	case "Create Note":
		handleContextMenuNote(s, i, log, grpcClient)
	case "Add to Wiki":
		handleContextMenuWiki(s, i, log, grpcClient)
	// User context menu commands
	case "Edit Note for User":
		handleContextMenuAddNoteForUser(s, i, cfg, log, grpcClient)
	case "View Note for User":
		handleContextMenuViewNotesForUser(s, i, cfg, log, grpcClient)
	default:
		respondError(s, i, "Unknown command", log)
	}
}

func handleComponent(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *config.Config, log *slog.Logger, grpcClient *client.Client) {
	customID := i.MessageComponentData().CustomID

	log.Info("component interaction received",
		slog.String("custom_id", customID),
		slog.String("user_id", i.Member.User.ID),
	)

	// Split customID by colon to get the handler type
	parts := strings.SplitN(customID, ":", 2)
	if len(parts) < 1 {
		log.Warn("invalid custom_id format", slog.String("custom_id", customID))
		return
	}

	handlerType := parts[0]
	remainder := ""
	if len(parts) > 1 {
		remainder = parts[1]
	}

	switch handlerType {
	case "wiki_select":
		handleWikiSelectMenu(s, i, remainder, cfg, log, grpcClient)
	case "wiki_action_btn":
		handleWikiActionButton(s, i, customID, cfg, log, grpcClient)
	case "wiki_edit_btn":
		handleWikiEditButton(s, i, remainder, cfg, log, grpcClient)
	case "wiki_add_to_chat":
		handleWikiAddToChat(s, i, remainder, cfg, log, grpcClient)
	case "wiki_close":
		handleWikiClose(s, i, log)
	case "wiki_unified_select":
		log.Info("routing to handleWikiUnifiedSelect", slog.String("messageID", remainder))
		handleWikiUnifiedSelect(s, i, remainder, log, grpcClient)
	case "note_edit_btn":
		handleNoteEditButton(s, i, remainder, log, grpcClient)
	case "note_delete_btn":
		handleNoteDeleteButton(s, i, remainder, log, grpcClient)
	case "note_delete_confirm":
		handleNoteDeleteConfirm(s, i, remainder, log, grpcClient)
	case "note_delete_cancel":
		handleNoteDeleteCancel(s, i, log)
	case "note_close_btn":
		handleNoteCloseButton(s, i, log)
	case "post_quote_select":
		handlePostQuoteSelect(s, i, log, grpcClient)
	case "view_note_select":
		handleViewNoteSelect(s, i, cfg, log, grpcClient)
	default:
		log.Warn("no handler found for custom_id", slog.String("custom_id", customID))
	}
}

func handleModal(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *config.Config, log *slog.Logger, grpcClient *client.Client) {
	customID := i.ModalSubmitData().CustomID

	log.Info("modal submission received",
		slog.String("custom_id", customID),
		slog.String("user_id", i.Member.User.ID),
	)

	// Split customID by colon to get the handler type
	parts := strings.SplitN(customID, ":", 2)
	if len(parts) < 1 {
		log.Warn("invalid modal custom_id format", slog.String("custom_id", customID))
		respondError(s, i, "Unknown modal", log)
		return
	}

	handlerType := parts[0]

	switch handlerType {
	case "context_wiki_unified_modal":
		log.Info("routing to handleContextWikiUnifiedModal", slog.String("custom_id", customID))
		handleContextWikiUnifiedModal(s, i, log, grpcClient)
	case "wiki_edit_modal":
		handleWikiEditModal(s, i, log, grpcClient)
	case "note_create_modal":
		handleNoteCreateModal(s, i, cfg, log, grpcClient)
	case "context_quote_modal":
		handleContextQuoteModal(s, i, log, grpcClient)
	case "context_note_modal":
		handleContextNoteModal(s, i, cfg, log, grpcClient)
	case "context_wiki_modal":
		handleContextWikiModal(s, i, log, grpcClient)
	case "user_note_modal":
		handleUserNoteModal(s, i, cfg, log, grpcClient)
	default:
		log.Warn("no handler found for modal", slog.String("custom_id", customID))
		respondError(s, i, "Unknown modal", log)
	}
}

func handleAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *config.Config, log *slog.Logger, grpcClient *client.Client, cache *TitlesCache) {
	data := i.ApplicationCommandData()

	log.Debug("autocomplete request received",
		slog.String("command", data.Name),
		slog.String("user_id", i.Member.User.ID),
	)

	switch data.Name {
	case "note":
		handleNoteAutocomplete(s, i, log, grpcClient, cache)
	case "wiki":
		handleWikiAutocomplete(s, i, log, grpcClient, cache)
	default:
		log.Warn("no autocomplete handler for command", slog.String("command", data.Name))
	}
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
