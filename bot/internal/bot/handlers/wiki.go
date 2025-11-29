package handlers

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"

	wikipb "github.com/devilmonastery/hivemind/api/generated/go/wikipb"
	"github.com/devilmonastery/hivemind/bot/internal/config"
	"github.com/devilmonastery/hivemind/internal/client"
)

// getWebBaseURL returns the web base URL from config or a default
func getWebBaseURL(cfg *config.Config) string {
	if cfg != nil && cfg.Backend.WebBaseURL != "" {
		return cfg.Backend.WebBaseURL
	}
	return "http://localhost:8081" // Default for development
}

// showWikiDetailEmbed creates the detailed embed and action buttons for a wiki page
func showWikiDetailEmbed(s *discordgo.Session, page *wikipb.WikiPage, cfg *config.Config, query string, showBackButton bool) (*discordgo.MessageEmbed, []discordgo.MessageComponent) {
	// Get channel name
	channel, _ := s.Channel(page.ChannelId)
	channelName := page.ChannelId
	if channel != nil {
		channelName = "#" + channel.Name
	}

	// Format tags
	tagsText := "None"
	if len(page.Tags) > 0 {
		tagsText = ""
		for idx, tag := range page.Tags {
			if idx > 0 {
				tagsText += ", "
			}
			tagsText += "#" + tag
		}
	}

	// Create detailed embed
	embed := &discordgo.MessageEmbed{
		Title:       page.Title,
		Description: page.Body,
		Color:       0x00D9FF, // Cyan
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "From",
				Value:  channelName,
				Inline: true,
			},
			{
				Name:   "Author",
				Value:  page.AuthorUsername,
				Inline: true,
			},
			{
				Name:   "Tags",
				Value:  tagsText,
				Inline: false,
			},
		},
		Timestamp: page.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z"),
	}

	// Build action buttons
	var components []discordgo.MessageComponent

	// First row: Cancel and optionally Back
	firstRow := []discordgo.MessageComponent{
		discordgo.Button{
			Label:    "❌ Cancel",
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("wiki_action_btn:cancel:%s", page.Id),
		},
	}
	if showBackButton {
		firstRow = append(firstRow, discordgo.Button{
			Label:    "◀ Back to Results",
			Style:    discordgo.SecondaryButton,
			CustomID: fmt.Sprintf("wiki_action_btn:back:%s:%s", page.Id, query),
		})
	}
	components = append(components, discordgo.ActionsRow{
		Components: firstRow,
	})

	// Second row: Add to Chat, Edit, View on Web
	components = append(components, discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "📢 Add to Chat",
				Style:    discordgo.SuccessButton,
				CustomID: fmt.Sprintf("wiki_action_btn:post:%s", page.Id),
			},
			discordgo.Button{
				Label:    "✏️ Edit",
				Style:    discordgo.PrimaryButton,
				CustomID: fmt.Sprintf("wiki_action_btn:edit:%s:%s", page.Id, page.Title),
			},
			discordgo.Button{
				Label: "🌐 View on Web",
				Style: discordgo.LinkButton,
				URL:   fmt.Sprintf("%s/wiki?guild=%s&title=%s", getWebBaseURL(cfg), page.GuildId, page.Title),
			},
		},
	})

	return embed, components
}

// stripMarkdownAndNewlines removes markdown formatting and newlines, returning plain text
func stripMarkdownAndNewlines(text string) string {
	// Replace newlines with spaces
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")

	// Strip common markdown formatting
	// Bold: **text** or __text__
	text = regexp.MustCompile(`\*\*([^*]+)\*\*`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`__([^_]+)__`).ReplaceAllString(text, "$1")

	// Italic: *text* or _text_
	text = regexp.MustCompile(`\*([^*]+)\*`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`_([^_]+)_`).ReplaceAllString(text, "$1")

	// Code: `text`
	text = regexp.MustCompile("`([^`]+)`").ReplaceAllString(text, "$1")

	// Headers: # text
	text = regexp.MustCompile(`#+\s*`).ReplaceAllString(text, "")

	// Links: [text](url)
	text = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(text, "$1")

	// Bullets and list markers
	text = regexp.MustCompile(`^\s*[-*+]\s+`).ReplaceAllString(text, "")

	// Clean up multiple spaces
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

// handleWiki routes /wiki subcommands to the appropriate handler
func handleWiki(s *discordgo.Session, i *discordgo.InteractionCreate, cfg *config.Config, log *slog.Logger, grpcClient *client.Client) {
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		respondError(s, i, "No subcommand provided", log)
		return
	}

	subcommand := options[0]

	switch subcommand.Name {
	case "search":
		handleWikiSearch(s, i, subcommand, cfg, log, grpcClient)
	case "view":
		handleWikiGet(s, i, subcommand, cfg, log, grpcClient)
	case "edit":
		handleWikiEdit(s, i, subcommand, cfg, log, grpcClient)
	default:
		respondError(s, i, "Unknown wiki subcommand", log)
	}
}

