package handlers

import (
	"fmt"
	"log/slog"
	"net/http"

	quotespb "github.com/devilmonastery/hivemind/api/generated/go/quotespb"
	"github.com/devilmonastery/hivemind/internal/pkg/textutil"
	"github.com/devilmonastery/hivemind/web/internal/render"
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

// QuotePage displays a single quote
func (h *Handler) QuotePage(w http.ResponseWriter, r *http.Request) {
	// Get quote ID from query params
	quoteID := r.URL.Query().Get("id")
	if quoteID == "" {
		http.Error(w, "Missing quote ID", http.StatusBadRequest)
		return
	}

	// Get authenticated client
	client, err := h.getClient(r, w)
	if err != nil {
		h.log.Error("Failed to create client for quote page",
			slog.String("quote_id", quoteID),
			slog.String("error", err.Error()))
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	defer client.Close()

	// Fetch quote
	quoteClient := quotespb.NewQuoteServiceClient(client.Conn())
	quote, err := quoteClient.GetQuote(r.Context(), &quotespb.GetQuoteRequest{
		Id: quoteID,
	})
	if err != nil {
		h.log.Error("Failed to find quote",
			slog.String("quote_id", quoteID),
			slog.String("error", err.Error()))
		h.renderError(w, r, ErrorPageOptions{
			StatusCode:        http.StatusNotFound,
			ErrorTitle:        "Quote Not Found",
			ErrorMessage:      "The quote you're looking for could not be found.",
			ErrorDetails:      "It may have been deleted or you might not have access to it.",
			SuggestedLink:     "/quotes",
			SuggestedLinkText: "💬 View All Quotes",
		})
		return
	}

	// Prepare template data
	data := h.newTemplateData(r)
	data["Quote"] = quote

	// Check if this is an HTMX request (e.g., from Cancel button)
	if r.Header.Get("HX-Request") == "true" {
		h.renderContentOnly(w, "quote_view.html", data)
	} else {
		h.renderTemplate(w, "quote.html", data)
	}
}

// QuoteEdit displays the editor for an existing quote
func (h *Handler) QuoteEdit(w http.ResponseWriter, r *http.Request) {
	// Get quote ID from query params
	quoteID := r.URL.Query().Get("id")
	if quoteID == "" {
		http.Error(w, "Missing quote ID", http.StatusBadRequest)
		return
	}

	// Get authenticated client
	client, err := h.getClient(r, w)
	if err != nil {
		h.log.Error("Failed to create client for quote edit",
			slog.String("quote_id", quoteID),
			slog.String("error", err.Error()))
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	defer client.Close()

	// Fetch quote
	quoteClient := quotespb.NewQuoteServiceClient(client.Conn())
	quote, err := quoteClient.GetQuote(r.Context(), &quotespb.GetQuoteRequest{
		Id: quoteID,
	})
	if err != nil {
		h.log.Error("Failed to fetch quote for editing",
			slog.String("quote_id", quoteID),
			slog.String("error", err.Error()))
		http.Error(w, "Quote not found", http.StatusNotFound)
		return
	}

	// Prepare template data
	data := h.newTemplateData(r)
	data["Quote"] = quote

	h.renderContentOnly(w, "quote_editor.html", data)
}

// QuotePreview renders markdown preview for the quote editor
func (h *Handler) QuotePreview(w http.ResponseWriter, r *http.Request) {
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

// QuoteSave updates a quote with new content
func (h *Handler) QuoteSave(w http.ResponseWriter, r *http.Request) {
	// Get quote ID from query params
	quoteID := r.URL.Query().Get("id")
	if quoteID == "" {
		http.Error(w, "Missing quote ID", http.StatusBadRequest)
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
		http.Error(w, "Quote body cannot be empty", http.StatusBadRequest)
		return
	}

	// Get authenticated client
	client, err := h.getClient(r, w)
	if err != nil {
		h.log.Error("Failed to create client for quote save",
			slog.String("quote_id", quoteID),
			slog.String("error", err.Error()))
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	defer client.Close()

	// Extract hashtags from body
	tags := textutil.ExtractHashtags(body)

	// Update quote
	quoteClient := quotespb.NewQuoteServiceClient(client.Conn())
	_, err = quoteClient.UpdateQuote(r.Context(), &quotespb.UpdateQuoteRequest{
		Id:   quoteID,
		Body: body,
		Tags: tags,
	})
	if err != nil {
		h.log.Error("Failed to update quote",
			slog.String("quote_id", quoteID),
			slog.String("error", err.Error()))
		http.Error(w, "Failed to update quote", http.StatusInternalServerError)
		return
	}

	h.log.Info("Quote updated",
		slog.String("quote_id", quoteID),
		slog.Int("tags", len(tags)))

	// Fetch the updated quote and return the quote page
	quote, err := quoteClient.GetQuote(r.Context(), &quotespb.GetQuoteRequest{
		Id: quoteID,
	})
	if err != nil {
		h.log.Error("Failed to fetch quote after update",
			slog.String("quote_id", quoteID),
			slog.String("error", err.Error()))
		http.Error(w, "Failed to fetch quote", http.StatusInternalServerError)
		return
	}

	// Return the updated quote page
	data := h.newTemplateData(r)
	data["Quote"] = quote

	h.renderContentOnly(w, "quote_view.html", data)
}
