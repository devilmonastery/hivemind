package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"

	wikipb "github.com/devilmonastery/hivemind/api/generated/go/wikipb"
	"github.com/devilmonastery/hivemind/bot/internal/config"
	"github.com/devilmonastery/hivemind/internal/client"
	"github.com/devilmonastery/hivemind/internal/pkg/urlutil"
)

// getWebBaseURL returns the web base URL from config or a default
func getWebBaseURL(cfg *config.Config) string {
	if cfg != nil && cfg.Backend.WebBaseURL != "" {
		return cfg.Backend.WebBaseURL
	}
	return "http://localhost:8080" // Default for development
}

// mustBuildWikiURL builds a wiki URL and returns a fallback on error (should never happen with valid baseURL)
func mustBuildWikiURL(baseURL, guildID, slug string) string {
	url, err := urlutil.BuildWikiViewURL(baseURL, guildID, slug)
	if err != nil {
		// Fallback to simple concatenation if URL parsing fails (should never happen)
		return baseURL + "/wiki?slug=" + slug + "&guild_id=" + guildID
	}
	return url
}

// fetchWikiMessageReferences fetches message references for a wiki page
func fetchWikiMessageReferences(ctx context.Context, wikiClient wikipb.WikiServiceClient, pageID string, log *slog.Logger) []*wikipb.WikiMessageReference {
	log.Info("fetching wiki message references",
		slog.String("page_id", pageID))

	resp, err := wikiClient.ListWikiMessageReferences(ctx, &wikipb.ListWikiMessageReferencesRequest{
		WikiPageId: pageID,
	})
	if err != nil {
		log.Warn("failed to fetch wiki message references",
			slog.String("page_id", pageID),
			slog.String("error", err.Error()))
		return nil
	}

	log.Info("fetched wiki message references",
		slog.String("page_id", pageID),
		slog.Int("count", len(resp.References)))

	return resp.References
}

// showWikiDetailEmbed creates the detailed embed and action buttons for a wiki page
func showWikiDetailEmbed(s *discordgo.Session, page *wikipb.WikiPage, references []*wikipb.WikiMessageReference, cfg *config.Config, query string, showBackButton bool) (*discordgo.MessageEmbed, []discordgo.MessageComponent) {
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

	// Add message references field if any exist
	if len(references) > 0 {
		// Build reference list with datetime and content preview
		refsList := ""
		displayCount := min(5, len(references)) // Show up to 5
		for idx := 0; idx < displayCount; idx++ {
			ref := references[idx]
			messageLink := urlutil.DiscordMessageURL(ref.GuildId, ref.ChannelId, ref.MessageId)

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

		slog.Default().Info("adding message references field to embed",
			slog.Int("ref_count", len(references)),
			slog.Int("displayed", displayCount))

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("📌 Referenced Messages (%d)", len(references)),
			Value:  refsList,
			Inline: false,
		})
	} else {
		slog.Default().Info("no message references to display for wiki page")
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
				URL:   mustBuildWikiURL(getWebBaseURL(cfg), page.GuildId, page.Slug),
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
	case "merge":
		handleWikiMerge(s, i, subcommand, cfg, log, grpcClient)
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
		// Fetch message references
		refs := fetchWikiMessageReferences(ctx, wikiClient, page.Id, log)
		log.Info("wiki search single result - displaying with references",
			slog.String("page_id", page.Id),
			slog.String("page_title", page.Title),
			slog.Int("ref_count", len(refs)))
		embed, components := showWikiDetailEmbed(s, page, refs, cfg, query, false)

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
	// Parse slug parameter (autocomplete returns slugs, manual input is normalized to slug)
	var slug string
	for _, opt := range subcommand.Options {
		if opt.Name == "title" {
			slug = opt.StringValue()
		}
	}

	if slug == "" {
		respondError(s, i, "Title is required", log)
		return
	}

	ctx := discordContextFor(i)

	// Lookup by slug (GetWikiPageByTitle normalizes input to slug for lookup)
	wikiClient := wikipb.NewWikiServiceClient(grpcClient.Conn())
	page, err := wikiClient.GetWikiPageByTitle(ctx, &wikipb.GetWikiPageByTitleRequest{
		GuildId: i.GuildID,
		Title:   slug, // Backend normalizes to slug for lookup
	})
	if err != nil {
		log.Error("failed to get wiki page",
			slog.String("error", err.Error()),
			slog.String("slug", slug))
		respondError(s, i, fmt.Sprintf("Wiki page not found: **%s**", slug), log)
		return
	}

	// Fetch message references
	refs := fetchWikiMessageReferences(ctx, wikiClient, page.Id, log)
	log.Info("wiki view - displaying with references",
		slog.String("page_id", page.Id),
		slog.String("page_title", page.Title),
		slog.Int("ref_count", len(refs)))

	// Use the standard embed function to include references
	embed, components := showWikiDetailEmbed(s, page, refs, cfg, "", false)

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
			Flags:      discordgo.MessageFlagsEphemeral,
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

	// Create modal with optional pre-filled content
	var modalCustomID string
	var modalTitle string
	var components []discordgo.MessageComponent

	if title != "" {
		// Editing existing page - no title field, just content
		modalCustomID = fmt.Sprintf("wiki_edit_modal:%s", title)
		modalTitle = fmt.Sprintf("Edit: %s", title)
	} else {
		// Creating new page - include title field
		modalCustomID = "wiki_edit_modal"
		modalTitle = "Create Wiki Page"
		components = append(components, discordgo.ActionsRow{
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
		})
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

	// Add body input to components
	components = append(components, discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			bodyInput,
		},
	})

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID:   modalCustomID,
			Title:      modalTitle,
			Components: components,
		},
	})
	if err != nil {
		log.Error("failed to show wiki edit modal", slog.String("error", err.Error()))
	}
}