func handleWikiSearch(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption, cfg *config.Config, log *slog.Logger, grpcClient *client.Client) {
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

	// If only one result, show it directly
	if len(resp.Pages) == 1 {
		page := resp.Pages[0]
		embed, components := showWikiDetailEmbed(s, page, cfg, query, false)

		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds:     []*discordgo.MessageEmbed{embed},
				Components: components,
				Flags:      discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			log.Error("failed to respond to wiki search", slog.String("error", err.Error()))
		}
		return
	}

	// Multiple results - build select menu
	var options []discordgo.SelectMenuOption
	for idx, page := range resp.Pages {
		if idx >= 25 { // Discord limit for select menu options
			break
		}

		// Create short excerpt for description (max 100 chars)
		excerpt := stripMarkdownAndNewlines(page.Body)
		if len(excerpt) > 97 {
			excerpt = excerpt[:97] + "..."
		}

		// Format emoji based on tags
		emoji := "📄"
		if len(page.Tags) > 0 {
			emoji = "🏷️"
		}

		options = append(options, discordgo.SelectMenuOption{
			Label:       truncateString(page.Title, 100),
			Value:       fmt.Sprintf("wiki_result:%s", page.Id),
			Description: excerpt,
			Emoji: &discordgo.ComponentEmoji{
				Name: emoji,
			},
		})
	}

	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    fmt.Sprintf("wiki_select:%s", query),
					Placeholder: fmt.Sprintf("Select from %d results...", len(resp.Pages)),
					Options:     options,
					MinValues:   intPtr(1),
					MaxValues:   1,
				},
			},
		},
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    fmt.Sprintf("🔍 Found **%d** wiki pages for: **%s**", resp.Total, query),
			Components: components,
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Error("failed to respond to wiki search", slog.String("error", err.Error()))
	}
}

// intPtr returns a pointer to an int
func intPtr(i int) *int {
	return &i
}

// truncateString truncates a string to the specified length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func handleWikiGet(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption, cfg *config.Config, log *slog.Logger, grpcClient *client.Client) {
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

	// Add action buttons
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Edit",
					Style:    discordgo.PrimaryButton,
					CustomID: fmt.Sprintf("wiki_edit_btn:%s", page.Title),
				},
				discordgo.Button{
					Label: "View on Web",
					Style: discordgo.LinkButton,
					URL:   fmt.Sprintf("%s/wiki?guild=%s&title=%s", getWebBaseURL(cfg), page.GuildId, page.Title),
				},
			},
		},
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
		},
	})
	if err != nil {
		log.Error("failed to respond to wiki get", slog.String("error", err.Error()))
	}
}

func handleWikiEdit(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption, cfg *config.Config, log *slog.Logger, grpcClient *client.Client) {
	// Parse optional title parameter
	var title string
	for _, opt := range subcommand.Options {
		if opt.Name == "title" {
			title = opt.StringValue()
		}
	}

	// If title provided, fetch existing content
	var existingBody string
	if title != "" {
		ctx := discordContextFor(i)
		wikiClient := wikipb.NewWikiServiceClient(grpcClient.Conn())
		resp, err := wikiClient.SearchWikiPages(ctx, &wikipb.SearchWikiPagesRequest{
			GuildId: i.GuildID,
			Query:   title,
			Limit:   1,
		})
		if err == nil && len(resp.Pages) > 0 {
			existingBody = resp.Pages[0].Body
		}
	}

	// Create modal with optional pre-filled title and body
	titleInput := discordgo.TextInput{
		CustomID:    "wiki_title",
		Label:       "Title",
		Style:       discordgo.TextInputShort,
		Placeholder: "Enter wiki page title",
		Required:    true,
		MaxLength:   200,
	}
	if title != "" {
		titleInput.Value = title
	}

	bodyInput := discordgo.TextInput{
		CustomID:    "wiki_body",
		Label:       "Content",
		Style:       discordgo.TextInputParagraph,
		Placeholder: "Enter wiki page content (markdown supported). Use #hashtags to add tags",
		Required:    true,
		MaxLength:   4000,
	}
	if existingBody != "" {
		bodyInput.Value = existingBody
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: "wiki_edit_modal",
			Title:    "Edit Wiki Page",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						titleInput,
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						bodyInput,
					},
				},
			},
		},
	})
	if err != nil {
		log.Error("failed to show wiki edit modal", slog.String("error", err.Error()))
	}
}

