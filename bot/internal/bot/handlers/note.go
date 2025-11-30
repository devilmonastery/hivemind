package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
	notespb "github.com/devilmonastery/hivemind/api/generated/go/notespb"
	"github.com/devilmonastery/hivemind/bot/internal/config"
	"github.com/devilmonastery/hivemind/internal/client"
)

// fetchNoteMessageReferences fetches message references for a note
func fetchNoteMessageReferences(ctx context.Context, noteClient notespb.NoteServiceClient, noteID string, log *slog.Logger) []*notespb.NoteMessageReference {
	log.Info("fetching note message references",
		slog.String("note_id", noteID))

	resp, err := noteClient.ListNoteMessageReferences(ctx, &notespb.ListNoteMessageReferencesRequest{
		NoteId: noteID,
	})
	if err != nil {
		log.Warn("failed to fetch note message references",
			slog.String("note_id", noteID),
			slog.String("error", err.Error()))
		return nil
	}

	log.Info("fetched note message references",
		slog.String("note_id", noteID),
		slog.Int("count", len(resp.References)))

	return resp.References
}

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
func handleNoteCreateModal(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *config.Config, log *slog.Logger, grpcClient *client.Client) {
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

	// Fetch message references
	refs := fetchNoteMessageReferences(ctx, noteClient, resp.Id, log)

	// Show standard note embed
	embed, components := createNoteEmbed(resp, refs, cfg)
	embed.Title = "✅ Note Created\n\n" + embed.Title

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: components,
		Flags:      discordgo.MessageFlagsEphemeral,
	})
	if err != nil {
		log.Error("Failed to send followup", "error", err)
	}
}

// handleNoteList lists user's notes
// createNoteEmbed creates an embed for displaying a note with action buttons
func createNoteEmbed(note *notespb.Note, references []*notespb.NoteMessageReference, cfg *config.Config) (*discordgo.MessageEmbed, []discordgo.MessageComponent) {
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

	// Add message references field if any exist
	if len(references) > 0 {
		// Build reference list with datetime and content preview
		refsList := ""
		displayCount := min(5, len(references)) // Show up to 5
		for idx := 0; idx < displayCount; idx++ {
			ref := references[idx]
			messageLink := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", ref.GuildId, ref.ChannelId, ref.MessageId)

			// Format timestamp
			timestamp := ref.MessageTimestamp.AsTime().Format("2006-01-02 15:04")

			// Truncate content preview
			contentPreview := ref.Content
			if len(contentPreview) > 60 {
				contentPreview = contentPreview[:57] + "..."
			}

			refsList += fmt.Sprintf("• [%s](%s) - %s\n  _%s_\n", ref.AuthorUsername, messageLink, timestamp, contentPreview)
		}
		if len(references) > displayCount {
			refsList += fmt.Sprintf("_...and %d more_", len(references)-displayCount)
		}

		slog.Default().Info("adding message references field to note embed",
			slog.Int("ref_count", len(references)),
			slog.Int("displayed", displayCount))

		embed.Fields = []*discordgo.MessageEmbedField{
			{
				Name:   fmt.Sprintf("📌 Referenced Messages (%d)", len(references)),
				Value:  refsList,
				Inline: false,
			},
		}
	} else {
		slog.Default().Info("no message references to display for note")
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

		// Fetch message references
		refs := fetchNoteMessageReferences(ctx, noteClient, note.Id, log)
		log.Info("note view - displaying with references",
			slog.String("note_id", note.Id),
			slog.String("note_title", note.Title),
			slog.Int("ref_count", len(refs)))

		embed, components := createNoteEmbed(note, refs, cfg)

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
	limit := int32(25) // Increased to match Discord's dropdown limit

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
			if limit > 25 {
				limit = 25 // Cap at Discord's dropdown limit
			}
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
			Content: fmt.Sprintf("No notes found matching \"%s\"", query),
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	// Show search results with a dropdown to select and post a note
	// Limit to 25 results (Discord's max for select menus)
	displayLimit := len(resp.Notes)
	if displayLimit > 25 {
		displayLimit = 25
	}

	var content strings.Builder
	content.WriteString(fmt.Sprintf("🔍 Found %d note(s) matching \"%s\"\n", resp.Total, query))
	content.WriteString("Select a note from the dropdown to view:")

	// Build select menu options
	options := []discordgo.SelectMenuOption{}
	for idx := 0; idx < displayLimit; idx++ {
		note := resp.Notes[idx]

		// Create a label from title or body snippet (truncate if too long - Discord max is 100 chars)
		label := note.Title
		if label == "" {
			// Use body snippet if no title
			label = note.Body
			if len(label) > 100 {
				label = label[:97] + "..."
			}
		} else if len(label) > 100 {
			label = label[:97] + "..."
		}

		// Create a description with body snippet or tags
		description := ""
		if note.Title != "" && note.Body != "" {
			// If we have a title, show body snippet in description
			description = note.Body
			if len(description) > 100 {
				description = description[:97] + "..."
			}
		} else if len(note.Tags) > 0 {
			// Otherwise show tags
			description = "Tags: " + strings.Join(note.Tags, ", ")
			if len(description) > 100 {
				description = description[:97] + "..."
			}
		}

		options = append(options, discordgo.SelectMenuOption{
			Label:       label,
			Description: description,
			Value:       note.Id,
			Emoji: &discordgo.ComponentEmoji{
				Name: "📝",
			},
		})
	}

	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "view_note_select",
					Placeholder: "Choose a note to view...",
					Options:     options,
				},
			},
		},
	}

	if resp.Total > int32(displayLimit) {
		content.WriteString(fmt.Sprintf("\n\n_Showing first %d of %d results. Refine your search for more specific results._", displayLimit, resp.Total))
	}

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content:    content.String(),
		Components: components,
		Flags:      discordgo.MessageFlagsEphemeral,
	})
	if err != nil {
		log.Error("Failed to send followup", "error", err)
	}
}

