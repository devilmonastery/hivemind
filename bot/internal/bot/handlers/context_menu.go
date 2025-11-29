package handlers

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
	"google.golang.org/protobuf/types/known/timestamppb"

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

	// Fetch wiki pages for this guild
	wikiClient := wikipb.NewWikiServiceClient(grpcClient.Conn())
	resp, err := wikiClient.ListWikiPages(discordContextFor(i), &wikipb.ListWikiPagesRequest{
		GuildId:   i.GuildID,
		Limit:     25,
		OrderBy:   "updated_at", // Most recently updated first
		Ascending: false,
	})
	if err != nil {
		respondError(s, i, fmt.Sprintf("Failed to fetch wiki pages: %v", err), log)
		return
	}

	// Build select menu options with "Create New" as first option
	options := []discordgo.SelectMenuOption{
		{
			Label:       "➕ Create New Page",
			Value:       "__create_new__",
			Description: "Create a new wiki page from this message",
		},
	}

	// Add existing pages
	for _, page := range resp.Pages {
		description := stripMarkdownAndNewlines(page.Body)
		if len(description) > 100 {
			description = description[:97] + "..."
		}
		options = append(options, discordgo.SelectMenuOption{
			Label:       page.Title,
			Value:       page.Id,
			Description: description,
		})
	}

	// Show select menu
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Where should this message go?",
			Flags:   discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							CustomID:    fmt.Sprintf("wiki_unified_select:%s", targetID),
							Placeholder: "Create new or select existing page...",
							Options:     options,
						},
					},
				},
			},
		},
	})
	if err != nil {
		log.Error("Failed to show wiki select menu", "error", err)
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

// handleContextMenuWikiRef handles "Add to Wiki Page" context menu command
func handleContextMenuWikiRef(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger, grpcClient *client.Client) {
	// This function is deprecated - unified flow now uses handleContextMenuWiki
	log.Warn("handleContextMenuWikiRef called - this should use unified flow")
}

// handleContextWikiUnifiedModal handles submission of the unified wiki modal
func handleContextWikiUnifiedModal(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger, grpcClient *client.Client) {
	// Parse custom ID: context_wiki_unified_modal:MessageID:PageID
	customID := i.ModalSubmitData().CustomID
	parts := strings.Split(customID, ":")
	if len(parts) < 3 {
		respondError(s, i, "Invalid modal data", log)
		return
	}

	messageID := parts[1]
	pageID := parts[2] // "__create_new__" or actual page ID

	// Get modal field values
	data := i.ModalSubmitData()
	var title, body string

	if pageID == "__create_new__" {
		// Creating new page - get title and body from modal
		for _, component := range data.Components {
			row := component.(*discordgo.ActionsRow)
			for _, comp := range row.Components {
				input := comp.(*discordgo.TextInput)
				switch input.CustomID {
				case "wiki_title":
					title = input.Value
				case "wiki_body":
					body = input.Value
				}
			}
		}
	} else {
		// Updating existing page - only body field
		for _, component := range data.Components {
			row := component.(*discordgo.ActionsRow)
			for _, comp := range row.Components {
				input := comp.(*discordgo.TextInput)
				if input.CustomID == "wiki_update_content" {
					body = input.Value
				}
			}
		}
	}

	// Fetch the original message for reference
	message, err := s.ChannelMessage(i.ChannelID, messageID)
	if err != nil {
		respondError(s, i, fmt.Sprintf("Failed to fetch message: %v", err), log)
		return
	}

	// Note: message.GuildID is often empty when fetched via REST API
	// Use the guild ID from the interaction instead
	guildID := i.GuildID

	// Note: message.Content might be empty if bot lacks MESSAGE_CONTENT intent
	// Log what we got
	log.Info("Fetched message for wiki reference",
		"message_id", message.ID,
		"content_len", len(message.Content),
		"guild_id_from_interaction", guildID,
		"author", message.Author.Username)

	// Extract tags from body
	tags := extractHashtags(body)

	// Extract attachment metadata
	attachments := make([]*wikipb.AttachmentMetadata, 0, len(message.Attachments))
	for _, attachment := range message.Attachments {
		attachments = append(attachments, &wikipb.AttachmentMetadata{
			Url:         attachment.URL,
			ContentType: attachment.ContentType,
			Filename:    attachment.Filename,
			Width:       int32(attachment.Width),
			Height:      int32(attachment.Height),
			Size:        int64(attachment.Size),
		})
	}

	// Get author display name
	authorDisplayName := message.Author.Username
	if message.Member != nil && message.Member.Nick != "" {
		authorDisplayName = message.Member.Nick
	}

	// Defer to acknowledge the interaction
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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

	if pageID == "__create_new__" {
		// Create new page or get existing if title exists
		var existingPage *wikipb.WikiPage
		existingPage, _ = wikiClient.GetWikiPage(discordContextFor(i), &wikipb.GetWikiPageRequest{
			Id: title, // This will fail, but we need to search by title
		})
		_ = existingPage // Suppress unused warning

		// Use UpsertWikiPage which handles create-or-update
		upsertResp, upsertErr := wikiClient.UpsertWikiPage(discordContextFor(i), &wikipb.UpsertWikiPageRequest{
			Title:     title,
			Body:      body,
			GuildId:   i.GuildID,
			ChannelId: i.ChannelID,
			Tags:      tags,
		})
		if upsertErr != nil {
			_, followupErr := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: fmt.Sprintf("❌ Failed to create wiki page: %v", upsertErr),
				Flags:   discordgo.MessageFlagsEphemeral,
			})
			if followupErr != nil {
				log.Error("Failed to send followup", "error", followupErr)
			}
			return
		}

		// Add message as reference to the page (whether new or existing)
		_, err = wikiClient.AddWikiMessageReference(discordContextFor(i), &wikipb.AddWikiMessageReferenceRequest{
			WikiPageId:        upsertResp.Page.Id,
			MessageId:         message.ID,
			ChannelId:         message.ChannelID,
			GuildId:           guildID,
			Content:           message.Content,
			AuthorId:          message.Author.ID,
			AuthorUsername:    message.Author.Username,
			AuthorDisplayName: authorDisplayName,
			MessageTimestamp:  timestamppb.New(message.Timestamp),
			Attachments:       attachments,
		})
		if err != nil {
			log.Warn("Failed to add message reference", "error", err)
		}

		// Send appropriate success message
		var content string
		if !upsertResp.Created {
			content = fmt.Sprintf("✅ Message added to existing wiki page: **%s**", title)
		} else {
			content = fmt.Sprintf("✅ Wiki page created: **%s**", title)
		}
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
	} else {
		// Adding reference to existing page
		// Check if user provided updated content
		if body != "" {
			// User wants to update page content
			// First get the existing page
			var existingPage *wikipb.WikiPage
			existingPage, pageErr := wikiClient.GetWikiPage(discordContextFor(i), &wikipb.GetWikiPageRequest{
				Id: pageID,
			})
			if pageErr != nil {
				_, followupErr := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
					Content: fmt.Sprintf("❌ Failed to fetch wiki page: %v", pageErr),
					Flags:   discordgo.MessageFlagsEphemeral,
				})
				if followupErr != nil {
					log.Error("Failed to send followup", "error", followupErr)
				}
				return
			}

			// Only update if content actually changed
			if body != existingPage.Body {
				_, err = wikiClient.UpdateWikiPage(discordContextFor(i), &wikipb.UpdateWikiPageRequest{
					Id:    pageID,
					Title: existingPage.Title,
					Body:  body,
					Tags:  extractHashtags(body),
				})
				if err != nil {
					log.Warn("Failed to update wiki page", "error", err)
				}
			}
		}

		// Add message as reference
		log.Info("Adding wiki message reference",
			"guild_id", guildID,
			"content", message.Content,
			"content_len", len(message.Content))
		_, err = wikiClient.AddWikiMessageReference(discordContextFor(i), &wikipb.AddWikiMessageReferenceRequest{
			WikiPageId:        pageID,
			MessageId:         message.ID,
			ChannelId:         message.ChannelID,
			GuildId:           guildID,
			Content:           message.Content,
			AuthorId:          message.Author.ID,
			AuthorUsername:    message.Author.Username,
			AuthorDisplayName: authorDisplayName,
			MessageTimestamp:  timestamppb.New(message.Timestamp),
			Attachments:       attachments,
		})
		if err != nil {
			_, followupErr := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: fmt.Sprintf("❌ Failed to add message reference: %v", err),
				Flags:   discordgo.MessageFlagsEphemeral,
			})
			if followupErr != nil {
				log.Error("Failed to send followup", "error", followupErr)
			}
			return
		}

		// Get page title for confirmation
		page, err := wikiClient.GetWikiPage(discordContextFor(i), &wikipb.GetWikiPageRequest{
			Id: pageID,
		})
		pageName := "wiki page"
		if err == nil {
			pageName = fmt.Sprintf("**%s**", page.Title)
		}

		content := fmt.Sprintf("✅ Message added to %s", pageName)
		if body != "" {
			content += " (page content updated)"
		}

		_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		if err != nil {
			log.Error("Failed to send followup", "error", err)
		}
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