// handleWikiEditModal processes the modal submission for wiki page creation/editing
func handleWikiEditModal(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger, grpcClient *client.Client) {
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

	// Extract hashtags from body
	tags := extractHashtags(body)

	ctx := discordContextFor(i)
	wikiClient := wikipb.NewWikiServiceClient(grpcClient.Conn())

	// Use upsert to create or update the page
	resp, err := wikiClient.UpsertWikiPage(ctx, &wikipb.UpsertWikiPageRequest{
		Title:     title,
		Body:      body,
		Tags:      tags,
		GuildId:   i.GuildID,
		ChannelId: i.ChannelID,
	})
	if err != nil {
		log.Error("failed to upsert wiki page",
			slog.String("error", err.Error()),
			slog.String("title", title))
		respondError(s, i, fmt.Sprintf("Failed to save wiki page: %v", err), log)
		return
	}

	// Success response
	actionVerb := "created"
	if !resp.Created {
		actionVerb = "updated"
	}
	content := fmt.Sprintf("✅ Wiki page %s: **%s**", actionVerb, resp.Page.Title)

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

// handleWikiEditButton handles the wiki edit button click
func handleWikiEditButton(s *discordgo.Session, i *discordgo.InteractionCreate, title string, cfg *config.Config, log *slog.Logger, grpcClient *client.Client) {
	// Fetch existing content
	ctx := discordContextFor(i)
	wikiClient := wikipb.NewWikiServiceClient(grpcClient.Conn())
	resp, err := wikiClient.SearchWikiPages(ctx, &wikipb.SearchWikiPagesRequest{
		GuildId: i.GuildID,
		Query:   title,
		Limit:   1,
	})

	var existingBody string
	if err == nil && len(resp.Pages) > 0 {
		existingBody = resp.Pages[0].Body
	}

	// Show modal with pre-filled content
	titleInput := discordgo.TextInput{
		CustomID:  "wiki_title",
		Label:     "Title",
		Style:     discordgo.TextInputShort,
		Value:     title,
		Required:  true,
		MaxLength: 200,
	}

	bodyInput := discordgo.TextInput{
		CustomID:    "wiki_body",
		Label:       "Content",
		Style:       discordgo.TextInputParagraph,
		Placeholder: "Enter wiki page content (markdown supported). Use #hashtags to add tags",
		Required:    true,
		MaxLength:   4000,
	}
	if existingBody != "" {
		bodyInput.Value = existingBody
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: "wiki_edit_modal",
			Title:    "Edit Wiki Page",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						titleInput,
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						bodyInput,
					},
				},
			},
		},
	})
	if err != nil {
		log.Error("failed to show wiki edit modal from button", slog.String("error", err.Error()))
	}
}

// handleWikiSelectMenu handles when user selects a wiki page from search results
func handleWikiSelectMenu(s *discordgo.Session, i *discordgo.InteractionCreate, query string, cfg *config.Config, log *slog.Logger, grpcClient *client.Client) {
	// Get selected value
	data := i.MessageComponentData()
	if len(data.Values) == 0 {
		return
	}

	// Extract wiki ID from value (format: wiki_result:ID)
	selectedValue := data.Values[0]
	if len(selectedValue) < 13 || selectedValue[:12] != "wiki_result:" {
		return
	}
	wikiID := selectedValue[12:]

	// Fetch the full wiki page
	ctx := discordContextFor(i)
	wikiClient := wikipb.NewWikiServiceClient(grpcClient.Conn())

	// Get by ID (we'll need to search and find matching ID)
	resp, err := wikiClient.SearchWikiPages(ctx, &wikipb.SearchWikiPagesRequest{
		GuildId: i.GuildID,
		Query:   query,
		Limit:   25,
	})
	if err != nil {
		log.Error("failed to fetch wiki page", slog.String("error", err.Error()))
		return
	}

	// Find the selected page
	var selectedPage *wikipb.WikiPage
	for _, page := range resp.Pages {
		if page.Id == wikiID {
			selectedPage = page
			break
		}
	}

	if selectedPage == nil {
		respondError(s, i, "Wiki page not found", log)
		return
	}

	// Create detailed embed and components
	embed, components := showWikiDetailEmbed(s, selectedPage, cfg, query, true)

	// Update the message with the detailed view
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Error("failed to update message with wiki details", slog.String("error", err.Error()))
	}
}

