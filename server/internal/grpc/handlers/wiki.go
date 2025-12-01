package handlers

import (
	"context"
	"log"
	"log/slog"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonpb "github.com/devilmonastery/hivemind/api/generated/go/commonpb"
	wikipb "github.com/devilmonastery/hivemind/api/generated/go/wikipb"
	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
	"github.com/devilmonastery/hivemind/internal/domain/services"
	"github.com/devilmonastery/hivemind/internal/pkg/urlutil"
	"github.com/devilmonastery/hivemind/server/internal/grpc/interceptors"
)

type wikiHandler struct {
	wikipb.UnimplementedWikiServiceServer
	wikiService     *services.WikiService
	discordService  *services.DiscordService
	guildMemberRepo repositories.GuildMemberRepository
	discordUserRepo repositories.DiscordUserRepository
	log             *slog.Logger
}

// NewWikiHandler creates a new wiki gRPC handler
func NewWikiHandler(wikiService *services.WikiService, discordService *services.DiscordService, guildMemberRepo repositories.GuildMemberRepository, discordUserRepo repositories.DiscordUserRepository, logger *slog.Logger) wikipb.WikiServiceServer {
	return &wikiHandler{
		wikiService:     wikiService,
		discordService:  discordService,
		guildMemberRepo: guildMemberRepo,
		discordUserRepo: discordUserRepo,
		log:             logger.With(slog.String("handler", "wiki")),
	}
}

func (h *wikiHandler) CreateWikiPage(ctx context.Context, req *wikipb.CreateWikiPageRequest) (*wikipb.WikiPage, error) {
	// Get user context from auth interceptor
	userCtx, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userDiscordID := h.getUserDiscordID(ctx, userCtx)

	page := &entities.WikiPage{
		Title:     req.Title,
		Body:      req.Body,
		AuthorID:  userCtx.UserID,
		GuildID:   req.GuildId,
		ChannelID: req.ChannelId,
		Tags:      req.Tags,
	}

	created, err := h.wikiService.CreateWikiPage(ctx, page, userDiscordID)
	if err != nil {
		return nil, err
	}

	authorUsername := h.getUsernameForAuthor(ctx, created.AuthorID)
	return toProtoWikiPage(created, authorUsername), nil
}

func (h *wikiHandler) GetWikiPage(ctx context.Context, req *wikipb.GetWikiPageRequest) (*wikipb.WikiPage, error) {
	userCtx, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userDiscordID := h.getUserDiscordID(ctx, userCtx)

	page, err := h.wikiService.GetWikiPage(ctx, req.Id, userDiscordID)
	if err != nil {
		return nil, err
	}

	return toProtoWikiPage(page, userCtx.Username), nil
}

func (h *wikiHandler) GetWikiPageByTitle(ctx context.Context, req *wikipb.GetWikiPageByTitleRequest) (*wikipb.WikiPage, error) {
	userCtx, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userDiscordID := h.getUserDiscordID(ctx, userCtx)

	page, err := h.wikiService.GetWikiPageByTitle(ctx, req.GuildId, req.Title, userDiscordID)
	if err != nil {
		return nil, err
	}
	if page == nil {
		return nil, status.Error(codes.NotFound, "wiki page not found")
	}

	return toProtoWikiPage(page, userCtx.Username), nil
}

func (h *wikiHandler) SearchWikiPages(ctx context.Context, req *wikipb.SearchWikiPagesRequest) (*wikipb.SearchWikiPagesResponse, error) {
	userCtx, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userDiscordID := h.getUserDiscordID(ctx, userCtx)

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 10
	}

	pages, total, err := h.wikiService.SearchWikiPages(ctx, req.GuildId, req.Query, req.Tags, limit, int(req.Offset), userDiscordID)
	if err != nil {
		return nil, err
	}

	protoPages := make([]*wikipb.WikiPage, len(pages))
	for i, page := range pages {
		authorUsername := h.getUsernameForAuthor(ctx, page.AuthorID)
		protoPages[i] = toProtoWikiPage(page, authorUsername)
	}

	return &wikipb.SearchWikiPagesResponse{
		Pages: protoPages,
		Total: int32(total),
	}, nil
}

