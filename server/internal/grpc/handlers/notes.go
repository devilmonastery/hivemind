package handlers

import (
	"context"
	"database/sql"
	"errors"

	"github.com/devilmonastery/hivemind/api/generated/go/commonpb"
	"github.com/devilmonastery/hivemind/api/generated/go/notespb"
	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/services"
	"github.com/devilmonastery/hivemind/server/internal/grpc/interceptors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// NoteHandler implements the NoteService gRPC handler
type NoteHandler struct {
	notespb.UnimplementedNoteServiceServer
	noteService *services.NoteService
}

// NewNoteHandler creates a new note handler
func NewNoteHandler(noteService *services.NoteService) *NoteHandler {
	return &NoteHandler{
		noteService: noteService,
	}
}

// CreateNote creates a new note
func (h *NoteHandler) CreateNote(ctx context.Context, req *notespb.CreateNoteRequest) (*notespb.Note, error) {
	user, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "user context not found")
	}

	note := &entities.Note{
		Title:       req.Title,
		Body:        req.Body,
		Tags:        req.Tags,
		AuthorID:    user.UserID,
		GuildID:     req.GuildId,
		ChannelID:   req.ChannelId,
		SourceMsgID: req.SourceMsgId,
	}

	created, err := h.noteService.CreateNote(ctx, note)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create note: %v", err)
	}

	return noteToProto(created), nil
}

// GetNote retrieves a note by ID
func (h *NoteHandler) GetNote(ctx context.Context, req *notespb.GetNoteRequest) (*notespb.Note, error) {
	_, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "user context not found")
	}

	note, err := h.noteService.GetNote(ctx, req.Id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "note not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get note: %v", err)
	}

	return noteToProto(note), nil
}

// UpdateNote updates an existing note
func (h *NoteHandler) UpdateNote(ctx context.Context, req *notespb.UpdateNoteRequest) (*notespb.Note, error) {
	user, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "user context not found")
	}

	// Get existing note to verify ownership
	existing, err := h.noteService.GetNote(ctx, req.Id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "note not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get note: %v", err)
	}

	if existing.AuthorID != user.UserID {
		return nil, status.Error(codes.PermissionDenied, "you can only update your own notes")
	}

	note := &entities.Note{
		ID:       req.Id,
		Title:    req.Title,
		Body:     req.Body,
		Tags:     req.Tags,
		AuthorID: user.UserID,
	}

	updated, err := h.noteService.UpdateNote(ctx, note)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update note: %v", err)
	}

	return noteToProto(updated), nil
}

// DeleteNote deletes a note
func (h *NoteHandler) DeleteNote(ctx context.Context, req *notespb.DeleteNoteRequest) (*commonpb.SuccessResponse, error) {
	user, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "user context not found")
	}

	// Get existing note to verify ownership
	existing, err := h.noteService.GetNote(ctx, req.Id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "note not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get note: %v", err)
	}

	if existing.AuthorID != user.UserID {
		return nil, status.Error(codes.PermissionDenied, "you can only delete your own notes")
	}

	if err := h.noteService.DeleteNote(ctx, req.Id); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete note: %v", err)
	}

	return &commonpb.SuccessResponse{Success: true}, nil
}

// ListNotes lists notes for the authenticated user
func (h *NoteHandler) ListNotes(ctx context.Context, req *notespb.ListNotesRequest) (*notespb.ListNotesResponse, error) {
	user, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "user context not found")
	}

	limit := int(req.Limit)
	if limit == 0 {
		limit = 20
	}

	orderBy := req.OrderBy
	if orderBy == "" {
		orderBy = "created_at"
	}

	notes, total, err := h.noteService.ListNotes(ctx, user.UserID, req.GuildId, req.Tags, limit, int(req.Offset), orderBy, req.Ascending)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list notes: %v", err)
	}

	protoNotes := make([]*notespb.Note, len(notes))
	for i, note := range notes {
		protoNotes[i] = noteToProto(note)
	}

	return &notespb.ListNotesResponse{
		Notes: protoNotes,
		Total: int32(total),
	}, nil
}

// SearchNotes searches notes by full-text query
func (h *NoteHandler) SearchNotes(ctx context.Context, req *notespb.SearchNotesRequest) (*notespb.SearchNotesResponse, error) {
	user, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "user context not found")
	}

	limit := int(req.Limit)
	if limit == 0 {
		limit = 20
	}

	notes, total, err := h.noteService.SearchNotes(ctx, user.UserID, req.Query, req.GuildId, req.Tags, limit, int(req.Offset))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to search notes: %v", err)
	}

	protoNotes := make([]*notespb.Note, len(notes))
	for i, note := range notes {
		protoNotes[i] = noteToProto(note)
	}

	return &notespb.SearchNotesResponse{
		Notes: protoNotes,
		Total: int32(total),
	}, nil
}

// noteToProto converts a domain note to protobuf
func noteToProto(note *entities.Note) *notespb.Note {
	return &notespb.Note{
		Id:              note.ID,
		Title:           note.Title,
		Body:            note.Body,
		Tags:            note.Tags,
		AuthorId:        note.AuthorID,
		GuildId:         note.GuildID,
		ChannelId:       note.ChannelID,
		SourceMsgId:     note.SourceMsgID,
		SourceChannelId: note.SourceChannelID,
		CreatedAt:       timestamppb.New(note.CreatedAt),
		UpdatedAt:       timestamppb.New(note.UpdatedAt),
	}
}
