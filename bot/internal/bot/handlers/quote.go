package handlers

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
	quotespb "github.com/devilmonastery/hivemind/api/generated/go/quotespb"
	"github.com/devilmonastery/hivemind/internal/client"
	"github.com/devilmonastery/hivemind/internal/pkg/urlutil"
)

// buildQuoteEmbed creates a standardized embed for displaying a quote
func buildQuoteEmbed(quote *quotespb.Quote) *discordgo.MessageEmbed {
	// Format the quote body with markdown quote styling
	quoteText := ""

	// Add attribution header if we have the original author
	if quote.SourceMsgAuthorUsername != "" {
		quoteText = fmt.Sprintf("**%s** said:\n", quote.SourceMsgAuthorUsername)
	}

	// Add the quote with > markdown
	lines := strings.Split(quote.Body, "\n")
	for _, line := range lines {
		quoteText += "> " + line + "\n"
	}

	// Add footer with timestamp, added by, and original message link (de-emphasized with italics)
	footer := []string{}

	// Add timestamp if available
	if quote.SourceMsgTimestamp != nil {
		footer = append(footer, fmt.Sprintf("_<t:%d:D>_", quote.SourceMsgTimestamp.Seconds))
	}

	if quote.AuthorUsername != "" {
		footer = append(footer, fmt.Sprintf("_added by %s_", quote.AuthorUsername))
	}

	if quote.SourceMsgId != "" && quote.SourceChannelId != "" && quote.GuildId != "" {
		messageURL := urlutil.DiscordMessageURL(quote.GuildId, quote.SourceChannelId, quote.SourceMsgId)
		footer = append(footer, fmt.Sprintf("_[original message](%s)_", messageURL))
	}

	if len(footer) > 0 {
		quoteText += "\n" + strings.Join(footer, " • ")
	}

	embed := &discordgo.MessageEmbed{
		Description: quoteText,
		Color:       0x5865F2, // Discord blurple
	}

	// Add tags field if present
	if len(quote.Tags) > 0 {
		embed.Fields = []*discordgo.MessageEmbedField{
			{
				Name:   "Tags",
				Value:  strings.Join(quote.Tags, ", "),
				Inline: false,
			},
		}
	}

	return embed
}

// buildQuoteActionButtons creates the standard action buttons for quote interactions
// Only shows the Edit button if currentUserDiscordID matches the quote's author_discord_id
func buildQuoteActionButtons(quote *quotespb.Quote, currentUserDiscordID string, log *slog.Logger) []discordgo.MessageComponent {
	buttons := []discordgo.MessageComponent{
		discordgo.Button{
			Label:    "📢 Add to Chat",
			Style:    discordgo.SuccessButton,
			CustomID: fmt.Sprintf("quote_add_to_chat:%s", quote.Id),
		},
	}

	// Debug logging
	log.Info("Checking quote edit permission",
		"quote_id", quote.Id,
		"quote_author_discord_id", quote.AuthorDiscordId,
		"current_user_discord_id", currentUserDiscordID,
		"match", quote.AuthorDiscordId == currentUserDiscordID)

	// Only show Edit button if the current user is the author (by Discord ID)
	if quote.AuthorDiscordId == currentUserDiscordID {
		buttons = append(buttons, discordgo.Button{
			Label:    "✏️ Edit",
			Style:    discordgo.PrimaryButton,
			CustomID: fmt.Sprintf("quote_edit_btn:%s", quote.Id),
		})
	}

	buttons = append(buttons, discordgo.Button{
		Label:    "❌ Dismiss",
		Style:    discordgo.SecondaryButton,
		CustomID: fmt.Sprintf("quote_dismiss:%s", quote.Id),
	})

	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: buttons,
		},
	}
}

// handleQuote routes /quote subcommands to the appropriate handler
func handleQuote(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger, grpcClient *client.Client) {
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		respondError(s, i, "No subcommand provided", log)
		return
	}

	subcommand := options[0]

	switch subcommand.Name {
	case "random":
		handleQuoteRandom(s, i, subcommand, log, grpcClient)
	case "search":
		handleQuoteSearch(s, i, subcommand, log, grpcClient)
	default:
		respondError(s, i, "Unknown quote subcommand", log)
	}
}

