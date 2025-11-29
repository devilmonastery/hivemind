package handlers

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
	quotespb "github.com/devilmonastery/hivemind/api/generated/go/quotespb"
	"github.com/devilmonastery/hivemind/internal/client"
)

// handleQuote routes /quote subcommands to the appropriate handler
func handleQuote(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger, grpcClient *client.Client) {
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		respondError(s, i, "No subcommand provided", log)
		return
	}

	subcommand := options[0]

	switch subcommand.Name {
	case "add":
		handleQuoteAdd(s, i, subcommand, log, grpcClient)
	case "random":
		handleQuoteRandom(s, i, subcommand, log, grpcClient)
	case "list":
		handleQuoteList(s, i, subcommand, log, grpcClient)
	case "search":
		handleQuoteSearch(s, i, subcommand, log, grpcClient)
	default:
		respondError(s, i, "Unknown quote subcommand", log)
	}
}

// handleQuoteAdd adds a new quote
func handleQuoteAdd(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption, log *slog.Logger, grpcClient *client.Client) {
	var text string
	for _, opt := range subcommand.Options {
		if opt.Name == "text" {
			text = opt.StringValue()
		}
	}

	if text == "" {
		respondError(s, i, "Quote text is required", log)
		return
	}

	// Extract hashtags from text
	tags := extractHashtags(text)

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

	req := &quotespb.CreateQuoteRequest{
		Body:    text,
		Tags:    tags,
		GuildId: i.GuildID,
		// Note: source message fields would be filled if quoting a specific message
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

	content := fmt.Sprintf("✅ Quote added (ID: %s)", resp.Id)
	if len(tags) > 0 {
		content += fmt.Sprintf("\nTags: %s", strings.Join(tags, ", "))
	}

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: content,
		Flags:   discordgo.MessageFlagsEphemeral,
	})
	if err != nil {
		log.Error("Failed to send followup", "error", err)
	}
}

// handleQuoteRandom gets a random quote
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
		})
		return
	}

	embed := &discordgo.MessageEmbed{
		Description: fmt.Sprintf("\"%s\"", resp.Body),
		Color:       0x5865F2,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Added by %s", resp.AuthorUsername),
		},
		Timestamp: resp.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z"),
	}

	if len(resp.Tags) > 0 {
		embed.Fields = []*discordgo.MessageEmbedField{
			{
				Name:   "Tags",
				Value:  strings.Join(resp.Tags, ", "),
				Inline: true,
			},
		}
	}

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embed},
	})
	if err != nil {
		log.Error("Failed to send followup", "error", err)
	}
}

// handleQuoteList lists quotes
func handleQuoteList(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption, log *slog.Logger, grpcClient *client.Client) {
	var tags []string
	limit := int32(10)

	for _, opt := range subcommand.Options {
		switch opt.Name {
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
	})
	if err != nil {
		log.Error("Failed to defer response", "error", err)
		return
	}

	quoteClient := quotespb.NewQuoteServiceClient(grpcClient.Conn())
	ctx := discordContextFor(i)

	resp, err := quoteClient.ListQuotes(ctx, &quotespb.ListQuotesRequest{
		GuildId: i.GuildID,
		Tags:    tags,
		Limit:   limit,
	})
	if err != nil {
		log.Error("Failed to list quotes", "error", err)
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("❌ Failed to list quotes: %v", err),
		})
		return
	}

	if len(resp.Quotes) == 0 {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "No quotes found",
		})
		return
	}

	var content strings.Builder
	content.WriteString(fmt.Sprintf("💬 Found %d quote(s):\n\n", resp.Total))
	for idx, quote := range resp.Quotes {
		content.WriteString(fmt.Sprintf("%d. \"%s\" (by %s)\n", idx+1, quote.Body, quote.AuthorUsername))
		if len(quote.Tags) > 0 {
			content.WriteString(fmt.Sprintf("   Tags: %s\n", strings.Join(quote.Tags, ", ")))
		}
		content.WriteString("\n")
	}

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: content.String(),
	})
	if err != nil {
		log.Error("Failed to send followup", "error", err)
	}
}

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
		})
		return
	}

	var content strings.Builder
	content.WriteString(fmt.Sprintf("🔍 Found %d quote(s) matching \"%s\":\n\n", resp.Total, query))
	for idx, quote := range resp.Quotes {
		content.WriteString(fmt.Sprintf("%d. \"%s\" (by %s)\n", idx+1, quote.Body, quote.AuthorUsername))
		if len(quote.Tags) > 0 {
			content.WriteString(fmt.Sprintf("   Tags: %s\n", strings.Join(quote.Tags, ", ")))
		}
		content.WriteString("\n")
	}

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: content.String(),
	})
	if err != nil {
		log.Error("Failed to send followup", "error", err)
	}
}
