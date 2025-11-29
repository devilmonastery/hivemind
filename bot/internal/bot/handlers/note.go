package handlers

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
	notespb "github.com/devilmonastery/hivemind/api/generated/go/notespb"
	"github.com/devilmonastery/hivemind/bot/internal/config"
	"github.com/devilmonastery/hivemind/internal/client"
)

// handleNote routes /note subcommands to the appropriate handler
func handleNote(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *config.Config, log *slog.Logger, grpcClient *client.Client) {
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		respondError(s, i, "No subcommand provided", log)
		return
	}

	subcommand := options[0]

	switch subcommand.Name {
	case "create":
		handleNoteCreate(s, i, subcommand, log, grpcClient)
	case "view":
		handleNoteView(s, i, subcommand, cfg, log, grpcClient)
	case "search":
		handleNoteSearch(s, i, subcommand, log, grpcClient)
	default:
		respondError(s, i, "Unknown note subcommand", log)
	}
}

// handleNoteCreate shows a modal to create a note
func handleNoteCreate(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption, log *slog.Logger, grpcClient *client.Client) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: "note_create_modal",
			Title:    "Create Note",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "note_title",
							Label:       "Title",
							Style:       discordgo.TextInputShort,
							Required:    true,
							MaxLength:   200,
							Placeholder: "My note title",
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "note_body",
							Label:       "Body",
							Style:       discordgo.TextInputParagraph,
							Required:    true,
							MaxLength:   4000,
							Placeholder: "Note content... Use #hashtags to add tags",
						},
					},
				},
			},
		},
	})
	if err != nil {
		log.Error("Failed to show note modal", "error", err)
	}
}

// handleNoteCreateModal handles the modal submission for note creation
func handleNoteCreateModal(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger, grpcClient *client.Client) {
	data := i.ModalSubmitData()

	var title, body string
	for _, comp := range data.Components {
		if actionRow, ok := comp.(*discordgo.ActionsRow); ok {
			for _, innerComp := range actionRow.Components {
				if textInput, ok := innerComp.(*discordgo.TextInput); ok {
					switch textInput.CustomID {
					case "note_title":
						title = textInput.Value
					case "note_body":
						body = textInput.Value
					}
				}
			}
		}
	}

	// Validate title is not empty
	title = strings.TrimSpace(title)
	if title == "" {
		respondError(s, i, "Note title cannot be empty", log)
		return
	}

	// Validate body is not empty
	body = strings.TrimSpace(body)
	if body == "" {
		respondError(s, i, "Note body cannot be empty", log)
		return
	}

	// Extract hashtags from body
	tags := extractHashtags(body)

	// Defer response to avoid timeout
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Error("Failed to defer response", "error", err)
		return
	}

	noteClient := notespb.NewNoteServiceClient(grpcClient.Conn())
	ctx := discordContextFor(i)

	req := &notespb.CreateNoteRequest{
		Title: title,
		Body:  body,
		Tags:  tags,
	}

	// Add guild context if in a guild
	if i.GuildID != "" {
		req.GuildId = i.GuildID
		req.ChannelId = i.ChannelID
	}

	resp, err := noteClient.CreateNote(ctx, req)
	if err != nil {
		log.Error("Failed to create note", "error", err)
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("❌ Failed to create note: %v", err),
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	displayTitle := title
	if displayTitle == "" {
		displayTitle = "(untitled)"
	}

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: fmt.Sprintf("✅ Note created: **%s** (ID: %s)", displayTitle, resp.Id),
		Flags:   discordgo.MessageFlagsEphemeral,
	})
	if err != nil {
		log.Error("Failed to send followup", "error", err)
	}
}

// handleNoteList lists user's notes
// createNoteEmbed creates an embed for displaying a note with action buttons
func createNoteEmbed(note *notespb.Note, cfg *config.Config) (*discordgo.MessageEmbed, []discordgo.MessageComponent) {
	title := note.Title
	if title == "" {
		title = "(untitled)"
	}

	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: note.Body,
		Color:       0x5865F2,
		Timestamp:   note.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z07:00"),
	}

	if len(note.Tags) > 0 {
		embed.Footer = &discordgo.MessageEmbedFooter{
			Text: "Tags: " + strings.Join(note.Tags, ", "),
		}
	}

	// Create action buttons
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Edit",
					Style:    discordgo.PrimaryButton,
					CustomID: fmt.Sprintf("note_edit_btn:%s", note.Id),
					Emoji: &discordgo.ComponentEmoji{
						Name: "✏️",
					},
				},
				discordgo.Button{
					Label:    "Delete",
					Style:    discordgo.DangerButton,
					CustomID: fmt.Sprintf("note_delete_btn:%s", note.Id),
					Emoji: &discordgo.ComponentEmoji{
						Name: "🗑️",
					},
				},
				discordgo.Button{
					Label: "View on Web",
					Style: discordgo.LinkButton,
					URL:   fmt.Sprintf("%s/note?id=%s", getWebBaseURL(cfg), note.Id),
					Emoji: &discordgo.ComponentEmoji{
						Name: "🌐",
					},
				},
				discordgo.Button{
					Label:    "Close",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("note_close_btn:%s", note.Id),
					Emoji: &discordgo.ComponentEmoji{
						Name: "✖️",
					},
				},
			},
		},
	}

	return embed, components
}