// handleQuoteRandom gets a random quote and shows it ephemerally with action buttons
func handleQuoteRandom(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption, log *slog.Logger, grpcClient *client.Client) {
	var tags []string
	for _, opt := range subcommand.Options {
		if opt.Name == "tags" {
			tagStr := opt.StringValue()
			parts := strings.Split(tagStr, ",")
			for _, tag := range parts {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					tags = append(tags, tag)
				}
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

	quoteClient := quotespb.NewQuoteServiceClient(grpcClient.Conn())
	ctx := discordContextFor(i)

	resp, err := quoteClient.GetRandomQuote(ctx, &quotespb.GetRandomQuoteRequest{
		GuildId: i.GuildID,
		Tags:    tags,
	})
	if err != nil {
		log.Error("Failed to get random quote", "error", err)
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("❌ Failed to get random quote: %v", err),
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	// Use standard quote embed
	embed := buildQuoteEmbed(resp)

	// Get current user Discord ID
	var discordID string
	if i.Member != nil && i.Member.User != nil {
		discordID = i.Member.User.ID
	} else if i.User != nil {
		discordID = i.User.ID
	}

	// Build action buttons (ephemeral - user decides whether to share)
	components := buildQuoteActionButtons(resp, discordID, log)

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: components,
		Flags:      discordgo.MessageFlagsEphemeral,
	})
	if err != nil {
		log.Error("Failed to send followup", "error", err)
	}
}

// handleQuoteList lists quotes
// handleQuoteSearch searches quotes
func handleQuoteSearch(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption, log *slog.Logger, grpcClient *client.Client) {
	var query string
	var tags []string
	limit := int32(10)

	for _, opt := range subcommand.Options {
		switch opt.Name {
		case "query":
			query = opt.StringValue()
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

	if query == "" {
		respondError(s, i, "Search query is required", log)
		return
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

	quoteClient := quotespb.NewQuoteServiceClient(grpcClient.Conn())
	ctx := discordContextFor(i)

	resp, err := quoteClient.SearchQuotes(ctx, &quotespb.SearchQuotesRequest{
		Query:   query,
		GuildId: i.GuildID,
		Tags:    tags,
		Limit:   limit,
	})
	if err != nil {
		log.Error("Failed to search quotes", "error", err)
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("❌ Failed to search quotes: %v", err),
		})
		return
	}

	if len(resp.Quotes) == 0 {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("No quotes found matching \"%s\"", query),
			Flags:   discordgo.MessageFlagsEphemeral,
		})
		return
	}

	// Show search results with a dropdown to select and post a quote
	// Limit to 25 results (Discord's max for select menus)
	displayLimit := len(resp.Quotes)
	if displayLimit > 25 {
		displayLimit = 25
	}

	var content strings.Builder
	content.WriteString(fmt.Sprintf("🔍 Found %d quote(s) matching \"%s\"\n", resp.Total, query))
	content.WriteString("Select a quote from the dropdown to post it to the channel:")

	// Build select menu options
	options := []discordgo.SelectMenuOption{}
	for idx := 0; idx < displayLimit; idx++ {
		quote := resp.Quotes[idx]
		// Create a label (truncate if too long - Discord max is 100 chars)
		label := quote.Body
		if len(label) > 100 {
			label = label[:97] + "..."
		}

		// Create a description with attribution
		description := ""
		if quote.SourceMsgAuthorUsername != "" {
			description = fmt.Sprintf("by %s", quote.SourceMsgAuthorUsername)
			if len(description) > 100 {
				description = description[:97] + "..."
			}
		}

		options = append(options, discordgo.SelectMenuOption{
			Label:       label,
			Description: description,
			Value:       quote.Id,
			Emoji: &discordgo.ComponentEmoji{
				Name: "💬",
			},
		})
	}

	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "post_quote_select",
					Placeholder: "Choose a quote to post...",
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

// handlePostQuoteSelect handles dropdown selection to post a quote to the channel
func handlePostQuoteSelect(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger, grpcClient *client.Client) {
	// Get the selected quote ID from the dropdown
	data := i.MessageComponentData()
	if len(data.Values) == 0 {
		log.Warn("No value selected in dropdown")
		return
	}

	quoteID := data.Values[0]

	// Fetch the quote by ID
	quoteClient := quotespb.NewQuoteServiceClient(grpcClient.Conn())
	ctx := discordContextFor(i)

	quote, err := quoteClient.GetQuote(ctx, &quotespb.GetQuoteRequest{
		Id: quoteID,
	})
	if err != nil {
		log.Error("Failed to fetch quote", "quote_id", quoteID, "error", err)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ Failed to fetch quote",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Show the quote ephemerally with action buttons
	embed := buildQuoteEmbed(quote)

	// Get current user Discord ID
	var discordID string
	if i.Member != nil && i.Member.User != nil {
		discordID = i.Member.User.ID
	} else if i.User != nil {
		discordID = i.User.ID
	}

	components := buildQuoteActionButtons(quote, discordID, log)

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Error("Failed to update interaction with quote", "error", err)
	}
}

// handleQuoteAddToChat posts the quote to the channel when "Add to Chat" is clicked
func handleQuoteAddToChat(s *discordgo.Session, i *discordgo.InteractionCreate, quoteID string, log *slog.Logger, grpcClient *client.Client) {
	quoteClient := quotespb.NewQuoteServiceClient(grpcClient.Conn())
	ctx := discordContextFor(i)

	// Fetch the quote
	quote, err := quoteClient.GetQuote(ctx, &quotespb.GetQuoteRequest{
		Id: quoteID,
	})
	if err != nil {
		log.Error("Failed to fetch quote for add to chat", "quote_id", quoteID, "error", err)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ Failed to fetch quote",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Update the ephemeral message to show success
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    "✅ Posted to chat!",
			Components: []discordgo.MessageComponent{}, // Remove buttons
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Error("Failed to update interaction", "error", err)
		return
	}

	// Post the quote to the channel
	embed := buildQuoteEmbed(quote)
	log.Debug("sending quote message to Discord",
		"channel_id", i.ChannelID,
		"quote_id", quote.Id)
	_, err = s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{embed},
	})
	if err != nil {
		log.Error("Failed to post quote to channel", "error", err)
	}
}

