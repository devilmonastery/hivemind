package handlers

import (
	"log/slog"
	"net/http"

	quotespb "github.com/devilmonastery/hivemind/api/generated/go/quotespb"
)

// QuotesListPage displays recent quotes from guilds the user is in
func (h *Handler) QuotesListPage(w http.ResponseWriter, r *http.Request) {
	// Get authenticated client
	client, err := h.getClient(r, w)
	if err != nil {
		h.log.Error("failed to create client", slog.String("error", err.Error()))
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
		h.log.Error("failed to fetch quotes", slog.String("error", err.Error()))
		http.Error(w, "Failed to fetch quotes", http.StatusInternalServerError)
		return
	}

	for _, quote := range resp.Quotes {
		h.log.Debug("quote", slog.Any("data", quote))
	}

	// Prepare template data
	data := h.newTemplateData(r)
	data["Quotes"] = resp.Quotes
	data["Total"] = resp.Total

	h.renderTemplate(w, "quotes.html", data)
}