// handleViewNoteSelect handles dropdown selection to view a note ephemerally
func handleViewNoteSelect(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *config.Config, log *slog.Logger, grpcClient *client.Client) {
	// Get the selected note ID from the dropdown
	data := i.MessageComponentData()
	if len(data.Values) == 0 {
		log.Warn("No value selected in dropdown")
		return
	}

	noteID := data.Values[0]

	// Fetch the note by ID
	noteClient := notespb.NewNoteServiceClient(grpcClient.Conn())
	ctx := discordContextFor(i)

	note, err := noteClient.GetNote(ctx, &notespb.GetNoteRequest{
		Id: noteID,
	})
	if err != nil {
		log.Error("Failed to fetch note", "note_id", noteID, "error", err)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ Failed to fetch note",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Fetch message references
	refs := fetchNoteMessageReferences(ctx, noteClient, note.Id, log)
	log.Info("note select - displaying with references",
		slog.String("note_id", note.Id),
		slog.String("note_title", note.Title),
		slog.Int("ref_count", len(refs)))

	// Use standard embed function
	embed, _ := createNoteEmbed(note, refs, cfg)

	// Display the note ephemerally (only visible to the user)
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: []discordgo.MessageComponent{}, // Remove dropdown
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Error("Failed to display note", "error", err)
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
func handleNoteAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger, grpcClient *client.Client, cache *TitlesCache) {
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
	userID := i.Member.User.ID

	// Check local cache first
	cachedTitles := cache.GetNoteTitles(userID, i.GuildID)

	// If cache miss, fetch from server and populate cache
	if cachedTitles == nil {
		noteClient := notespb.NewNoteServiceClient(grpcClient.Conn())
		ctx := discordContextFor(i)

		autocompleteResp, err := noteClient.AutocompleteNoteTitles(ctx, &notespb.AutocompleteNoteTitlesRequest{
			GuildId: i.GuildID,
		})
		if err != nil {
			log.Error("Failed to fetch note titles for cache", "error", err)
			return
		}

		// Convert to cache format
		cachedTitles = make([]TitleSuggestion, len(autocompleteResp.Suggestions))
		for idx, suggestion := range autocompleteResp.Suggestions {
			cachedTitles[idx] = TitleSuggestion{
				ID:    suggestion.Id,
				Title: suggestion.Title,
			}
		}

		// Store in cache (user-specific)
		cache.SetNoteTitles(userID, i.GuildID, cachedTitles)
	}

	// Filter titles locally
	filtered := FilterTitles(cachedTitles, query, 25)

	// Build autocomplete choices
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(filtered))
	for _, title := range filtered {
		displayTitle := title.Title
		if displayTitle == "" {
			displayTitle = "(untitled)"
		}

		// Limit title length for display
		if len(displayTitle) > 100 {
			displayTitle = displayTitle[:97] + "..."
		}

		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  displayTitle,
			Value: title.Title, // Return the full title
		})
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	})
	if err != nil {
		log.Error("Failed to send autocomplete response", "error", err)
	}
}
