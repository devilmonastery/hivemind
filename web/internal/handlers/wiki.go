package handlers

import (
	"log"
	"net/http"

	wikipb "github.com/devilmonastery/hivemind/api/generated/go/wikipb"
)

// WikiPage displays a single wiki page with its references
func (h *Handler) WikiPage(w http.ResponseWriter, r *http.Request) {
	// Get wiki page title and guild_id from query params
	title := r.URL.Query().Get("title")
	guildID := r.URL.Query().Get("guild_id")

	if title == "" || guildID == "" {
		http.Error(w, "Missing wiki page title or guild_id", http.StatusBadRequest)
		return
	}

	// Get authenticated client
	client, err := h.getClient(r, w)
	if err != nil {
		log.Printf("Failed to create client: %v", err)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	defer client.Close()

	// Search for wiki page by exact title match in the guild
	wikiClient := wikipb.NewWikiServiceClient(client.Conn())
	searchResp, err := wikiClient.SearchWikiPages(r.Context(), &wikipb.SearchWikiPagesRequest{
		GuildId: guildID,
		Query:   title,
		Limit:   1,
	})
	if err != nil || len(searchResp.Pages) == 0 {
		log.Printf("Failed to find wiki page: %v", err)
		http.Error(w, "Wiki page not found", http.StatusNotFound)
		return
	}

	page := searchResp.Pages[0]

	// Verify exact title match (search might return partial matches)
	if page.Title != title {
		http.Error(w, "Wiki page not found", http.StatusNotFound)
		return
	}

	// Fetch message references
	refsResp, err := wikiClient.ListWikiMessageReferences(r.Context(), &wikipb.ListWikiMessageReferencesRequest{
		WikiPageId: page.Id,
	})
	if err != nil {
		log.Printf("Failed to fetch wiki references: %v", err)
		// Continue without references rather than failing completely
	}

	// Prepare template data with page-specific fields
	data := h.newTemplateData(r)
	data["Page"] = page
	data["References"] = refsResp.GetReferences()

	h.renderTemplate(w, "wiki_page.html", data)
}
