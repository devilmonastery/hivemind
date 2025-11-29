package handlers

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

	commonpb "github.com/devilmonastery/hivemind/api/generated/go/commonpb"
	wikipb "github.com/devilmonastery/hivemind/api/generated/go/wikipb"
	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/services"
	"github.com/devilmonastery/hivemind/server/internal/grpc/interceptors"
)

type wikiHandler struct {
	wikipb.UnimplementedWikiServiceServer
	wikiService    *services.WikiService
	discordService *services.DiscordService
}

// NewWikiHandler creates a new wiki gRPC handler
func NewWikiHandler(wikiService *services.WikiService, discordService *services.DiscordService) wikipb.WikiServiceServer {
	return &wikiHandler{
		wikiService:    wikiService,
		discordService: discordService,
	}
}

func (h *wikiHandler) CreateWikiPage(ctx context.Context, req *wikipb.CreateWikiPageRequest) (*wikipb.WikiPage, error) {
	// Get user context from auth interceptor
	userCtx, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	page := &entities.WikiPage{
		Title:     req.Title,
		Body:      req.Body,
		AuthorID:  userCtx.UserID,
		GuildID:   req.GuildId,
		ChannelID: req.ChannelId,
		Tags:      req.Tags,
	}

	created, err := h.wikiService.CreateWikiPage(ctx, page)
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

	page, err := h.wikiService.GetWikiPage(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	return toProtoWikiPage(page, userCtx.Username), nil
}

func (h *wikiHandler) SearchWikiPages(ctx context.Context, req *wikipb.SearchWikiPagesRequest) (*wikipb.SearchWikiPagesResponse, error) {
	userCtx, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 10
	}

	pages, total, err := h.wikiService.SearchWikiPages(ctx, req.GuildId, req.Query, req.Tags, limit, int(req.Offset))
	if err != nil {
		return nil, err
	}

	protoPages := make([]*wikipb.WikiPage, len(pages))
	for i, page := range pages {
		protoPages[i] = toProtoWikiPage(page, userCtx.Username)
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

	page := &entities.WikiPage{
		ID:    req.Id,
		Title: req.Title,
		Body:  req.Body,
		Tags:  req.Tags,
	}

	updated, err := h.wikiService.UpdateWikiPage(ctx, page)
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

	page := &entities.WikiPage{
		Title:     req.Title,
		Body:      req.Body,
		AuthorID:  userCtx.UserID,
		GuildID:   req.GuildId,
		ChannelID: req.ChannelId,
		Tags:      req.Tags,
	}

	upserted, created, err := h.wikiService.UpsertWikiPage(ctx, page)
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
	if err := h.wikiService.DeleteWikiPage(ctx, req.Id); err != nil {
		return nil, err
	}

	return &commonpb.SuccessResponse{
		Success: true,
		Message: "Wiki page deleted successfully",
	}, nil
}

func (h *wikiHandler) ListWikiPages(ctx context.Context, req *wikipb.ListWikiPagesRequest) (*wikipb.ListWikiPagesResponse, error) {
	_, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}

	orderBy := req.OrderBy
	if orderBy == "" {
		orderBy = "created_at"
	}

	pages, total, err := h.wikiService.ListWikiPages(ctx, req.GuildId, limit, int(req.Offset), orderBy, req.Ascending)
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
		return discordUser.DiscordUsername
	}
	// Fallback to author ID if username not found
	return authorID
}
