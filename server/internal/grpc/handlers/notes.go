package handlers

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/devilmonastery/hivemind/api/generated/go/commonpb"
	"github.com/devilmonastery/hivemind/api/generated/go/notespb"
	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/services"
	"github.com/devilmonastery/hivemind/internal/pkg/urlutil"
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

	// Validate title is not empty
	if strings.TrimSpace(req.Title) == "" {
		return nil, status.Error(codes.InvalidArgument, "note title cannot be empty")
	}

	// Validate body is not empty
	if strings.TrimSpace(req.Body) == "" {
		return nil, status.Error(codes.InvalidArgument, "note body cannot be empty")
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

	// Validate title is not empty
	if strings.TrimSpace(req.Title) == "" {
		return nil, status.Error(codes.InvalidArgument, "note title cannot be empty")
	}

	// Validate body is not empty
	if strings.TrimSpace(req.Body) == "" {
		return nil, status.Error(codes.InvalidArgument, "note body cannot be empty")
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
		GuildName:       note.GuildName,
		ChannelId:       note.ChannelID,
		SourceMsgId:     note.SourceMsgID,
		SourceChannelId: note.SourceChannelID,
		CreatedAt:       timestamppb.New(note.CreatedAt),
		UpdatedAt:       timestamppb.New(note.UpdatedAt),
	}
}

// AddNoteMessageReference adds a Discord message reference to a note
func (h *NoteHandler) AddNoteMessageReference(ctx context.Context, req *notespb.AddNoteMessageReferenceRequest) (*notespb.NoteMessageReference, error) {
	user, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "user context not found")
	}

	// Verify note ownership
	note, err := h.noteService.GetNote(ctx, req.NoteId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "note not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get note: %v", err)
	}

	if note.AuthorID != user.UserID {
		return nil, status.Error(codes.PermissionDenied, "you can only add references to your own notes")
	}

	// Convert proto attachments to entity attachments
	attachments := make([]entities.AttachmentMetadata, len(req.Attachments))
	for i, att := range req.Attachments {
		attachments[i] = entities.AttachmentMetadata{
			URL:         att.Url,
			ContentType: att.ContentType,
			Filename:    att.Filename,
			Width:       int(att.Width),
			Height:      int(att.Height),
			Size:        att.Size,
		}
	}

	ref := &entities.NoteMessageReference{
		NoteID:            req.NoteId,
		MessageID:         req.MessageId,
		ChannelID:         req.ChannelId,
		GuildID:           req.GuildId,
		Content:           req.Content,
		AuthorID:          req.AuthorId,
		AuthorUsername:    req.AuthorUsername,
		AuthorDisplayName: req.AuthorDisplayName,
		MessageTimestamp:  req.MessageTimestamp.AsTime(),
		Attachments:       attachments,
	}

	created, err := h.noteService.AddMessageReference(ctx, ref)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to add message reference: %v", err)
	}

	return noteMessageReferenceToProto(created), nil
}

// ListNoteMessageReferences lists all message references for a note
func (h *NoteHandler) ListNoteMessageReferences(ctx context.Context, req *notespb.ListNoteMessageReferencesRequest) (*notespb.ListNoteMessageReferencesResponse, error) {
	user, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "user context not found")
	}

	// Verify note ownership
	note, err := h.noteService.GetNote(ctx, req.NoteId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "note not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get note: %v", err)
	}

	if note.AuthorID != user.UserID {
		return nil, status.Error(codes.PermissionDenied, "you can only view references for your own notes")
	}

	refs, err := h.noteService.ListMessageReferences(ctx, req.NoteId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list message references: %v", err)
	}

	protoRefs := make([]*notespb.NoteMessageReference, len(refs))
	for i, ref := range refs {
		protoRefs[i] = noteMessageReferenceToProto(ref)
	}

	return &notespb.ListNoteMessageReferencesResponse{
		References: protoRefs,
	}, nil
}

func (h *NoteHandler) AutocompleteNoteTitles(ctx context.Context, req *notespb.AutocompleteNoteTitlesRequest) (*notespb.AutocompleteNoteTitlesResponse, error) {
	user, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "user context not found")
	}

	titles, err := h.noteService.AutocompleteNoteTitles(ctx, user.UserID, req.GuildId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to autocomplete note titles: %v", err)
	}

	suggestions := make([]*notespb.NoteTitleSuggestion, len(titles))
	for i, title := range titles {
		suggestions[i] = &notespb.NoteTitleSuggestion{
			Id:    title.ID,
			Title: title.Title,
		}
	}

	return &notespb.AutocompleteNoteTitlesResponse{
		Suggestions: suggestions,
	}, nil
}

// noteMessageReferenceToProto converts a domain note message reference to protobuf
func noteMessageReferenceToProto(ref *entities.NoteMessageReference) *notespb.NoteMessageReference {
	attachments := make([]*notespb.AttachmentMetadata, len(ref.Attachments))
	for i, att := range ref.Attachments {
		attachments[i] = &notespb.AttachmentMetadata{
			Url:         att.URL,
			ContentType: att.ContentType,
			Filename:    att.Filename,
			Width:       int32(att.Width),
			Height:      int32(att.Height),
			Size:        att.Size,
		}
	}

	// Compute Discord link if we have the required IDs
	discordLink := ""
	if ref.GuildID != "" && ref.ChannelID != "" && ref.MessageID != "" {
		discordLink = urlutil.DiscordMessageURL(ref.GuildID, ref.ChannelID, ref.MessageID)
	}

	return &notespb.NoteMessageReference{
		Id:                ref.ID,
		NoteId:            ref.NoteID,
		MessageId:         ref.MessageID,
		ChannelId:         ref.ChannelID,
		GuildId:           ref.GuildID,
		Content:           ref.Content,
		AuthorId:          ref.AuthorID,
		AuthorUsername:    ref.AuthorUsername,
		AuthorDisplayName: ref.AuthorDisplayName,
		AuthorAvatarUrl:   ref.AuthorAvatarURL,
		MessageTimestamp:  timestamppb.New(ref.MessageTimestamp),
		Attachments:       attachments,
		AddedAt:           timestamppb.New(ref.AddedAt),
		DiscordLink:       discordLink,
	}
}
