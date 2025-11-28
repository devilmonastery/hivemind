package handlers

import (
	"context"
	"net/http"
	"time"
)

// Home handles the home page
func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	// Only handle root path
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Get current user if logged in
	user := h.getCurrentUser(r)

	// Prepare template data
	data := map[string]interface{}{
		"User":        user,
		"CurrentPage": "home",
	}

	// If not logged in, get available OAuth providers
	if user == nil {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		providers, err := h.getAvailableProviders(ctx)
		if err == nil {
			data["Providers"] = providers
		}
	}

	// Render the home page template
	h.renderTemplate(w, "home.html", data)
}