func handleWikiMerge(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption, cfg *config.Config, log *slog.Logger, grpcClient *client.Client) {
	// Parse source and target slug parameters
	var sourceSlug, targetSlug string
	for _, opt := range subcommand.Options {
		switch opt.Name {
		case "source":
			sourceSlug = opt.StringValue()
		case "target":
			targetSlug = opt.StringValue()
		}
	}

	// Validate inputs
	if sourceSlug == "" || targetSlug == "" {
		respondError(s, i, "Both source and target pages are required", log)
		return
	}

	if sourceSlug == targetSlug {
		respondError(s, i, "Cannot merge a page into itself", log)
		return
	}

	// Defer the response since this might take a moment
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Error("failed to defer interaction", slog.String("error", err.Error()))
		return
	}

	ctx := discordContextFor(i)
	wikiClient := wikipb.NewWikiServiceClient(grpcClient.Conn())

	// Fetch source page
	sourceResp, err := wikiClient.GetWikiPageByTitle(ctx, &wikipb.GetWikiPageByTitleRequest{
		GuildId: i.GuildID,
		Title:   sourceSlug,
	})
	if err != nil {
		log.Error("failed to fetch source page",
			slog.String("slug", sourceSlug),
			slog.String("error", err.Error()))
		_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: ptrString(fmt.Sprintf("❌ Failed to find source page: %s", sourceSlug)),
		})
		return
	}

	// Fetch target page
	targetResp, err := wikiClient.GetWikiPageByTitle(ctx, &wikipb.GetWikiPageByTitleRequest{
		GuildId: i.GuildID,
		Title:   targetSlug,
	})
	if err != nil {
		log.Error("failed to fetch target page",
			slog.String("slug", targetSlug),
			slog.String("error", err.Error()))
		_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: ptrString(fmt.Sprintf("❌ Failed to find target page: %s", targetSlug)),
		})
		return
	}

	// Perform merge
	mergedPage, err := wikiClient.MergeWikiPages(ctx, &wikipb.MergeWikiPagesRequest{
		SourcePageId: sourceResp.Id,
		TargetPageId: targetResp.Id,
	})
	if err != nil {
		log.Error("failed to merge wiki pages",
			slog.String("source_id", sourceResp.Id),
			slog.String("target_id", targetResp.Id),
			slog.String("error", err.Error()))
		_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: ptrString("❌ Failed to merge wiki pages"),
		})
		return
	}

	// Fetch message references for the merged page
	refs := fetchWikiMessageReferences(ctx, wikiClient, mergedPage.Id, log)

	// Show standard wiki embed with success header
	embed, components := showWikiDetailEmbed(s, mergedPage, refs, cfg, "", false)
	embed.Title = fmt.Sprintf("✅ Successfully merged **%s** into **%s**\n\n%s",
		sourceResp.Title,
		mergedPage.Title,
		embed.Title)

	// Send success response with embed and buttons
	_, err = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds:     &[]*discordgo.MessageEmbed{embed},
		Components: &components,
	})
	if err != nil {
		log.Error("failed to send merge confirmation", slog.String("error", err.Error()))
	}
}

