package handlers

import (
	"log/slog"
	"net/http"

	wikipb "github.com/devilmonastery/hivemind/api/generated/go/wikipb"
)

// WikiListPage displays recent wiki pages from guilds the user is in
func (h *Handler) WikiListPage(w http.ResponseWriter, r *http.Request) {
	// Get authenticated client
	client, err := h.getClient(r, w)
	if err != nil {
		h.log.Error("Failed to create client for wiki list",
			slog.String("error", err.Error()))
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	defer client.Close()

	// Fetch recent wiki pages (limit 25 for now)
	wikiClient := wikipb.NewWikiServiceClient(client.Conn())
	resp, err := wikiClient.ListWikiPages(r.Context(), &wikipb.ListWikiPagesRequest{
		Limit:     25,
		OrderBy:   "updated_at",
		Ascending: false,
	})
	if err != nil {
		h.log.Error("Failed to fetch wiki pages",
			slog.String("error", err.Error()))
		http.Error(w, "Failed to fetch wiki pages", http.StatusInternalServerError)
		return
	}

	// Prepare template data
	data := h.newTemplateData(r)
	data["Pages"] = resp.Pages
	data["Total"] = resp.Total

	h.renderTemplate(w, "wiki-list.html", data)
}

// WikiPage displays a single wiki page with its references
func (h *Handler) WikiPage(w http.ResponseWriter, r *http.Request) {
	// Get wiki page slug and guild_id from query params
	slug := r.URL.Query().Get("slug")
	guildID := r.URL.Query().Get("guild_id")

	if slug == "" || guildID == "" {
		http.Error(w, "Missing wiki page slug or guild_id", http.StatusBadRequest)
		return
	}

	// Get authenticated client
	client, err := h.getClient(r, w)
	if err != nil {
		h.log.Error("Failed to create client for wiki page",
			slog.String("slug", slug),
			slog.String("guild_id", guildID),
			slog.String("error", err.Error()))
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	defer client.Close()

	// Get wiki page by slug (normalized for lookup)
	wikiClient := wikipb.NewWikiServiceClient(client.Conn())
	page, err := wikiClient.GetWikiPageByTitle(r.Context(), &wikipb.GetWikiPageByTitleRequest{
		GuildId: guildID,
		Title:   slug,
	})
	if err != nil {
		h.log.Error("Failed to find wiki page",
			slog.String("slug", slug),
			slog.String("guild_id", guildID),
			slog.String("error", err.Error()))
		http.Error(w, "Wiki page not found", http.StatusNotFound)
		return
	}

	// Fetch message references
	refsResp, err := wikiClient.ListWikiMessageReferences(r.Context(), &wikipb.ListWikiMessageReferencesRequest{
		WikiPageId: page.Id,
	})
	if err != nil {
		h.log.Error("Failed to fetch wiki references",
			slog.String("wiki_page_id", page.Id),
			slog.String("error", err.Error()))
		// Continue without references rather than failing completely
	}

	// Prepare template data with page-specific fields
	data := h.newTemplateData(r)
	data["Page"] = page
	data["References"] = refsResp.GetReferences()

	h.renderTemplate(w, "wiki_page.html", data)
}