// handleNoteView shows a specific note
func handleNoteView(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption, cfg *config.Config, log *slog.Logger, grpcClient *client.Client) {
	if len(subcommand.Options) == 0 {
		respondError(s, i, "Please provide a note title", log)
		return
	}

	titleQuery := subcommand.Options[0].StringValue()

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Error("Failed to defer response", "error", err)
		return
	}

	noteClient := notespb.NewNoteServiceClient(grpcClient.Conn())
	ctx := discordContextFor(i)

	// Search for notes by title
	searchResp, err := noteClient.SearchNotes(ctx, &notespb.SearchNotesRequest{
		Query:   titleQuery,
		GuildId: i.GuildID,
		Limit:   5,
	})
	if err != nil {
		log.Error("Failed to search notes", "error", err)
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("❌ Failed to search notes: %v", err),
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	// No results
	if len(searchResp.Notes) == 0 {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("📝 No notes found matching \"%s\"", titleQuery),
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	// If exactly one match, show it
	if len(searchResp.Notes) == 1 {
		note := searchResp.Notes[0]
		embed, components := createNoteEmbed(note, cfg)

		_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
			Flags:      discordgo.MessageFlagsEphemeral,
		})
		if err != nil {
			log.Error("Failed to send followup", "error", err)
		}
		return
	}

	// Multiple matches - show list to disambiguate
	var content strings.Builder
	content.WriteString(fmt.Sprintf("📝 Found %d notes matching \"%s\". Please be more specific:\n\n", len(searchResp.Notes), titleQuery))
	for idx, note := range searchResp.Notes {
		title := note.Title
		if title == "" {
			title = "(untitled)"
		}
		preview := note.Body
		if len(preview) > 60 {
			preview = preview[:57] + "..."
		}
		content.WriteString(fmt.Sprintf("%d. **%s**\n   %s\n", idx+1, title, preview))
		if len(note.Tags) > 0 {
			content.WriteString(fmt.Sprintf("   Tags: %s\n", strings.Join(note.Tags, ", ")))
		}
		content.WriteString("\n")
	}
	content.WriteString("\n_Try using `/note view` with the exact title, or use `/note search` for more options_")

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: content.String(),
		Flags:   discordgo.MessageFlagsEphemeral,
	})
	if err != nil {
		log.Error("Failed to send followup", "error", err)
	}
}

// handleNoteSearch searches notes by full-text query
func handleNoteSearch(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption, log *slog.Logger, grpcClient *client.Client) {
	if len(subcommand.Options) == 0 {
		respondError(s, i, "Please provide a search query", log)
		return
	}

	var query, guildID string
	var tags []string
	limit := int32(10)

	for _, opt := range subcommand.Options {
		switch opt.Name {
		case "query":
			query = opt.StringValue()
		case "guild":
			if opt.BoolValue() {
				guildID = i.GuildID
			}
		case "tags":
			tagStr := opt.StringValue()
			parts := strings.Split(tagStr, ",")
			for _, tag := range parts {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					tags = append(tags, tag)
				}
			}
		case "limit":
			limit = int32(opt.IntValue())
		}
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Error("Failed to defer response", "error", err)
		return
	}

	noteClient := notespb.NewNoteServiceClient(grpcClient.Conn())
	ctx := discordContextFor(i)

	req := &notespb.SearchNotesRequest{
		Query:   query,
		GuildId: guildID,
		Tags:    tags,
		Limit:   limit,
	}

	resp, err := noteClient.SearchNotes(ctx, req)
	if err != nil {
		log.Error("Failed to search notes", "error", err)
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("❌ Failed to search notes: %v", err),
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	if len(resp.Notes) == 0 {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "No notes found matching your query",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	var content strings.Builder
	content.WriteString(fmt.Sprintf("🔍 Found %d note(s) matching \"%s\":\n\n", resp.Total, query))
	for idx, note := range resp.Notes {
		title := note.Title
		if title == "" {
			title = "(untitled)"
		}
		content.WriteString(fmt.Sprintf("%d. **%s** (ID: %s)\n", idx+1, title, note.Id))

		// Show snippet of body
		body := note.Body
		if len(body) > 100 {
			body = body[:100] + "..."
		}
		content.WriteString(fmt.Sprintf("   %s\n", body))

		if len(note.Tags) > 0 {
			content.WriteString(fmt.Sprintf("   Tags: %s\n", strings.Join(note.Tags, ", ")))
		}
		content.WriteString("\n")
	}

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: content.String(),
		Flags:   discordgo.MessageFlagsEphemeral,
	})
	if err != nil {
		log.Error("Failed to send followup", "error", err)
	}
}

