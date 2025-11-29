package handlers

import (
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	notespb "github.com/devilmonastery/hivemind/api/generated/go/notespb"
	quotespb "github.com/devilmonastery/hivemind/api/generated/go/quotespb"
	wikipb "github.com/devilmonastery/hivemind/api/generated/go/wikipb"
	"github.com/devilmonastery/hivemind/internal/client"
)

// handleContextMenuQuote handles "Save as Quote" context menu command
func handleContextMenuQuote(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger, grpcClient *client.Client) {
	// Get the target message
	targetID := i.ApplicationCommandData().TargetID
	message := i.ApplicationCommandData().Resolved.Messages[targetID]

	if message == nil {
		respondError(s, i, "Could not find the target message", log)
		return
	}

	// Show modal with pre-filled data
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: fmt.Sprintf("context_quote_modal:%s", targetID),
			Title:    "Save as Quote",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "quote_text",
							Label:       "Quote Text",
							Style:       discordgo.TextInputParagraph,
							Required:    true,
							Value:       message.Content,
							MaxLength:   4000,
							Placeholder: "Edit the quote if needed. Use #hashtags for tags",
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:  "quote_author",
							Label:     "Original Author (auto-detected)",
							Style:     discordgo.TextInputShort,
							Required:  false,
							Value:     message.Author.Username,
							MaxLength: 100,
						},
					},
				},
			},
		},
	})
	if err != nil {
		log.Error("Failed to show quote modal", "error", err)
	}
}

// handleContextMenuNote handles "Create Note" context menu command
func handleContextMenuNote(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger, grpcClient *client.Client) {
	// Get the target message
	targetID := i.ApplicationCommandData().TargetID
	message := i.ApplicationCommandData().Resolved.Messages[targetID]

	if message == nil {
		respondError(s, i, "Could not find the target message", log)
		return
	}

	// Create a reference to the original message
	messageRef := fmt.Sprintf("Referenced from: %s in <#%s>\n\n%s",
		message.Author.Username, message.ChannelID, message.Content)

	// Show modal with pre-filled data
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: fmt.Sprintf("context_note_modal:%s", targetID),
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
							Placeholder: "Note title",
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "note_body",
							Label:       "Note Content",
							Style:       discordgo.TextInputParagraph,
							Required:    true,
							Value:       messageRef,
							MaxLength:   4000,
							Placeholder: "Add your notes. Use #hashtags for tags",
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

// handleContextMenuWiki handles "Add to Wiki" context menu command
func handleContextMenuWiki(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger, grpcClient *client.Client) {
	// Get the target message
	targetID := i.ApplicationCommandData().TargetID
	message := i.ApplicationCommandData().Resolved.Messages[targetID]

	if message == nil {
		respondError(s, i, "Could not find the target message", log)
		return
	}

	// Show modal with pre-filled data
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: fmt.Sprintf("context_wiki_modal:%s", targetID),
			Title:    "Add to Wiki",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "wiki_title",
							Label:       "Wiki Page Title",
							Style:       discordgo.TextInputShort,
							Required:    true,
							MaxLength:   200,
							Placeholder: "Enter wiki page title",
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "wiki_body",
							Label:       "Wiki Content",
							Style:       discordgo.TextInputParagraph,
							Required:    true,
							Value:       message.Content,
							MaxLength:   4000,
							Placeholder: "Edit content. Use #hashtags for tags",
						},
					},
				},
			},
		},
	})
	if err != nil {
		log.Error("Failed to show wiki modal", "error", err)
	}
}