// handleQuoteEditButton opens a modal for editing the quote
func handleQuoteEditButton(s *discordgo.Session, i *discordgo.InteractionCreate, quoteID string, log *slog.Logger, grpcClient *client.Client) {
	quoteClient := quotespb.NewQuoteServiceClient(grpcClient.Conn())
	ctx := discordContextFor(i)

	// Fetch the quote
	quote, err := quoteClient.GetQuote(ctx, &quotespb.GetQuoteRequest{
		Id: quoteID,
	})
	if err != nil {
		log.Error("Failed to fetch quote for editing", "quote_id", quoteID, "error", err)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ Failed to fetch quote",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Show edit modal
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: fmt.Sprintf("quote_edit_modal:%s", quoteID),
			Title:    "Edit Quote",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "quote_body",
							Label:       "Quote Text",
							Style:       discordgo.TextInputParagraph,
							Placeholder: "Enter the quote text...",
							Required:    true,
							Value:       quote.Body,
							MaxLength:   2000,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "quote_tags",
							Label:       "Tags (comma-separated)",
							Style:       discordgo.TextInputShort,
							Placeholder: "funny, memorable, etc.",
							Required:    false,
							Value:       strings.Join(quote.Tags, ", "),
							MaxLength:   200,
						},
					},
				},
			},
		},
	})
	if err != nil {
		log.Error("Failed to show edit modal", "error", err)
	}
}

// handleQuoteDismiss dismisses the ephemeral quote message
func handleQuoteDismiss(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    "Quote dismissed.",
			Embeds:     []*discordgo.MessageEmbed{},
			Components: []discordgo.MessageComponent{},
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Error("Failed to dismiss quote", "error", err)
	}
}

// handleQuoteEditModal processes the quote edit modal submission
func handleQuoteEditModal(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger, grpcClient *client.Client) {
	data := i.ModalSubmitData()

	// Extract quote ID from custom_id
	parts := strings.SplitN(data.CustomID, ":", 2)
	if len(parts) < 2 {
		log.Error("Invalid modal custom_id", "custom_id", data.CustomID)
		respondError(s, i, "Invalid modal submission", log)
		return
	}
	quoteID := parts[1]

	// Extract form values
	var body, tagsStr string
	for _, component := range data.Components {
		if actionRow, ok := component.(*discordgo.ActionsRow); ok {
			for _, comp := range actionRow.Components {
				if textInput, ok := comp.(*discordgo.TextInput); ok {
					switch textInput.CustomID {
					case "quote_body":
						body = textInput.Value
					case "quote_tags":
						tagsStr = textInput.Value
					}
				}
			}
		}
	}

	if body == "" {
		respondError(s, i, "Quote text cannot be empty", log)
		return
	}

	// Parse tags
	var tags []string
	if tagsStr != "" {
		tagParts := strings.Split(tagsStr, ",")
		for _, tag := range tagParts {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}

	// Update the quote
	quoteClient := quotespb.NewQuoteServiceClient(grpcClient.Conn())
	ctx := discordContextFor(i)

	_, err := quoteClient.UpdateQuote(ctx, &quotespb.UpdateQuoteRequest{
		Id:   quoteID,
		Body: body,
		Tags: tags,
	})
	if err != nil {
		log.Error("Failed to update quote", "quote_id", quoteID, "error", err)
		respondError(s, i, fmt.Sprintf("Failed to update quote: %v", err), log)
		return
	}

	// Fetch the updated quote
	updatedQuote, err := quoteClient.GetQuote(ctx, &quotespb.GetQuoteRequest{
		Id: quoteID,
	})
	if err != nil {
		log.Error("Failed to fetch updated quote", "quote_id", quoteID, "error", err)
		respondError(s, i, "Quote updated but failed to fetch updated version", log)
		return
	}

	// Show the updated quote with action buttons
	embed := buildQuoteEmbed(updatedQuote)

	// Get current user Discord ID
	var discordID string
	if i.Member != nil && i.Member.User != nil {
		discordID = i.Member.User.ID
	} else if i.User != nil {
		discordID = i.User.ID
	}

	components := buildQuoteActionButtons(updatedQuote, discordID, log)

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    "✅ Quote updated!",
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Error("Failed to respond to modal", "error", err)
	}
}
