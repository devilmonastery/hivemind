package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
)

// noteTitlesCacheEntry holds cached note titles for a user in a guild
type noteTitlesCacheEntry struct {
	titles    []struct{ ID, Title string }
	expiresAt time.Time
}

// NoteService handles business logic for notes
type NoteService struct {
	noteRepo       repositories.NoteRepository
	noteRefRepo    repositories.NoteMessageReferenceRepository
	titlesCache    sync.Map // map[authorID:guildID]noteTitlesCacheEntry
	titlesCacheTTL time.Duration
}

// NewNoteService creates a new note service
func NewNoteService(noteRepo repositories.NoteRepository, noteRefRepo repositories.NoteMessageReferenceRepository) *NoteService {
	return &NoteService{
		noteRepo:       noteRepo,
		noteRefRepo:    noteRefRepo,
		titlesCacheTTL: 1 * time.Minute,
	}
}

// CreateNote creates a new note
func (s *NoteService) CreateNote(ctx context.Context, note *entities.Note) (*entities.Note, error) {
	if err := s.noteRepo.Create(ctx, note); err != nil {
		return nil, fmt.Errorf("failed to create note: %w", err)
	}

	// Invalidate cache for this user+guild
	s.invalidateNoteTitlesCache(note.AuthorID, note.GuildID)

	return note, nil
}

// GetNote retrieves a note by ID
func (s *NoteService) GetNote(ctx context.Context, id string) (*entities.Note, error) {
	note, err := s.noteRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get note: %w", err)
	}
	return note, nil
}

// UpdateNote updates an existing note
func (s *NoteService) UpdateNote(ctx context.Context, note *entities.Note) (*entities.Note, error) {
	if err := s.noteRepo.Update(ctx, note); err != nil {
		return nil, fmt.Errorf("failed to update note: %w", err)
	}

	// Invalidate cache for this user+guild
	s.invalidateNoteTitlesCache(note.AuthorID, note.GuildID)

	return s.noteRepo.GetByID(ctx, note.ID)
}

// DeleteNote soft-deletes a note
func (s *NoteService) DeleteNote(ctx context.Context, id string) error {
	// Fetch the note to get authorID and guildID
	note, err := s.noteRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get note: %w", err)
	}

	if err := s.noteRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete note: %w", err)
	}

	// Invalidate cache for this user+guild
	s.invalidateNoteTitlesCache(note.AuthorID, note.GuildID)

	return nil
}

// ListNotes lists notes for a user
func (s *NoteService) ListNotes(ctx context.Context, authorID, guildID string, tags []string, limit, offset int, orderBy string, ascending bool) ([]*entities.Note, int, error) {
	notes, total, err := s.noteRepo.List(ctx, authorID, guildID, tags, limit, offset, orderBy, ascending)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list notes: %w", err)
	}
	return notes, total, nil
}

// SearchNotes searches notes by full-text query
func (s *NoteService) SearchNotes(ctx context.Context, authorID, query, guildID string, tags []string, limit, offset int) ([]*entities.Note, int, error) {
	notes, total, err := s.noteRepo.Search(ctx, authorID, query, guildID, tags, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search notes: %w", err)
	}
	return notes, total, nil
}

// AddMessageReference adds a message reference to a note
func (s *NoteService) AddMessageReference(ctx context.Context, ref *entities.NoteMessageReference) (*entities.NoteMessageReference, error) {
	// First verify the note exists and belongs to the user making the request
	note, err := s.noteRepo.GetByID(ctx, ref.NoteID)
	if err != nil {
		return nil, fmt.Errorf("failed to get note: %w", err)
	}
	if note == nil {
		return nil, fmt.Errorf("note not found")
	}

	if err := s.noteRefRepo.Create(ctx, ref); err != nil {
		return nil, fmt.Errorf("failed to add message reference: %w", err)
	}
	return ref, nil
}

// ListMessageReferences retrieves all message references for a note
func (s *NoteService) ListMessageReferences(ctx context.Context, noteID string) ([]*entities.NoteMessageReference, error) {
	// First verify the note exists and belongs to the user making the request
	note, err := s.noteRepo.GetByID(ctx, noteID)
	if err != nil {
		return nil, fmt.Errorf("failed to get note: %w", err)
	}
	if note == nil {
		return nil, fmt.Errorf("note not found")
	}

	refs, err := s.noteRefRepo.GetByNoteID(ctx, noteID)
	if err != nil {
		return nil, fmt.Errorf("failed to list message references: %w", err)
	}
	return refs, nil
}

// AutocompleteNoteTitles returns all note titles for a user in a guild (lightweight for autocomplete)
func (s *NoteService) AutocompleteNoteTitles(ctx context.Context, authorID, guildID string) ([]struct{ ID, Title string }, error) {
	// Get all titles for the user in the guild (with caching)
	titles, err := s.getNoteTitlesForUser(ctx, authorID, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get note titles: %w", err)
	}

	return titles, nil
}

// getNoteTitlesForUser retrieves note titles with caching
func (s *NoteService) getNoteTitlesForUser(ctx context.Context, authorID, guildID string) ([]struct{ ID, Title string }, error) {
	cacheKey := s.noteTitlesCacheKey(authorID, guildID)

	// Check cache
	if entry, ok := s.titlesCache.Load(cacheKey); ok {
		cached := entry.(noteTitlesCacheEntry)
		if time.Now().Before(cached.expiresAt) {
			return cached.titles, nil
		}
		// Cache expired, remove it
		s.titlesCache.Delete(cacheKey)
	}

	// Cache miss or expired, fetch from database
	titles, err := s.noteRepo.GetTitlesForUser(ctx, authorID, guildID)
	if err != nil {
		return nil, err
	}

	// Store in cache
	s.titlesCache.Store(cacheKey, noteTitlesCacheEntry{
		titles:    titles,
		expiresAt: time.Now().Add(s.titlesCacheTTL),
	})

	return titles, nil
}

// invalidateNoteTitlesCache invalidates the cache for a user in a guild
func (s *NoteService) invalidateNoteTitlesCache(authorID, guildID string) {
	s.titlesCache.Delete(s.noteTitlesCacheKey(authorID, guildID))
}

// noteTitlesCacheKey generates a cache key for user+guild combination
func (s *NoteService) noteTitlesCacheKey(authorID, guildID string) string {
	return authorID + ":" + guildID
}
