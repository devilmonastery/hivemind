package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"time"

	notespb "github.com/devilmonastery/hivemind/api/generated/go/notespb"
	quotespb "github.com/devilmonastery/hivemind/api/generated/go/quotespb"
	wikipb "github.com/devilmonastery/hivemind/api/generated/go/wikipb"
)

// ActivityItem represents a unified activity item for the home feed
type ActivityItem struct {
	Type           string // "note", "quote", "wiki"
	ID             string
	Title          string
	Slug           string // URL-friendly slug (for wiki pages)
	Body           string
	Preview        string // First 200 chars of body
	GuildID        string
	GuildName      string
	ChannelName    string
	Timestamp      time.Time
	Author         string
	Tags           []string
	ReferenceCount int32 // Number of message references (for wiki pages)
}

// Home handles the home page
func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	// Only handle root path
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Prepare template data with standard fields
	data := h.newTemplateData(r)
	data["CurrentPage"] = "home"

	// If not logged in, get available OAuth providers
	// Check both nil and empty map (in case of typed nil)
	user, hasUser := data["User"].(map[string]interface{})
	if !hasUser || user == nil || len(user) == 0 {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		providers, err := h.getAvailableProviders(ctx)
		if err == nil {
			data["Providers"] = providers
		}
	} else {
		// Fetch recent activity for logged-in users
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		activity, err := h.fetchRecentActivity(ctx, r, w)
		if err != nil {
			// Check if it's an auth error (token not found, session expired, etc.)
			if isAuthError(err) {
				h.clearSessionAndRedirect(w, r)
				return
			}
			h.log.Error("failed to fetch recent activity",
				slog.String("error", err.Error()))
			// Continue with empty activity rather than error
			activity = []ActivityItem{}
		}
		data["RecentActivity"] = activity
	}

	// Render the home page template
	h.renderTemplate(w, "home.html", data)
}

// fetchRecentActivity fetches recent notes, quotes, and wiki pages
func (h *Handler) fetchRecentActivity(ctx context.Context, r *http.Request, w http.ResponseWriter) ([]ActivityItem, error) {
	client, err := h.getClient(r, w)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var activity []ActivityItem
	limit := int32(10) // Fetch 10 of each type

	// Fetch recent notes
	noteClient := notespb.NewNoteServiceClient(client.Conn())
	notesResp, err := noteClient.ListNotes(ctx, &notespb.ListNotesRequest{
		Limit:     limit,
		OrderBy:   "created_at",
		Ascending: false,
	})
	if err != nil {
		if isAuthError(err) {
			return nil, err // Return auth error to trigger session clear
		}
		h.log.Error("failed to fetch notes",
			slog.String("error", err.Error()))
	} else {
		h.log.Debug("fetched notes from server",
			slog.Int("count", len(notesResp.Notes)))
		for _, note := range notesResp.Notes {
			activity = append(activity, ActivityItem{
				Type:        "note",
				ID:          note.Id,
				Title:       note.Title,
				Body:        note.Body,
				Preview:     truncateText(note.Body, 200),
				GuildID:     note.GuildId,
				GuildName:   note.GuildName,
				ChannelName: note.ChannelName,
				Timestamp:   note.CreatedAt.AsTime(),
				Author:      note.AuthorUsername,
				Tags:        note.Tags,
			})
		}
	}

	// Fetch recent quotes (get from all guilds user has access to)
	quoteClient := quotespb.NewQuoteServiceClient(client.Conn())
	h.log.Debug("fetching quotes")
	quotesResp, err := quoteClient.ListQuotes(ctx, &quotespb.ListQuotesRequest{
		GuildId:   "", // Empty = all guilds
		Limit:     limit,
		OrderBy:   "created_at",
		Ascending: false,
	})
	if err != nil {
		if isAuthError(err) {
			return nil, err // Return auth error to trigger session clear
		}
		h.log.Error("failed to fetch quotes",
			slog.String("error", err.Error()))
	} else {
		h.log.Debug("fetched quotes from server",
			slog.Int("count", len(quotesResp.Quotes)))
		for _, quote := range quotesResp.Quotes {
			// Prefer guild nickname over username
			author := quote.SourceMsgAuthorUsername
			if quote.SourceMsgAuthorGuildNick != "" {
				author = quote.SourceMsgAuthorGuildNick
			}
			activity = append(activity, ActivityItem{
				Type:        "quote",
				ID:          quote.Id,
				Body:        quote.Body,
				Preview:     truncateText(quote.Body, 200),
				GuildID:     quote.GuildId,
				GuildName:   quote.GuildName,
				ChannelName: quote.SourceChannelName,
				Timestamp:   quote.CreatedAt.AsTime(),
				Author:      author,
				Tags:        quote.Tags,
			})
		}
	}

	// Fetch recent wiki pages (get from all guilds)
	wikiClient := wikipb.NewWikiServiceClient(client.Conn())
	h.log.Debug("fetching wiki pages")
	wikiResp, err := wikiClient.ListWikiPages(ctx, &wikipb.ListWikiPagesRequest{
		GuildId:   "", // Empty = all guilds
		Limit:     limit,
		OrderBy:   "created_at",
		Ascending: false,
	})
	if err != nil {
		if isAuthError(err) {
			return nil, err // Return auth error to trigger session clear
		}
		h.log.Error("failed to fetch wiki pages",
			slog.String("error", err.Error()))
	} else {
		h.log.Debug("fetched wiki pages from server",
			slog.Int("count", len(wikiResp.Pages)))
		for _, page := range wikiResp.Pages {
			// Fetch reference count for this page
			refCount := int32(0)
			refsResp, refErr := wikiClient.ListWikiMessageReferences(ctx, &wikipb.ListWikiMessageReferencesRequest{
				WikiPageId: page.Id,
			})
			if refErr == nil && refsResp != nil {
				refCount = int32(len(refsResp.References))
			}

			activity = append(activity, ActivityItem{
				Type:           "wiki",
				ID:             page.Id,
				Title:          page.Title,
				Slug:           page.Slug,
				Body:           page.Body,
				Preview:        truncateText(page.Body, 200),
				GuildID:        page.GuildId,
				GuildName:      page.GuildName,
				ChannelName:    page.ChannelName,
				Timestamp:      page.CreatedAt.AsTime(),
				Author:         page.AuthorUsername,
				Tags:           page.Tags,
				ReferenceCount: refCount,
			})
		}
	}

	// Sort by timestamp descending (most recent first)
	sort.Slice(activity, func(i, j int) bool {
		return activity[i].Timestamp.After(activity[j].Timestamp)
	})

	// Limit to 20 total items
	if len(activity) > 20 {
		activity = activity[:20]
	}

	return activity, nil
}

// truncateText truncates text to maxLen characters, adding "..." if truncated
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}
