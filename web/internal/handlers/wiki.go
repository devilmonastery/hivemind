package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/gosimple/slug"

	wikipb "github.com/devilmonastery/hivemind/api/generated/go/wikipb"
	"github.com/devilmonastery/hivemind/internal/pkg/textutil"
	"github.com/devilmonastery/hivemind/web/internal/render"
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
	slugParam := r.URL.Query().Get("slug")
	guildID := r.URL.Query().Get("guild_id")

	if slugParam == "" || guildID == "" {
		http.Error(w, "Missing wiki page slug or guild_id", http.StatusBadRequest)
		return
	}

	// Get authenticated client
	client, err := h.getClient(r, w)
	if err != nil {
		h.log.Error("Failed to create client for wiki page",
			slog.String("slug", slugParam),
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
		Title:   slugParam,
	})
	if err != nil {
		h.log.Error("Failed to find wiki page",
			slog.String("slug", slugParam),
			slog.String("guild_id", guildID),
			slog.String("error", err.Error()))
		http.Error(w, "Wiki page not found", http.StatusNotFound)
		return
	}

	// If accessed slug differs from canonical slug, redirect to canonical URL
	// This happens when accessing an alias (e.g., old page name after merge)
	normalizedSlug := slug.Make(slugParam)

	h.log.Debug("checking for alias redirect",
		slog.String("accessed_slug", slugParam),
		slog.String("normalized_slug", normalizedSlug),
		slog.String("page_slug", page.Slug),
		slog.Bool("should_redirect", normalizedSlug != page.Slug))

	if normalizedSlug != page.Slug {
		// Build canonical URL with the page's canonical slug
		canonicalURL := fmt.Sprintf("/wiki?slug=%s&guild_id=%s",
			url.QueryEscape(page.Slug),
			url.QueryEscape(guildID))
		h.log.Info("redirecting to canonical slug",
			slog.String("from", slugParam),
			slog.String("to", page.Slug),
			slog.String("url", canonicalURL))
		http.Redirect(w, r, canonicalURL, http.StatusMovedPermanently)
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

// WikiEdit displays the editor for an existing wiki page
func (h *Handler) WikiEdit(w http.ResponseWriter, r *http.Request) {
	// Get wiki page slug and guild_id from query params
	slugParam := r.URL.Query().Get("slug")
	guildID := r.URL.Query().Get("guild_id")

	if slugParam == "" || guildID == "" {
		http.Error(w, "Missing wiki page slug or guild_id", http.StatusBadRequest)
		return
	}

	// Get authenticated client
	client, err := h.getClient(r, w)
	if err != nil {
		h.log.Error("Failed to create client for wiki edit",
			slog.String("slug", slugParam),
			slog.String("guild_id", guildID),
			slog.String("error", err.Error()))
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	defer client.Close()

	// Get wiki page by slug
	wikiClient := wikipb.NewWikiServiceClient(client.Conn())
	page, err := wikiClient.GetWikiPageByTitle(r.Context(), &wikipb.GetWikiPageByTitleRequest{
		GuildId: guildID,
		Title:   slugParam,
	})
	if err != nil {
		h.log.Error("Failed to find wiki page for editing",
			slog.String("slug", slugParam),
			slog.String("guild_id", guildID),
			slog.String("error", err.Error()))
		http.Error(w, "Wiki page not found", http.StatusNotFound)
		return
	}

	// Prepare template data
	data := h.newTemplateData(r)
	data["Page"] = page

	h.renderTemplate(w, "wiki_editor.html", data)
}

// WikiPreview renders markdown preview for the editor
func (h *Handler) WikiPreview(w http.ResponseWriter, r *http.Request) {
	// Parse form data
	if err := r.ParseForm(); err != nil {
		h.log.Error("Failed to parse preview form",
			slog.String("error", err.Error()))
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	body := r.FormValue("body")
	if body == "" {
		w.Write([]byte(`<div class="text-gray-500 text-center py-8">No content to preview</div>`))
		return
	}

	// Render markdown
	html := render.Markdown(body)

	// Wrap in prose styling
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<div class="prose prose-invert prose-cyan max-w-none">%s</div>`, html)
}

// WikiSave updates a wiki page with new content
func (h *Handler) WikiSave(w http.ResponseWriter, r *http.Request) {
	// Get wiki page slug and guild_id from query params
	slugParam := r.URL.Query().Get("slug")
	guildID := r.URL.Query().Get("guild_id")

	if slugParam == "" || guildID == "" {
		http.Error(w, "Missing wiki page slug or guild_id", http.StatusBadRequest)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		h.log.Error("Failed to parse save form",
			slog.String("error", err.Error()))
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	body := r.FormValue("body")
	if body == "" {
		http.Error(w, "Wiki body cannot be empty", http.StatusBadRequest)
		return
	}

	// Get authenticated client
	client, err := h.getClient(r, w)
	if err != nil {
		h.log.Error("Failed to create client for wiki save",
			slog.String("slug", slugParam),
			slog.String("guild_id", guildID),
			slog.String("error", err.Error()))
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	defer client.Close()

	wikiClient := wikipb.NewWikiServiceClient(client.Conn())

	// First, get the existing page to retrieve its ID
	existingPage, err := wikiClient.GetWikiPageByTitle(r.Context(), &wikipb.GetWikiPageByTitleRequest{
		GuildId: guildID,
		Title:   slugParam,
	})
	if err != nil {
		h.log.Error("Failed to find wiki page for update",
			slog.String("slug", slugParam),
			slog.String("guild_id", guildID),
			slog.String("error", err.Error()))
		http.Error(w, "Wiki page not found", http.StatusNotFound)
		return
	}

	// Extract hashtags from body
	tags := textutil.ExtractHashtags(body)

	// Update wiki page
	page, err := wikiClient.UpdateWikiPage(r.Context(), &wikipb.UpdateWikiPageRequest{
		Id:    existingPage.Id,
		Title: existingPage.Title, // Keep the same title
		Body:  body,
		Tags:  tags,
	})
	if err != nil {
		h.log.Error("Failed to update wiki page",
			slog.String("slug", slugParam),
			slog.String("guild_id", guildID),
			slog.String("error", err.Error()))
		http.Error(w, "Failed to update wiki page", http.StatusInternalServerError)
		return
	}

	h.log.Info("Wiki page updated",
		slog.String("slug", slugParam),
		slog.String("guild_id", guildID),
		slog.Int("tags", len(tags)))

	// Fetch message references for the updated page
	refsResp, err := wikiClient.ListWikiMessageReferences(r.Context(), &wikipb.ListWikiMessageReferencesRequest{
		WikiPageId: page.Id,
	})
	if err != nil {
		h.log.Error("Failed to fetch wiki references after update",
			slog.String("wiki_page_id", page.Id),
			slog.String("error", err.Error()))
		// Continue without references
	}

	// Return the updated page view
	data := h.newTemplateData(r)
	data["Page"] = page
	data["References"] = refsResp.GetReferences()

	h.renderTemplate(w, "wiki_page.html", data)
}
