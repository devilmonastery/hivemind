package handlers

import (
	"log"
	"net/http"

	quotespb "github.com/devilmonastery/hivemind/api/generated/go/quotespb"
)

// QuotesListPage displays recent quotes from guilds the user is in
func (h *Handler) QuotesListPage(w http.ResponseWriter, r *http.Request) {
	// Get authenticated client
	client, err := h.getClient(r, w)
	if err != nil {
		log.Printf("Failed to create client: %v", err)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	defer client.Close()

	// Fetch recent quotes (limit 25 for now)
	quoteClient := quotespb.NewQuoteServiceClient(client.Conn())
	resp, err := quoteClient.ListQuotes(r.Context(), &quotespb.ListQuotesRequest{
		Limit:     25,
		OrderBy:   "created_at",
		Ascending: false,
	})
	if err != nil {
		log.Printf("Failed to fetch quotes: %v", err)
		http.Error(w, "Failed to fetch quotes", http.StatusInternalServerError)
		return
	}

	// Prepare template data
	data := h.newTemplateData(r)
	data["Quotes"] = resp.Quotes
	data["Total"] = resp.Total

	h.renderTemplate(w, "quotes.html", data)
}