// handleWikiEditModal processes the modal submission for wiki page creation/editing
func handleWikiEditModal(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger, grpcClient *client.Client) {
	data := i.ModalSubmitData()

	// Check if this is an edit (custom ID format: wiki_edit_modal:OriginalTitle)
	var originalTitle string
	parts := strings.SplitN(data.CustomID, ":", 2)
	if len(parts) == 2 {
		// Editing existing page - use original title
		originalTitle = parts[1]
	}

	var title, body string
	for _, comp := range data.Components {
		if actionRow, ok := comp.(*discordgo.ActionsRow); ok {
			for _, innerComp := range actionRow.Components {
				if textInput, ok := innerComp.(*discordgo.TextInput); ok {
					switch textInput.CustomID {
					case "wiki_title":
						// Only use submitted title if creating (no originalTitle)
						if originalTitle == "" {
							title = textInput.Value
						}
					case "wiki_body":
						body = textInput.Value
					}
				}
			}
		}
	}

	// Use original title if editing
	if originalTitle != "" {
		title = originalTitle
	}

	// Validate title is not empty
	title = strings.TrimSpace(title)
	if title == "" {
		respondError(s, i, "Wiki page title cannot be empty", log)
		return
	}

	// Validate body is not empty
	body = strings.TrimSpace(body)
	if body == "" {
		respondError(s, i, "Wiki page body cannot be empty", log)
		return
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
			Flags:   discordgo.MessageFlagsEphemeral,
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

	// Show modal with only content field - title is fixed
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
			CustomID: fmt.Sprintf("wiki_edit_modal:%s", title),
			Title:    fmt.Sprintf("Edit: %s", title),
			Components: []discordgo.MessageComponent{
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

// handleWikiAddToChat posts the wiki page content to the current channel
func handleWikiAddToChat(s *discordgo.Session, i *discordgo.InteractionCreate, title string, cfg *config.Config, log *slog.Logger, grpcClient *client.Client) {
	// Fetch the wiki page
	ctx := discordContextFor(i)
	wikiClient := wikipb.NewWikiServiceClient(grpcClient.Conn())
	resp, err := wikiClient.SearchWikiPages(ctx, &wikipb.SearchWikiPagesRequest{
		GuildId: i.GuildID,
		Query:   title,
		Limit:   1,
	})
	if err != nil || len(resp.Pages) == 0 {
		respondError(s, i, "Failed to find wiki page", log)
		return
	}

	page := resp.Pages[0]

	// Format as embed with web link
	webURL := mustBuildWikiURL(getWebBaseURL(cfg), page.GuildId, page.Slug)
	embed := &discordgo.MessageEmbed{
		Title:       page.Title,
		URL:         webURL,
		Description: page.Body,
		Color:       0x5865F2, // Discord blurple
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Created by %s", page.AuthorUsername),
		},
		Timestamp: page.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z"),
	}

	if len(page.Tags) > 0 {
		tagsText := ""
		for idx, tag := range page.Tags {
			if idx > 0 {
				tagsText += ", "
			}
			tagsText += "#" + tag
		}
		embed.Fields = []*discordgo.MessageEmbedField{
			{
				Name:   "Tags",
				Value:  tagsText,
				Inline: false,
			},
		}
	}

	// Post to channel (non-ephemeral)
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
	if err != nil {
		log.Error("failed to post wiki to chat", slog.String("error", err.Error()))
	}
}

// handleWikiClose closes/dismisses the ephemeral wiki view message
func handleWikiClose(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    "✖️ Closed",
			Embeds:     []*discordgo.MessageEmbed{},
			Components: []discordgo.MessageComponent{},
		},
	})
	if err != nil {
		log.Error("failed to close wiki view", slog.String("error", err.Error()))
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

	// Fetch message references
	refs := fetchWikiMessageReferences(ctx, wikiClient, selectedPage.Id, log)

	// Create detailed embed and components
	embed, components := showWikiDetailEmbed(s, selectedPage, refs, cfg, query, true)

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

		// Fetch message references
		refs := fetchWikiMessageReferences(ctx, wikiClient, page.Id, log)

		// Create embed for posting (reuse the embed function, discard components)
		embed, _ := showWikiDetailEmbed(s, page, refs, cfg, "", false) // Send as new message in channel
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

// handleWikiUnifiedSelect handles the unified wiki select menu (create new or select existing)
func handleWikiUnifiedSelect(s *discordgo.Session, i *discordgo.InteractionCreate, messageID string, log *slog.Logger, grpcClient *client.Client) {
	// Get selected value
	selectedValues := i.MessageComponentData().Values
	if len(selectedValues) == 0 {
		respondError(s, i, "No selection made", log)
		return
	}
	selectedValue := selectedValues[0]

	log.Info("handleWikiUnifiedSelect called", "messageID", messageID, "selectedValue", selectedValue)

	// Fetch the original message
	message, err := s.ChannelMessage(i.ChannelID, messageID)
	if err != nil {
		log.Error("Failed to fetch message", "error", err, "messageID", messageID, "channelID", i.ChannelID)
		respondError(s, i, fmt.Sprintf("Failed to fetch message: %v", err), log)
		return
	}

	if selectedValue == "__create_new__" {
		log.Info("Showing create new modal", "messageID", messageID, "messageContent", message.Content)
		// User wants to create new page - show modal with message content
		modalErr := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseModal,
			Data: &discordgo.InteractionResponseData{
				CustomID: fmt.Sprintf("context_wiki_unified_modal:%s:__create_new__", messageID),
				Title:    "Create Wiki Page",
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
		if modalErr != nil {
			log.Error("Failed to show wiki creation modal", "error", modalErr, "messageID", messageID)
			// Can't call respondError here as we already tried to respond with modal
		}
		return
	}

	log.Info("Fetching existing wiki page", "pageID", selectedValue)
	// User selected an existing page - fetch it to show in modal with checkbox
	wikiClient := wikipb.NewWikiServiceClient(grpcClient.Conn())
	page, err := wikiClient.GetWikiPage(discordContextFor(i), &wikipb.GetWikiPageRequest{
		Id: selectedValue,
	})
	if err != nil {
		log.Error("Failed to fetch wiki page", "error", err, "pageID", selectedValue)
		respondError(s, i, fmt.Sprintf("Failed to fetch wiki page: %v", err), log)
		return
	}

	log.Info("Showing update modal for existing page", "pageID", page.Id, "pageTitle", page.Title)
	// Show modal with existing page content and checkbox for optional editing
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: fmt.Sprintf("context_wiki_unified_modal:%s:%s", messageID, page.Id),
			Title:    fmt.Sprintf("Add to: %s", truncateString(page.Title, 30)),
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "wiki_update_content",
							Label:       "Update page content? (leave empty to skip)",
							Style:       discordgo.TextInputParagraph,
							Required:    false,
							Value:       page.Body,
							MaxLength:   4000,
							Placeholder: "Edit to update page, or leave as-is to only add reference",
						},
					},
				},
			},
		},
	})
	if err != nil {
		log.Error("Failed to show wiki update modal", "error", err, "pageID", page.Id)
		// Can't call respondError here as we already tried to respond with modal
	}
}

