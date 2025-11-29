package handlers

import (
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"

	wikipb "github.com/devilmonastery/hivemind/api/generated/go/wikipb"
	"github.com/devilmonastery/hivemind/internal/client"
)

// handleWiki routes /wiki subcommands to the appropriate handler
func handleWiki(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger, grpcClient *client.Client) {
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		respondError(s, i, "No subcommand provided", log)
		return
	}

	subcommand := options[0]

	switch subcommand.Name {
	case "search":
		handleWikiSearch(s, i, subcommand, log, grpcClient)
	case "view":
		handleWikiGet(s, i, subcommand, log, grpcClient)
	case "create":
		handleWikiCreate(s, i, subcommand, log, grpcClient)
	default:
		respondError(s, i, "Unknown wiki subcommand", log)
	}
}

func handleWikiSearch(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption, log *slog.Logger, grpcClient *client.Client) {
	// Parse query parameter
	var query string
	for _, opt := range subcommand.Options {
		if opt.Name == "query" {
			query = opt.StringValue()
		}
	}

	if query == "" {
		respondError(s, i, "Search query is required", log)
		return
	}

	ctx := discordContextFor(i)

	// Call backend to search wiki pages
	wikiClient := wikipb.NewWikiServiceClient(grpcClient.Conn())
	resp, err := wikiClient.SearchWikiPages(ctx, &wikipb.SearchWikiPagesRequest{
		GuildId: i.GuildID,
		Query:   query,
		Limit:   5,
	})
	if err != nil {
		log.Error("failed to search wiki pages",
			slog.String("error", err.Error()),
			slog.String("query", query))
		respondError(s, i, fmt.Sprintf("Failed to search: %v", err), log)
		return
	}

	// Format response
	var content string
	if len(resp.Pages) == 0 {
		content = fmt.Sprintf("🔍 No wiki pages found for: **%s**", query)
	} else {
		content = fmt.Sprintf("🔍 Found %d wiki pages for: **%s**\n\n", resp.Total, query)
		for idx, page := range resp.Pages {
			content += fmt.Sprintf("%d. **%s** (by %s)\n", idx+1, page.Title, page.AuthorUsername)
			// Show preview of body (first 100 chars)
			preview := page.Body
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			content += fmt.Sprintf("   %s\n\n", preview)
		}
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
		},
	})
	if err != nil {
		log.Error("failed to respond to wiki search", slog.String("error", err.Error()))
	}
}

func handleWikiGet(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption, log *slog.Logger, grpcClient *client.Client) {
	// Parse title parameter
	var title string
	for _, opt := range subcommand.Options {
		if opt.Name == "title" {
			title = opt.StringValue()
		}
	}

	if title == "" {
		respondError(s, i, "Title is required", log)
		return
	}

	ctx := discordContextFor(i)

	// Search by title to get ID (simplified approach)
	wikiClient := wikipb.NewWikiServiceClient(grpcClient.Conn())
	resp, err := wikiClient.SearchWikiPages(ctx, &wikipb.SearchWikiPagesRequest{
		GuildId: i.GuildID,
		Query:   title,
		Limit:   1,
	})
	if err != nil {
		log.Error("failed to get wiki page",
			slog.String("error", err.Error()),
			slog.String("title", title))
		respondError(s, i, fmt.Sprintf("Failed to get page: %v", err), log)
		return
	}

	if len(resp.Pages) == 0 {
		respondError(s, i, fmt.Sprintf("Wiki page not found: **%s**", title), log)
		return
	}

	page := resp.Pages[0]

	// Format as Discord embed
	embed := &discordgo.MessageEmbed{
		Title:       page.Title,
		Description: page.Body,
		Color:       0x5865F2, // Discord blurple
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Created by %s", page.AuthorUsername),
		},
		Timestamp: page.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z"),
	}

	if len(page.Tags) > 0 {
		embed.Fields = []*discordgo.MessageEmbedField{
			{
				Name:   "Tags",
				Value:  fmt.Sprintf("`%s`", page.Tags[0]),
				Inline: true,
			},
		}
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
	if err != nil {
		log.Error("failed to respond to wiki get", slog.String("error", err.Error()))
	}
}

func handleWikiCreate(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption, log *slog.Logger, grpcClient *client.Client) {
	// For create, we'll use a modal to get title and body
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: "wiki_create_modal",
			Title:    "Create Wiki Page",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "wiki_title",
							Label:       "Title",
							Style:       discordgo.TextInputShort,
							Placeholder: "Enter wiki page title",
							Required:    true,
							MaxLength:   200,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "wiki_body",
							Label:       "Content",
							Style:       discordgo.TextInputParagraph,
							Placeholder: "Enter wiki page content (markdown supported)",
							Required:    true,
							MaxLength:   4000,
						},
					},
				},
			},
		},
	})
	if err != nil {
		log.Error("failed to show wiki create modal", slog.String("error", err.Error()))
	}
}

// handleWikiCreateModal processes the modal submission for wiki page creation
func handleWikiCreateModal(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger, grpcClient *client.Client) {
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

	ctx := discordContextFor(i)

	// Call backend to create wiki page
	wikiClient := wikipb.NewWikiServiceClient(grpcClient.Conn())
	page, err := wikiClient.CreateWikiPage(ctx, &wikipb.CreateWikiPageRequest{
		Title:     title,
		Body:      body,
		GuildId:   i.GuildID,
		ChannelId: i.ChannelID,
	})
	if err != nil {
		log.Error("failed to create wiki page",
			slog.String("error", err.Error()),
			slog.String("title", title))
		respondError(s, i, fmt.Sprintf("Failed to create wiki page: %v", err), log)
		return
	}

	// Success response
	content := fmt.Sprintf("✅ Wiki page created: **%s**\nID: `%s`", page.Title, page.Id)

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
		},
	})
	if err != nil {
		log.Error("failed to respond to wiki create", slog.String("error", err.Error()))
	}
}
