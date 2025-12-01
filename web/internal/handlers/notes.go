package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	notespb "github.com/devilmonastery/hivemind/api/generated/go/notespb"
	"github.com/devilmonastery/hivemind/internal/pkg/textutil"
	"github.com/devilmonastery/hivemind/web/internal/render"
)

// NotesListPage displays a list of user's notes
func (h *Handler) NotesListPage(w http.ResponseWriter, r *http.Request) {
	// Get authenticated client
	client, err := h.getClient(r, w)
	if err != nil {
		h.log.Error("Failed to create client for notes list",
			slog.String("error", err.Error()))
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	defer client.Close()

	// Get filter params
	guildID := r.URL.Query().Get("guild_id")
	tagsParam := r.URL.Query().Get("tags")
	var tags []string
	if tagsParam != "" {
		for _, tag := range strings.Split(tagsParam, ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}

	// Fetch notes
	noteClient := notespb.NewNoteServiceClient(client.Conn())
	resp, err := noteClient.ListNotes(r.Context(), &notespb.ListNotesRequest{
		GuildId:   guildID,
		Tags:      tags,
		Limit:     50,
		OrderBy:   "updated_at",
		Ascending: false,
	})
	if err != nil {
		h.log.Error("Failed to fetch notes",
			slog.String("guild_id", guildID),
			slog.Any("tags", tags),
			slog.String("error", err.Error()))
		http.Error(w, "Failed to fetch notes", http.StatusInternalServerError)
		return
	}

	// Prepare template data with notes-specific fields
	data := h.newTemplateData(r)
	data["Notes"] = resp.Notes
	data["Total"] = resp.Total
	data["GuildID"] = guildID
	data["Tags"] = tags

	h.renderTemplate(w, "notes.html", data)
}

// NotePage displays a single note with its message references
func (h *Handler) NotePage(w http.ResponseWriter, r *http.Request) {
	// Get note ID from query params
	noteID := r.URL.Query().Get("id")
	if noteID == "" {
		http.Error(w, "Missing note ID", http.StatusBadRequest)
		return
	}

	// Get authenticated client
	client, err := h.getClient(r, w)
	if err != nil {
		h.log.Error("Failed to create client for note page",
			slog.String("note_id", noteID),
			slog.String("error", err.Error()))
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	defer client.Close()

	// Fetch note
	noteClient := notespb.NewNoteServiceClient(client.Conn())
	note, err := noteClient.GetNote(r.Context(), &notespb.GetNoteRequest{
		Id: noteID,
	})
	if err != nil {
		h.log.Error("Failed to fetch note",
			slog.String("note_id", noteID),
			slog.String("error", err.Error()))
		http.Error(w, "Note not found", http.StatusNotFound)
		return
	}

	// Fetch message references
	refsResp, err := noteClient.ListNoteMessageReferences(r.Context(), &notespb.ListNoteMessageReferencesRequest{
		NoteId: noteID,
	})
	if err != nil {
		h.log.Error("Failed to fetch note references",
			slog.String("note_id", noteID),
			slog.String("error", err.Error()))
		// Continue without references rather than failing completely
	}

	// Prepare template data with note-specific fields
	data := h.newTemplateData(r)
	data["Note"] = note
	data["References"] = refsResp.GetReferences()

	// Check if this is an HTMX request (e.g., from Cancel button)
	if r.Header.Get("HX-Request") == "true" {
		h.renderContentOnly(w, "note_view.html", data)
	} else {
		h.renderTemplate(w, "note.html", data)
	}
}

// NoteEdit displays the editor for an existing note
func (h *Handler) NoteEdit(w http.ResponseWriter, r *http.Request) {
	// Get note ID from query params
	noteID := r.URL.Query().Get("id")
	if noteID == "" {
		http.Error(w, "Missing note ID", http.StatusBadRequest)
		return
	}

	// Get authenticated client
	client, err := h.getClient(r, w)
	if err != nil {
		h.log.Error("Failed to create client for note edit",
			slog.String("note_id", noteID),
			slog.String("error", err.Error()))
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	defer client.Close()

	// Fetch note
	noteClient := notespb.NewNoteServiceClient(client.Conn())
	note, err := noteClient.GetNote(r.Context(), &notespb.GetNoteRequest{
		Id: noteID,
	})
	if err != nil {
		h.log.Error("Failed to fetch note for editing",
			slog.String("note_id", noteID),
			slog.String("error", err.Error()))
		http.Error(w, "Note not found", http.StatusNotFound)
		return
	}

	// Prepare template data
	data := h.newTemplateData(r)
	data["Note"] = note

	h.renderContentOnly(w, "note_editor.html", data)
}

// NotePreview renders markdown preview for the note editor
func (h *Handler) NotePreview(w http.ResponseWriter, r *http.Request) {
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

// NoteSave updates a note with new content
func (h *Handler) NoteSave(w http.ResponseWriter, r *http.Request) {
	// Get note ID from query params
	noteID := r.URL.Query().Get("id")
	if noteID == "" {
		http.Error(w, "Missing note ID", http.StatusBadRequest)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		h.log.Error("Failed to parse save form",
			slog.String("error", err.Error()))
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	body := r.FormValue("body")
	if body == "" {
		http.Error(w, "Note body cannot be empty", http.StatusBadRequest)
		return
	}

	// Get authenticated client
	client, err := h.getClient(r, w)
	if err != nil {
		h.log.Error("Failed to create client for note save",
			slog.String("note_id", noteID),
			slog.String("error", err.Error()))
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	defer client.Close()

	// Extract hashtags from body
	tags := textutil.ExtractHashtags(body)

	// Update note
	noteClient := notespb.NewNoteServiceClient(client.Conn())
	note, err := noteClient.UpdateNote(r.Context(), &notespb.UpdateNoteRequest{
		Id:    noteID,
		Title: title,
		Body:  body,
		Tags:  tags,
	})
	if err != nil {
		h.log.Error("Failed to update note",
			slog.String("note_id", noteID),
			slog.String("error", err.Error()))
		http.Error(w, "Failed to update note", http.StatusInternalServerError)
		return
	}

	h.log.Info("Note updated",
		slog.String("note_id", noteID),
		slog.Int("tags", len(tags)))

	// Fetch message references for the updated note
	refsResp, err := noteClient.ListNoteMessageReferences(r.Context(), &notespb.ListNoteMessageReferencesRequest{
		NoteId: noteID,
	})
	if err != nil {
		h.log.Error("Failed to fetch note references after update",
			slog.String("note_id", noteID),
			slog.String("error", err.Error()))
		// Continue without references
	}

	// Return the updated note view
	data := h.newTemplateData(r)
	data["Note"] = note
	data["References"] = refsResp.GetReferences()

	h.renderContentOnly(w, "note_view.html", data)
}