// handleContextQuoteModal handles the modal submission for context menu quote
func handleContextQuoteModal(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger, grpcClient *client.Client) {
	data := i.ModalSubmitData()

	var quoteText, author string
	for _, comp := range data.Components {
		if actionRow, ok := comp.(*discordgo.ActionsRow); ok {
			for _, innerComp := range actionRow.Components {
				if textInput, ok := innerComp.(*discordgo.TextInput); ok {
					switch textInput.CustomID {
					case "quote_text":
						quoteText = textInput.Value
					case "quote_author":
						author = textInput.Value
					}
				}
			}
		}
	}

	// Extract hashtags from the quote text (but keep original text)
	tags := extractHashtags(quoteText)

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

	quoteClient := quotespb.NewQuoteServiceClient(grpcClient.Conn())
	ctx := discordContextFor(i)

	// Extract message ID from custom ID if needed
	// Format: "context_quote_modal:MESSAGE_ID"
	// For now, we'll just save without the source message reference
	// Future enhancement: parse customID to get original message details

	req := &quotespb.CreateQuoteRequest{
		Body:    quoteText,
		Tags:    tags,
		GuildId: i.GuildID,
		// Could add source message fields here if we parse the customID
	}

	resp, err := quoteClient.CreateQuote(ctx, req)
	if err != nil {
		log.Error("Failed to create quote", "error", err)
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("❌ Failed to create quote: %v", err),
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	content := fmt.Sprintf("✅ Quote saved (ID: %s)", resp.Id)
	if len(tags) > 0 {
		content += fmt.Sprintf("\nTags: %s", formatTags(tags))
	}
	if author != "" {
		content += fmt.Sprintf("\nAuthor: %s", author)
	}

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: content,
		Flags:   discordgo.MessageFlagsEphemeral,
	})
	if err != nil {
		log.Error("Failed to send followup", "error", err)
	}
}

// handleContextNoteModal handles the modal submission for context menu note
func handleContextNoteModal(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger, grpcClient *client.Client) {
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

	// Extract hashtags from the body (but keep original text)
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

	content := fmt.Sprintf("✅ Note created: **%s** (ID: %s)", displayTitle, resp.Id)
	if len(tags) > 0 {
		content += fmt.Sprintf("\nTags: %s", formatTags(tags))
	}

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: content,
		Flags:   discordgo.MessageFlagsEphemeral,
	})
	if err != nil {
		log.Error("Failed to send followup", "error", err)
	}
}

// handleContextWikiModal handles the modal submission for context menu wiki
func handleContextWikiModal(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger, grpcClient *client.Client) {
	data := i.ModalSubmitData()

	var title, body string
	for _, comp := range data.Components {
		if actionRow, ok := comp.(*discordgo.ActionsRow); ok {
			for _, innerComp := range actionRow.Components {
				if textInput, ok := innerComp.(*discordgo.TextInput); ok {
					switch textInput.CustomID {
					case "wiki_title":
						title = textInput.Value
					case "wiki_body":
						body = textInput.Value
					}
				}
			}
		}
	}

	// Extract hashtags from body (but keep original text)
	tags := extractHashtags(body)

	// Defer response
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

	wikiClient := wikipb.NewWikiServiceClient(grpcClient.Conn())
	ctx := discordContextFor(i)

	page, err := wikiClient.CreateWikiPage(ctx, &wikipb.CreateWikiPageRequest{
		Title:     title,
		Body:      body,
		Tags:      tags,
		GuildId:   i.GuildID,
		ChannelId: i.ChannelID,
	})
	if err != nil {
		log.Error("Failed to create wiki page", "error", err)
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("❌ Failed to create wiki page: %v", err),
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	content := fmt.Sprintf("✅ Wiki page created: **%s**", page.Title)
	if len(tags) > 0 {
		content += fmt.Sprintf("\nTags: %s", formatTags(tags))
	}

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: content,
		Flags:   discordgo.MessageFlagsEphemeral,
	})
	if err != nil {
		log.Error("Failed to send followup", "error", err)
	}
}

// Helper functions

// formatTags converts a slice of tags to a comma-separated string
func formatTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	result := ""
	for i, tag := range tags {
		if i > 0 {
			result += ", "
		}
		result += tag
	}
	return result
}
