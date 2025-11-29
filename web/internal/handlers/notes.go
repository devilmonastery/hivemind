package handlers

import (
	"log"
	"net/http"
	"strings"

	notespb "github.com/devilmonastery/hivemind/api/generated/go/notespb"
)

// NotesListPage displays a list of user's notes
func (h *Handler) NotesListPage(w http.ResponseWriter, r *http.Request) {
	// Get authenticated client
	client, err := h.getClient(r, w)
	if err != nil {
		log.Printf("Failed to create client: %v", err)
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
		log.Printf("Failed to fetch notes: %v", err)
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
		log.Printf("Failed to create client: %v", err)
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
		log.Printf("Failed to fetch note: %v", err)
		http.Error(w, "Note not found", http.StatusNotFound)
		return
	}

	// Fetch message references
	refsResp, err := noteClient.ListNoteMessageReferences(r.Context(), &notespb.ListNoteMessageReferencesRequest{
		NoteId: noteID,
	})
	if err != nil {
		log.Printf("Failed to fetch note references: %v", err)
		// Continue without references rather than failing completely
	}

	// Prepare template data with note-specific fields
	data := h.newTemplateData(r)
	data["Note"] = note
	data["References"] = refsResp.GetReferences()

	h.renderTemplate(w, "note.html", data)
}