func (h *wikiHandler) UpdateWikiPage(ctx context.Context, req *wikipb.UpdateWikiPageRequest) (*wikipb.WikiPage, error) {
	userCtx, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userDiscordID := h.getUserDiscordID(ctx, userCtx)

	page := &entities.WikiPage{
		ID:    req.Id,
		Title: req.Title,
		Body:  req.Body,
		Tags:  req.Tags,
	}

	updated, err := h.wikiService.UpdateWikiPage(ctx, page, userDiscordID)
	if err != nil {
		return nil, err
	}

	return toProtoWikiPage(updated, userCtx.Username), nil
}

func (h *wikiHandler) UpsertWikiPage(ctx context.Context, req *wikipb.UpsertWikiPageRequest) (*wikipb.UpsertWikiPageResponse, error) {
	// Get user context from auth interceptor
	userCtx, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userDiscordID := h.getUserDiscordID(ctx, userCtx)

	// Validate title is not empty
	if strings.TrimSpace(req.Title) == "" {
		return nil, status.Error(codes.InvalidArgument, "wiki page title cannot be empty")
	}

	// Validate body is not empty
	if strings.TrimSpace(req.Body) == "" {
		return nil, status.Error(codes.InvalidArgument, "wiki page body cannot be empty")
	}

	page := &entities.WikiPage{
		Title:     req.Title,
		Body:      req.Body,
		AuthorID:  userCtx.UserID,
		GuildID:   req.GuildId,
		ChannelID: req.ChannelId,
		Tags:      req.Tags,
	}

	upserted, created, err := h.wikiService.UpsertWikiPage(ctx, page, userDiscordID)
	if err != nil {
		return nil, err
	}

	authorUsername := h.getUsernameForAuthor(ctx, upserted.AuthorID)
	return &wikipb.UpsertWikiPageResponse{
		Page:    toProtoWikiPage(upserted, authorUsername),
		Created: created,
	}, nil
}

func (h *wikiHandler) DeleteWikiPage(ctx context.Context, req *wikipb.DeleteWikiPageRequest) (*commonpb.SuccessResponse, error) {
	userCtx, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userDiscordID := h.getUserDiscordID(ctx, userCtx)

	if err := h.wikiService.DeleteWikiPage(ctx, req.Id, userDiscordID); err != nil {
		return nil, err
	}

	return &commonpb.SuccessResponse{
		Success: true,
		Message: "Wiki page deleted successfully",
	}, nil
}

func (h *wikiHandler) ListWikiPages(ctx context.Context, req *wikipb.ListWikiPagesRequest) (*wikipb.ListWikiPagesResponse, error) {
	userCtx, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userDiscordID := h.getUserDiscordID(ctx, userCtx)

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}

	orderBy := req.OrderBy
	if orderBy == "" {
		orderBy = "created_at"
	}

	pages, total, err := h.wikiService.ListWikiPages(ctx, req.GuildId, limit, int(req.Offset), orderBy, req.Ascending, userDiscordID)
	if err != nil {
		return nil, err
	}

	protoPages := make([]*wikipb.WikiPage, len(pages))
	for i, page := range pages {
		authorUsername := h.getUsernameForAuthor(ctx, page.AuthorID)
		protoPages[i] = toProtoWikiPage(page, authorUsername)
	}

	return &wikipb.ListWikiPagesResponse{
		Pages: protoPages,
		Total: int32(total),
	}, nil
}

