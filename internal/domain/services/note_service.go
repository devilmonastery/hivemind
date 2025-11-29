package services

import (
	"context"
	"fmt"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
)

// NoteService handles business logic for notes
type NoteService struct {
	noteRepo repositories.NoteRepository
}

// NewNoteService creates a new note service
func NewNoteService(noteRepo repositories.NoteRepository) *NoteService {
	return &NoteService{
		noteRepo: noteRepo,
	}
}

// CreateNote creates a new note
func (s *NoteService) CreateNote(ctx context.Context, note *entities.Note) (*entities.Note, error) {
	if err := s.noteRepo.Create(ctx, note); err != nil {
		return nil, fmt.Errorf("failed to create note: %w", err)
	}
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
	return s.noteRepo.GetByID(ctx, note.ID)
}

// DeleteNote soft-deletes a note
func (s *NoteService) DeleteNote(ctx context.Context, id string) error {
	if err := s.noteRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete note: %w", err)
	}
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
