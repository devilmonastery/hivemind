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
	"github.com/devilmonastery/hivemind/bot/internal/config"
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

	// Use the message content directly without the "Referenced from:" prefix
	// since message references will be shown in the embed
	messageContent := message.Content

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
							Value:       messageContent,
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

	// Show modal with title and body inputs for creating a new wiki page
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: fmt.Sprintf("context_wiki_modal:%s", targetID),
			Title:    "Add to Wiki Page",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "wiki_title",
							Label:       "Wiki Page Title",
							Style:       discordgo.TextInputShort,
							Required:    true,
							MaxLength:   200,
							Placeholder: "Enter page title",
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

	var quoteText string
	for _, comp := range data.Components {
		if actionRow, ok := comp.(*discordgo.ActionsRow); ok {
			for _, innerComp := range actionRow.Components {
				if textInput, ok := innerComp.(*discordgo.TextInput); ok {
					switch textInput.CustomID {
					case "quote_text":
						quoteText = textInput.Value
					case "quote_author":
						// Author info is captured from the source message, not from modal input
						// This field is just for display in the modal
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

	// Extract message ID from custom ID
	// Format: "context_quote_modal:MESSAGE_ID"
	var sourceMessageID, sourceChannelID, sourceChannelName, sourceAuthorDiscordID, sourceAuthorUsername string
	customID := data.CustomID
	parts := strings.Split(customID, ":")
	if len(parts) == 2 {
		sourceMessageID = parts[1]
		// Fetch the message to get channel and author details
		message, msgErr := s.ChannelMessage(i.ChannelID, sourceMessageID)
		if msgErr != nil {
			log.Warn("Failed to fetch original message", "message_id", sourceMessageID, "error", msgErr)
		} else {
			sourceChannelID = message.ChannelID
			sourceAuthorDiscordID = message.Author.ID
			sourceAuthorUsername = message.Author.Username

			// Fetch channel name
			channel, chanErr := s.Channel(sourceChannelID)
			if chanErr != nil {
				log.Warn("Failed to fetch channel", "channel_id", sourceChannelID, "error", chanErr)
			} else {
				sourceChannelName = channel.Name
			}
		}
	}

	req := &quotespb.CreateQuoteRequest{
		Body:                     quoteText,
		Tags:                     tags,
		GuildId:                  i.GuildID,
		SourceMsgId:              sourceMessageID,
		SourceChannelId:          sourceChannelID,
		SourceChannelName:        sourceChannelName,
		SourceMsgAuthorDiscordId: sourceAuthorDiscordID,
		SourceMsgAuthorUsername:  sourceAuthorUsername,
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

	// Show the created quote with standard embed
	embed := buildQuoteEmbed(resp)
	embed.Title = "✅ Quote Saved"

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embed},
		Flags:  discordgo.MessageFlagsEphemeral,
	})
	if err != nil {
		log.Error("Failed to send followup", "error", err)
	}
}

// handleContextNoteModal handles the modal submission for context menu note
func handleContextNoteModal(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *config.Config, log *slog.Logger, grpcClient *client.Client) {
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

	// Extract target message ID from CustomID (format: "context_note_modal:MESSAGE_ID")
	parts := strings.Split(data.CustomID, ":")
	var messageID string
	if len(parts) == 2 {
		messageID = parts[1]
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

	// If we have a message ID, add it as a reference to the note
	if messageID != "" {
		// Fetch the original message to get its details
		message, fetchErr := s.ChannelMessage(i.ChannelID, messageID)
		if fetchErr == nil {
			// Extract attachment metadata
			attachments := make([]*notespb.AttachmentMetadata, 0, len(message.Attachments))
			for _, attachment := range message.Attachments {
				attachments = append(attachments, &notespb.AttachmentMetadata{
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

			// Add message reference to note
			_, err = noteClient.AddNoteMessageReference(ctx, &notespb.AddNoteMessageReferenceRequest{
				NoteId:            resp.Id,
				MessageId:         message.ID,
				ChannelId:         message.ChannelID,
				GuildId:           i.GuildID,
				Content:           message.Content,
				AuthorId:          message.Author.ID,
				AuthorUsername:    message.Author.Username,
				AuthorDisplayName: authorDisplayName,
				MessageTimestamp:  timestamppb.New(message.Timestamp),
				Attachments:       attachments,
			})
			if err != nil {
				log.Warn("Failed to add message reference to note", "error", err)
				// Don't fail the whole operation if reference addition fails
			}
		} else {
			log.Warn("Failed to fetch message for note reference", "error", err, "message_id", messageID)
		}
	}

	// Fetch message references for the created note
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

// handleContextWikiModal handles the modal submission for context menu wiki
func handleContextWikiModal(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *config.Config, log *slog.Logger, grpcClient *client.Client) {
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

	// Extract message ID from CustomID (format: "context_wiki_modal:MESSAGE_ID")
	parts := strings.Split(data.CustomID, ":")
	var messageID string
	if len(parts) == 2 {
		messageID = parts[1]
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

	// Check if a page with this title already exists
	searchResp, err := wikiClient.SearchWikiPages(ctx, &wikipb.SearchWikiPagesRequest{
		GuildId: i.GuildID,
		Query:   title,
		Limit:   25,
	})

	// If page exists, append to body instead of replacing
	var existingPage *wikipb.WikiPage
	if err == nil && searchResp.Total > 0 {
		// Find exact title match (case-insensitive)
		for _, p := range searchResp.Pages {
			if strings.EqualFold(p.Title, title) {
				existingPage = p
				break
			}
		}
	}

	if existingPage != nil {
		// Append new content to existing body
		body = existingPage.Body + "\n\n" + body
		// Merge tags
		tagSet := make(map[string]bool)
		for _, t := range existingPage.Tags {
			tagSet[t] = true
		}
		for _, t := range tags {
			tagSet[t] = true
		}
		tags = make([]string, 0, len(tagSet))
		for t := range tagSet {
			tags = append(tags, t)
		}
	}

	// Use upsert to create or update the page
	resp, err := wikiClient.UpsertWikiPage(ctx, &wikipb.UpsertWikiPageRequest{
		Title:     title,
		Body:      body,
		Tags:      tags,
		GuildId:   i.GuildID,
		ChannelId: i.ChannelID,
	})
	if err != nil {
		log.Error("Failed to upsert wiki page", "error", err)
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("❌ Failed to save wiki page: %v", err),
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	page := resp.Page

	// If we have a message ID, add it as a reference to the wiki page
	if messageID != "" {
		// Fetch the original message to get its details
		message, fetchErr := s.ChannelMessage(i.ChannelID, messageID)
		if fetchErr == nil {
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

			// Add message reference
			_, err = wikiClient.AddWikiMessageReference(ctx, &wikipb.AddWikiMessageReferenceRequest{
				WikiPageId:        page.Id,
				MessageId:         message.ID,
				ChannelId:         message.ChannelID,
				GuildId:           i.GuildID,
				Content:           message.Content,
				AuthorId:          message.Author.ID,
				AuthorUsername:    message.Author.Username,
				AuthorDisplayName: authorDisplayName,
				MessageTimestamp:  timestamppb.New(message.Timestamp),
				Attachments:       attachments,
			})
			if err != nil {
				log.Warn("Failed to add message reference to wiki page", "error", err)
				// Don't fail the whole operation if reference addition fails
			}
		} else {
			log.Warn("Failed to fetch message for wiki reference", "error", fetchErr, "message_id", messageID)
		}
	}

	// Fetch message references for the wiki page
	refs := fetchWikiMessageReferences(ctx, wikiClient, page.Id, log)

	// Show standard wiki embed
	embed, components := showWikiDetailEmbed(s, page, refs, cfg, "", false)

	// Set title based on whether page was created or updated
	if resp.Created {
		embed.Title = "✅ Wiki Page Created\n\n" + embed.Title
	} else {
		embed.Title = "✅ Wiki Page Updated\n\n" + embed.Title
	}

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: components,
		Flags:      discordgo.MessageFlagsEphemeral,
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

// handleContextMenuAddNoteForUser handles "Add Note for User" context menu command
func handleContextMenuAddNoteForUser(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *config.Config, log *slog.Logger, grpcClient *client.Client) {
	// Get the target user
	targetID := i.ApplicationCommandData().TargetID
	user := i.ApplicationCommandData().Resolved.Users[targetID]

	if user == nil {
		respondError(s, i, "Could not find the target user", log)
		return
	}

	// Search for existing notes about this user
	noteClient := notespb.NewNoteServiceClient(grpcClient.Conn())
	ctx := discordContextFor(i)

	resp, err := noteClient.SearchNotes(ctx, &notespb.SearchNotesRequest{
		Query:   user.Username,
		GuildId: i.GuildID,
		Limit:   1, // Just get the first one
	})

	// Default values
	defaultTitle := user.Username
	if user.GlobalName != "" {
		defaultTitle = user.GlobalName
	}
	defaultBody := ""
	noteID := "new"
	modalTitle := fmt.Sprintf("Note about @%s", user.Username)

	// If existing note found, pre-fill with its content
	if err == nil && len(resp.Notes) > 0 {
		existingNote := resp.Notes[0]
		defaultTitle = existingNote.Title
		defaultBody = existingNote.Body
		noteID = existingNote.Id
		modalTitle = fmt.Sprintf("Edit Note about @%s", user.Username)

		// Remove the user mention prefix if present (added during creation)
		userMentionPrefix := fmt.Sprintf("**Note about <@%s>", targetID)
		if strings.HasPrefix(defaultBody, userMentionPrefix) {
			// Find the end of the first line and remove it
			if idx := strings.Index(defaultBody, "\n\n"); idx != -1 {
				defaultBody = defaultBody[idx+2:]
			}
		}

		// Check if body exceeds Discord's modal limit
		if len(defaultBody) > 4000 {
			truncatedBody := defaultBody[:3900]
			defaultBody = truncatedBody + "\n\n[Note truncated - please use web interface to edit full content]"
		}
	}

	// Show modal to create or edit a note about this user
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: fmt.Sprintf("user_note_modal:%s:%s", targetID, noteID),
			Title:    modalTitle,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "note_title",
							Label:       "Title",
							Style:       discordgo.TextInputShort,
							Required:    true,
							Value:       defaultTitle,
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
							Value:       defaultBody,
							MaxLength:   4000,
							Placeholder: "What do you want to remember about this person? Use #hashtags for tags.",
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

// handleContextMenuViewNotesForUser handles "View Notes for User" context menu command
func handleContextMenuViewNotesForUser(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *config.Config, log *slog.Logger, grpcClient *client.Client) {
	// Get the target user
	targetID := i.ApplicationCommandData().TargetID
	user := i.ApplicationCommandData().Resolved.Users[targetID]

	if user == nil {
		respondError(s, i, "Could not find the target user", log)
		return
	}

	// Acknowledge the interaction
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

	// Search for notes mentioning this user
	noteClient := notespb.NewNoteServiceClient(grpcClient.Conn())
	resp, err := noteClient.SearchNotes(discordContextFor(i), &notespb.SearchNotesRequest{
		Query:   user.Username,
		GuildId: i.GuildID,
		Limit:   10,
	})
	if err != nil {
		log.Error("Failed to search notes", "error", err)
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "❌ Failed to search notes. Please try again.",
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	// If no notes found, show message
	if len(resp.Notes) == 0 {
		content := fmt.Sprintf("📝 No notes found mentioning **@%s**\n\nCreate a new note using the \"Add Note for User\" context menu option.", user.Username)
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	// Display each note with embed and action buttons
	ctx := discordContextFor(i)
	for idx, note := range resp.Notes {
		// Fetch message references for each note
		refs := fetchNoteMessageReferences(ctx, noteClient, note.Id, log)
		embed, components := createNoteEmbed(note, refs, cfg)

		// Add note number to embed title
		if idx == 0 {
			embed.Title = fmt.Sprintf("📝 %d note(s) for @%s\n\n%s", len(resp.Notes), user.Username, embed.Title)
		}

		_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
			Flags:      discordgo.MessageFlagsEphemeral,
		})
		if err != nil {
			log.Error("Failed to send note", "error", err)
		}
	}
}

// handleUserNoteModal handles submission of the user note modal
func handleUserNoteModal(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *config.Config, log *slog.Logger, grpcClient *client.Client) {
	// Extract user ID and note ID from custom ID (format: user_note_modal:{userID}:{noteID})
	parts := strings.SplitN(i.ModalSubmitData().CustomID, ":", 3)
	if len(parts) != 3 {
		respondError(s, i, "Invalid modal format", log)
		return
	}
	targetUserID := parts[1]
	noteID := parts[2]

	// Get the target user
	targetUser, err := s.User(targetUserID)
	if err != nil {
		respondError(s, i, "Failed to get user information", log)
		return
	}

	// Extract fields from modal
	data := i.ModalSubmitData()
	var title, body string
	for _, component := range data.Components {
		if actionRow, ok := component.(*discordgo.ActionsRow); ok {
			for _, innerComponent := range actionRow.Components {
				if textInput, ok := innerComponent.(*discordgo.TextInput); ok {
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

	// Prepend user mention to body
	userMention := fmt.Sprintf("**Note about <@%s> (@%s)**\n\n", targetUserID, targetUser.Username)
	body = userMention + body

	// Extract hashtags for tags
	tags := extractHashtags(body)

	// Defer response
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

	ctx := discordContextFor(i)
	noteClient := notespb.NewNoteServiceClient(grpcClient.Conn())

	var resultNote *notespb.Note
	var actionText string

	// Check if we're updating an existing note or creating a new one
	if noteID != "new" {
		// Update existing note
		updateReq := &notespb.UpdateNoteRequest{
			Id:    noteID,
			Title: title,
			Body:  body,
			Tags:  tags,
		}

		resultNote, err = noteClient.UpdateNote(ctx, updateReq)
		if err != nil {
			log.Error("Failed to update note", "note_id", noteID, "error", err)
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: fmt.Sprintf("❌ Failed to update note: %v", err),
				Flags:   discordgo.MessageFlagsEphemeral,
			})
			return
		}
		actionText = "updated"
	} else {
		// Create new note
		createReq := &notespb.CreateNoteRequest{
			Title:   title,
			Body:    body,
			GuildId: i.GuildID,
			Tags:    tags,
		}

		resultNote, err = noteClient.CreateNote(ctx, createReq)
		if err != nil {
			log.Error("Failed to create note", "error", err)
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: fmt.Sprintf("❌ Failed to create note: %v", err),
				Flags:   discordgo.MessageFlagsEphemeral,
			})
			return
		}

		// Add user message reference for new notes
		refReq := &notespb.AddNoteMessageReferenceRequest{
			NoteId:            resultNote.Id,
			MessageId:         "",
			ChannelId:         "",
			GuildId:           i.GuildID,
			Content:           "",
			AuthorId:          targetUserID,
			AuthorUsername:    targetUser.Username,
			AuthorDisplayName: targetUser.GlobalName,
			MessageTimestamp:  timestamppb.Now(),
		}

		_, err = noteClient.AddNoteMessageReference(ctx, refReq)
		if err != nil {
			log.Warn("Failed to add user reference", "error", err)
			// Don't fail the whole operation if reference fails
		}
		actionText = "created"
	}

	// Success response - show standard note embed
	// Fetch message references
	refs := fetchNoteMessageReferences(ctx, noteClient, resultNote.Id, log)
	embed, components := createNoteEmbed(resultNote, refs, cfg)

	// Add action text to title
	if actionText == "updated" {
		embed.Title = "✅ Note Updated\n\n" + embed.Title
	} else {
		embed.Title = "✅ Note Created\n\n" + embed.Title
	}

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: components,
		Flags:      discordgo.MessageFlagsEphemeral,
	})
	if err != nil {
		log.Error("Failed to send followup", "error", err)
	}

	log.Info("Note operation completed via user context menu",
		"note_id", resultNote.Id,
		"action", actionText,
		"title", title,
		"author_id", i.Member.User.ID,
		"target_user_id", targetUserID,
	)
}