func toProtoWikiPage(page *entities.WikiPage, authorUsername string) *wikipb.WikiPage {
	return &wikipb.WikiPage{
		Id:             page.ID,
		Title:          page.Title,
		Slug:           page.Slug,
		Body:           page.Body,
		AuthorId:       page.AuthorID,
		AuthorUsername: authorUsername,
		GuildId:        page.GuildID,
		GuildName:      page.GuildName,
		ChannelId:      page.ChannelID,
		Tags:           page.Tags,
		CreatedAt:      timestamppb.New(page.CreatedAt),
		UpdatedAt:      timestamppb.New(page.UpdatedAt),
	}
}

// getUsernameForAuthor looks up the username for a given author ID
func (h *wikiHandler) getUsernameForAuthor(ctx context.Context, authorID string) string {
	// Try to get Discord user for this author
	discordUser, err := h.discordService.GetDiscordUserByHivemindID(ctx, authorID)
	if err == nil && discordUser != nil {
		// Prefer global name, fallback to username
		if discordUser.DiscordGlobalName != nil && *discordUser.DiscordGlobalName != "" {
			return *discordUser.DiscordGlobalName
		}
		return discordUser.DiscordUsername
	}
	// Fallback to author ID if username not found
	return authorID
}

// getUserDiscordID extracts Discord ID from context for ACL filtering
// Returns empty string for admin users (no ACL filtering)
func (h *wikiHandler) getUserDiscordID(ctx context.Context, userCtx *interceptors.UserContext) string {
	log.Printf("[WikiHandler.getUserDiscordID] UserID: %s, Role: %s", userCtx.UserID, userCtx.Role)

	// Admin bypass: empty string means no ACL filtering
	if userCtx.Role == "admin" {
		log.Printf("[WikiHandler.getUserDiscordID] Admin bypass - no ACL filtering")
		return ""
	}

	// Get user's Discord ID for ACL filtering
	discordUser, err := h.discordUserRepo.GetByUserID(ctx, userCtx.UserID)
	if err != nil {
		log.Printf("[WikiHandler.getUserDiscordID] Error getting Discord user: %v", err)
		return ""
	}
	if discordUser == nil {
		log.Printf("[WikiHandler.getUserDiscordID] No Discord user found for UserID %s", userCtx.UserID)
		return ""
	}

	log.Printf("[WikiHandler.getUserDiscordID] Found Discord ID: %s", discordUser.DiscordID)
	return discordUser.DiscordID
}

func (h *wikiHandler) AddWikiMessageReference(ctx context.Context, req *wikipb.AddWikiMessageReferenceRequest) (*wikipb.WikiMessageReference, error) {
	// Get user context from auth interceptor
	userCtx, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Debug logging
	h.log.Debug("AddWikiMessageReference request",
		slog.String("guild_id", req.GuildId),
		slog.String("content", req.Content),
		slog.Int("content_length", len(req.Content)))

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

	ref := &entities.WikiMessageReference{
		WikiPageID:        req.WikiPageId,
		MessageID:         req.MessageId,
		ChannelID:         req.ChannelId,
		GuildID:           req.GuildId,
		Content:           req.Content,
		AuthorID:          req.AuthorId,
		AuthorUsername:    req.AuthorUsername,
		AuthorDisplayName: req.AuthorDisplayName,
		MessageTimestamp:  req.MessageTimestamp.AsTime(),
		AttachmentURLs:    req.AttachmentUrls, // Keep for backwards compatibility
		Attachments:       attachments,
		AddedByUserID:     userCtx.UserID,
	}

	err = h.wikiService.AddWikiMessageReference(ctx, ref)
	if err != nil {
		h.log.Error("error adding message reference", slog.String("error", err.Error()))
		return nil, err
	}

	return toProtoWikiMessageReference(ref), nil
}

func (h *wikiHandler) ListWikiMessageReferences(ctx context.Context, req *wikipb.ListWikiMessageReferencesRequest) (*wikipb.ListWikiMessageReferencesResponse, error) {
	refs, err := h.wikiService.ListWikiMessageReferences(ctx, req.WikiPageId)
	if err != nil {
		return nil, err
	}

	protoRefs := make([]*wikipb.WikiMessageReference, len(refs))
	for i, ref := range refs {
		protoRefs[i] = toProtoWikiMessageReference(ref)
	}

	return &wikipb.ListWikiMessageReferencesResponse{
		References: protoRefs,
	}, nil
}

