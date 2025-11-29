package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
	notespb "github.com/devilmonastery/hivemind/api/generated/go/notespb"
	"github.com/devilmonastery/hivemind/internal/client"
)

// handleNote routes /note subcommands to the appropriate handler
func handleNote(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger, grpcClient *client.Client) {
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		respondError(s, i, "No subcommand provided", log)
		return
	}

	subcommand := options[0]

	switch subcommand.Name {
	case "create":
		handleNoteCreate(s, i, subcommand, log, grpcClient)
	case "list":
		handleNoteList(s, i, subcommand, log, grpcClient)
	case "view":
		handleNoteView(s, i, subcommand, log, grpcClient)
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
							Label:       "Title (optional)",
							Style:       discordgo.TextInputShort,
							Required:    false,
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
							Placeholder: "Note content...",
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "note_tags",
							Label:       "Tags (comma-separated, optional)",
							Style:       discordgo.TextInputShort,
							Required:    false,
							MaxLength:   200,
							Placeholder: "tag1, tag2, tag3",
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

	var title, body, tagsStr string
	for _, comp := range data.Components {
		if actionRow, ok := comp.(*discordgo.ActionsRow); ok {
			for _, innerComp := range actionRow.Components {
				if textInput, ok := innerComp.(*discordgo.TextInput); ok {
					switch textInput.CustomID {
					case "note_title":
						title = textInput.Value
					case "note_body":
						body = textInput.Value
					case "note_tags":
						tagsStr = textInput.Value
					}
				}
			}
		}
	}

	var tags []string
	if tagsStr != "" {
		parts := strings.Split(tagsStr, ",")
		for _, tag := range parts {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}

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
	ctx := context.Background()

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
func handleNoteList(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption, log *slog.Logger, grpcClient *client.Client) {
	var guildID string
	var tags []string
	limit := int32(10)

	for _, opt := range subcommand.Options {
		switch opt.Name {
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
	ctx := context.Background()

	req := &notespb.ListNotesRequest{
		GuildId: guildID,
		Tags:    tags,
		Limit:   limit,
	}

	resp, err := noteClient.ListNotes(ctx, req)
	if err != nil {
		log.Error("Failed to list notes", "error", err)
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("❌ Failed to list notes: %v", err),
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	if len(resp.Notes) == 0 {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "No notes found",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	var content strings.Builder
	content.WriteString(fmt.Sprintf("📝 Found %d note(s):\n\n", resp.Total))
	for idx, note := range resp.Notes {
		title := note.Title
		if title == "" {
			title = "(untitled)"
		}
		content.WriteString(fmt.Sprintf("%d. **%s** (ID: %s)\n", idx+1, title, note.Id))
		if len(note.Tags) > 0 {
			content.WriteString(fmt.Sprintf("   Tags: %s\n", strings.Join(note.Tags, ", ")))
		}
	}

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: content.String(),
		Flags:   discordgo.MessageFlagsEphemeral,
	})
	if err != nil {
		log.Error("Failed to send followup", "error", err)
	}
}

// handleNoteView shows a specific note
func handleNoteView(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption, log *slog.Logger, grpcClient *client.Client) {
	if len(subcommand.Options) == 0 {
		respondError(s, i, "Please provide a note ID", log)
		return
	}

	noteID := subcommand.Options[0].StringValue()

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
	ctx := context.Background()

	note, err := noteClient.GetNote(ctx, &notespb.GetNoteRequest{Id: noteID})
	if err != nil {
		log.Error("Failed to get note", "error", err)
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("❌ Failed to get note: %v", err),
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

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

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embed},
		Flags:  discordgo.MessageFlagsEphemeral,
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
	ctx := context.Background()

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