// handleNoteEditButton handles the edit button click
func handleNoteEditButton(s *discordgo.Session, i *discordgo.InteractionCreate, noteID string, log *slog.Logger, grpcClient *client.Client) {
	// TODO: Implement note editing - for now just show a message
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("✏️ Note editing is coming soon! Note ID: %s\n\nFor now, please use the web interface to edit this note.", noteID),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Error("Failed to respond to edit button", "error", err)
	}
}

// handleNoteDeleteButton handles the delete button click
func handleNoteDeleteButton(s *discordgo.Session, i *discordgo.InteractionCreate, noteID string, log *slog.Logger, grpcClient *client.Client) {
	// Show confirmation
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("⚠️ Are you sure you want to delete this note?\n\nNote ID: %s", noteID),
			Flags:   discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Yes, Delete",
							Style:    discordgo.DangerButton,
							CustomID: fmt.Sprintf("note_delete_confirm:%s", noteID),
						},
						discordgo.Button{
							Label:    "Cancel",
							Style:    discordgo.SecondaryButton,
							CustomID: "note_delete_cancel",
						},
					},
				},
			},
		},
	})
	if err != nil {
		log.Error("Failed to show delete confirmation", "error", err)
	}
}

// handleNoteCloseButton handles the close button click
func handleNoteCloseButton(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger) {
	// Delete the message
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    "✖️ Note closed",
			Embeds:     []*discordgo.MessageEmbed{},
			Components: []discordgo.MessageComponent{},
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Error("Failed to close note", "error", err)
	}
}

// handleNoteDeleteConfirm handles the confirmed delete action
func handleNoteDeleteConfirm(s *discordgo.Session, i *discordgo.InteractionCreate, noteID string, log *slog.Logger, grpcClient *client.Client) {
	// Defer the response
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
	if err != nil {
		log.Error("Failed to defer delete response", "error", err)
		return
	}

	// Delete the note via gRPC
	ctx := discordContextFor(i)
	noteClient := notespb.NewNoteServiceClient(grpcClient.Conn())

	_, err = noteClient.DeleteNote(ctx, &notespb.DeleteNoteRequest{Id: noteID})
	if err != nil {
		log.Error("Failed to delete note", "error", err)
		_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: ptrString(fmt.Sprintf("❌ Failed to delete note: %v", err)),
		})
		return
	}

	// Update message to show success
	_, err = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content:    ptrString("✅ Note deleted successfully"),
		Components: &[]discordgo.MessageComponent{},
	})
	if err != nil {
		log.Error("Failed to update message", "error", err)
	}

	log.Info("Note deleted", "note_id", noteID, "user_id", i.Member.User.ID)
}

// handleNoteDeleteCancel handles cancelling the delete action
func handleNoteDeleteCancel(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    "Delete cancelled",
			Components: []discordgo.MessageComponent{},
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Error("Failed to cancel delete", "error", err)
	}
}

// ptrString returns a pointer to a string
func ptrString(s string) *string {
	return &s
}

// handleNoteAutocomplete handles autocomplete for note commands
func handleNoteAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger, grpcClient *client.Client) {
	data := i.ApplicationCommandData()

	// Get the focused option
	var focusedOption *discordgo.ApplicationCommandInteractionDataOption
	if len(data.Options) > 0 && len(data.Options[0].Options) > 0 {
		for _, opt := range data.Options[0].Options {
			if opt.Focused {
				focusedOption = opt
				break
			}
		}
	}

	if focusedOption == nil {
		return
	}

	// Only handle title autocomplete for "view" subcommand
	if data.Options[0].Name != "view" || focusedOption.Name != "title" {
		return
	}

	query := focusedOption.StringValue()

	// Use lightweight autocomplete RPC
	noteClient := notespb.NewNoteServiceClient(grpcClient.Conn())
	ctx := discordContextFor(i)

	autocompleteResp, err := noteClient.AutocompleteNoteTitles(ctx, &notespb.AutocompleteNoteTitlesRequest{
		Query:   query,
		GuildId: i.GuildID,
		Limit:   25, // Discord allows up to 25 autocomplete choices
	})
	if err != nil {
		log.Error("Failed to autocomplete note titles", "error", err)
		return
	}

	// Build autocomplete choices
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(autocompleteResp.Suggestions))
	for _, suggestion := range autocompleteResp.Suggestions {
		title := suggestion.Title
		if title == "" {
			title = "(untitled)"
		}

		// Limit title length for display
		displayTitle := title
		if len(displayTitle) > 100 {
			displayTitle = displayTitle[:97] + "..."
		}

		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  displayTitle,
			Value: title, // Return the full title
		})
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	})
	if err != nil {
		log.Error("Failed to send autocomplete response", "error", err)
	}
}