func (h *wikiHandler) AutocompleteWikiTitles(ctx context.Context, req *wikipb.AutocompleteWikiTitlesRequest) (*wikipb.AutocompleteWikiTitlesResponse, error) {
	titles, err := h.wikiService.AutocompleteWikiTitles(ctx, req.GuildId)
	if err != nil {
		return nil, err
	}

	suggestions := make([]*wikipb.WikiTitleSuggestion, len(titles))
	for i, title := range titles {
		suggestions[i] = &wikipb.WikiTitleSuggestion{
			Id:    title.ID,
			Title: title.Title,
			Slug:  title.Slug,
		}
	}

	return &wikipb.AutocompleteWikiTitlesResponse{
		Suggestions: suggestions,
	}, nil
}

func (h *wikiHandler) MergeWikiPages(ctx context.Context, req *wikipb.MergeWikiPagesRequest) (*wikipb.WikiPage, error) {
	// Get user context from auth interceptor
	userCtx, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Validate request
	if req.SourcePageId == "" {
		return nil, status.Error(codes.InvalidArgument, "source_page_id is required")
	}
	if req.TargetPageId == "" {
		return nil, status.Error(codes.InvalidArgument, "target_page_id is required")
	}
	if req.SourcePageId == req.TargetPageId {
		return nil, status.Error(codes.InvalidArgument, "source and target pages must be different")
	}

	// Perform merge
	merged, err := h.wikiService.MergeWikiPages(ctx, req.SourcePageId, req.TargetPageId, userCtx.UserID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if strings.Contains(err.Error(), "different guilds") {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		h.log.ErrorContext(ctx, "failed to merge wiki pages",
			slog.String("source_page_id", req.SourcePageId),
			slog.String("target_page_id", req.TargetPageId),
			slog.String("error", err.Error()),
		)
		return nil, status.Error(codes.Internal, "failed to merge wiki pages")
	}

	authorUsername := h.getUsernameForAuthor(ctx, merged.AuthorID)
	return toProtoWikiPage(merged, authorUsername), nil
}

func toProtoWikiMessageReference(ref *entities.WikiMessageReference) *wikipb.WikiMessageReference {
	discordLink := urlutil.DiscordMessageURL(ref.GuildID, ref.ChannelID, ref.MessageID)

	// Convert entity attachments to proto attachments
	protoAttachments := make([]*wikipb.AttachmentMetadata, len(ref.Attachments))
	for i, att := range ref.Attachments {
		protoAttachments[i] = &wikipb.AttachmentMetadata{
			Url:         att.URL,
			ContentType: att.ContentType,
			Filename:    att.Filename,
			Width:       int32(att.Width),
			Height:      int32(att.Height),
			Size:        att.Size,
		}
	}

	return &wikipb.WikiMessageReference{
		Id:                ref.ID,
		WikiPageId:        ref.WikiPageID,
		MessageId:         ref.MessageID,
		ChannelId:         ref.ChannelID,
		GuildId:           ref.GuildID,
		Content:           ref.Content,
		AuthorId:          ref.AuthorID,
		AuthorUsername:    ref.AuthorUsername,
		AuthorDisplayName: ref.AuthorDisplayName,
		AuthorAvatarUrl:   ref.AuthorAvatarURL,
		MessageTimestamp:  timestamppb.New(ref.MessageTimestamp),
		AttachmentUrls:    ref.AttachmentURLs, // Keep for backwards compatibility
		Attachments:       protoAttachments,
		AddedAt:           timestamppb.New(ref.AddedAt),
		AddedByUserId:     ref.AddedByUserID,
		DiscordLink:       discordLink,
	}
}