// handleWikiActionButton handles action buttons on wiki detail view
func handleWikiActionButton(s *discordgo.Session, i *discordgo.InteractionCreate, customID string, cfg *config.Config, log *slog.Logger, grpcClient *client.Client) {
	// Parse customID: wiki_action_btn:action:params...
	parts := splitCustomID(customID, "wiki_action_btn:")
	if len(parts) < 1 {
		return
	}

	action := parts[0]

	switch action {
	case "cancel":
		// Delete the message
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    "Search cancelled.",
				Embeds:     []*discordgo.MessageEmbed{},
				Components: []discordgo.MessageComponent{},
				Flags:      discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			log.Error("failed to cancel search", slog.String("error", err.Error()))
		}

	case "back":
		// Return to search results
		if len(parts) < 3 {
			return
		}
		query := parts[2]

		// Re-run the search
		ctx := discordContextFor(i)
		wikiClient := wikipb.NewWikiServiceClient(grpcClient.Conn())
		resp, err := wikiClient.SearchWikiPages(ctx, &wikipb.SearchWikiPagesRequest{
			GuildId: i.GuildID,
			Query:   query,
			Limit:   25,
		})
		if err != nil {
			log.Error("failed to re-fetch search results", slog.String("error", err.Error()))
			return
		}

		// Rebuild select menu
		var options []discordgo.SelectMenuOption
		for idx, page := range resp.Pages {
			if idx >= 25 {
				break
			}

			excerpt := stripMarkdownAndNewlines(page.Body)
			if len(excerpt) > 97 {
				excerpt = excerpt[:97] + "..."
			}

			emoji := "📄"
			if len(page.Tags) > 0 {
				emoji = "🏷️"
			}

			options = append(options, discordgo.SelectMenuOption{
				Label:       truncateString(page.Title, 100),
				Value:       fmt.Sprintf("wiki_result:%s", page.Id),
				Description: excerpt,
				Emoji: &discordgo.ComponentEmoji{
					Name: emoji,
				},
			})
		}

		components := []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:    fmt.Sprintf("wiki_select:%s", query),
						Placeholder: fmt.Sprintf("Select from %d results...", len(resp.Pages)),
						Options:     options,
						MinValues:   intPtr(1),
						MaxValues:   1,
					},
				},
			},
		}

		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    fmt.Sprintf("🔍 Found **%d** wiki pages for: **%s**", resp.Total, query),
				Embeds:     []*discordgo.MessageEmbed{},
				Components: components,
				Flags:      discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			log.Error("failed to show search results", slog.String("error", err.Error()))
		}

	case "post":
		// Post to channel (remove ephemeral flag)
		if len(parts) < 2 {
			return
		}
		wikiID := parts[1]

		// Fetch the wiki page again
		ctx := discordContextFor(i)
		wikiClient := wikipb.NewWikiServiceClient(grpcClient.Conn())
		resp, err := wikiClient.SearchWikiPages(ctx, &wikipb.SearchWikiPagesRequest{
			GuildId: i.GuildID,
			Query:   "",
			Limit:   100,
		})
		if err != nil {
			log.Error("failed to fetch wiki page", slog.String("error", err.Error()))
			return
		}

		var page *wikipb.WikiPage
		for _, p := range resp.Pages {
			if p.Id == wikiID {
				page = p
				break
			}
		}

		if page == nil {
			return
		}

		// Create embed for posting (reuse the embed function, discard components)
		embed, _ := showWikiDetailEmbed(s, page, cfg, "", false)

		// Send as new message in channel
		_, err = s.ChannelMessageSendEmbed(i.ChannelID, embed)
		if err != nil {
			log.Error("failed to post wiki to channel", slog.String("error", err.Error()))
			return
		}

		// Update original ephemeral message
		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    "✅ Wiki page posted to channel!",
				Embeds:     []*discordgo.MessageEmbed{},
				Components: []discordgo.MessageComponent{},
				Flags:      discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			log.Error("failed to update message", slog.String("error", err.Error()))
		}

	case "edit":
		// Open edit modal
		if len(parts) < 3 {
			return
		}
		title := parts[2]
		handleWikiEditButton(s, i, title, cfg, log, grpcClient)
	}
}

// splitCustomID splits a custom ID by prefix and returns the parts after it
func splitCustomID(customID, prefix string) []string {
	if !strings.HasPrefix(customID, prefix) {
		return nil
	}
	remainder := customID[len(prefix):]
	return strings.Split(remainder, ":")
}