// handleWikiAutocomplete handles autocomplete for wiki commands
func handleWikiAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger, grpcClient *client.Client, cache *TitlesCache) {
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

	// Handle title, source, and target autocomplete (all use wiki titles)
	if focusedOption.Name != "title" && focusedOption.Name != "source" && focusedOption.Name != "target" {
		return
	}

	query := focusedOption.StringValue()

	// Check local cache first
	cachedTitles := cache.GetWikiTitles(i.GuildID)

	// If cache miss, fetch from server and populate cache
	if cachedTitles == nil {
		wikiClient := wikipb.NewWikiServiceClient(grpcClient.Conn())
		ctx := discordContextFor(i)

		autocompleteResp, err := wikiClient.AutocompleteWikiTitles(ctx, &wikipb.AutocompleteWikiTitlesRequest{
			GuildId: i.GuildID,
		})
		if err != nil {
			log.Error("Failed to fetch wiki titles for cache", "error", err)
			return
		}

		// Convert to cache format
		cachedTitles = make([]TitleSuggestion, len(autocompleteResp.Suggestions))
		for idx, suggestion := range autocompleteResp.Suggestions {
			cachedTitles[idx] = TitleSuggestion{
				ID:    suggestion.Id,
				Title: suggestion.Title,
				Slug:  suggestion.Slug,
			}
		}

		// Store in cache
		cache.SetWikiTitles(i.GuildID, cachedTitles)
	}

	// Filter titles locally
	filtered := FilterTitles(cachedTitles, query, 25)

	// Build autocomplete choices
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(filtered))
	for _, title := range filtered {
		// Limit title length for display
		displayTitle := title.Title
		if len(displayTitle) > 100 {
			displayTitle = displayTitle[:97] + "..."
		}

		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  displayTitle,
			Value: title.Slug, // Return the slug for uniqueness
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
