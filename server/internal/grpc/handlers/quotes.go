package handlers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/devilmonastery/hivemind/api/generated/go/commonpb"
	"github.com/devilmonastery/hivemind/api/generated/go/quotespb"
	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
	"github.com/devilmonastery/hivemind/internal/domain/services"
	"github.com/devilmonastery/hivemind/server/internal/grpc/interceptors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// QuoteHandler implements the QuoteService gRPC handler
type QuoteHandler struct {
	quotespb.UnimplementedQuoteServiceServer
	quoteService    *services.QuoteService
	discordUserRepo repositories.DiscordUserRepository
	log             *slog.Logger
}

// NewQuoteHandler creates a new quote handler
func NewQuoteHandler(quoteService *services.QuoteService, discordUserRepo repositories.DiscordUserRepository) *QuoteHandler {
	return &QuoteHandler{
		quoteService:    quoteService,
		discordUserRepo: discordUserRepo,
		log:             slog.Default().With(slog.String("handler", "quote")),
	}
}

// getUserDiscordID extracts Discord ID from context for ACL filtering
// Returns empty string for admin users (no ACL filtering)
func (h *QuoteHandler) getUserDiscordID(ctx context.Context, userCtx *interceptors.UserContext) string {
	h.log.Debug("getting user discord ID for ACL",
		slog.String("user_id", userCtx.UserID),
		slog.String("role", userCtx.Role))

	// Admin bypass: empty string means no ACL filtering
	if userCtx.Role == "admin" {
		h.log.Debug("admin bypass - no ACL filtering")
		return ""
	}

	// Get user's Discord ID for ACL filtering
	discordUser, err := h.discordUserRepo.GetByUserID(ctx, userCtx.UserID)
	if err != nil {
		h.log.Debug("error getting discord user", slog.String("error", err.Error()))
		return ""
	}
	if discordUser == nil {
		h.log.Debug("no discord user found", slog.String("user_id", userCtx.UserID))
		return ""
	}

	h.log.Debug("found discord ID", slog.String("discord_id", discordUser.DiscordID))
	return discordUser.DiscordID
}

// CreateQuote creates a new quote
func (h *QuoteHandler) CreateQuote(ctx context.Context, req *quotespb.CreateQuoteRequest) (*quotespb.Quote, error) {
	user, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "user context not found")
	}

	quote := &entities.Quote{
		Body:                     req.Body,
		Tags:                     req.Tags,
		GuildID:                  req.GuildId,
		AuthorID:                 user.UserID,
		AuthorDiscordID:          user.DiscordID,
		SourceMsgID:              req.SourceMsgId,
		SourceChannelID:          req.SourceChannelId,
		SourceChannelName:        req.SourceChannelName,
		SourceMsgAuthorDiscordID: req.SourceMsgAuthorDiscordId,
		SourceMsgAuthorUsername:  req.SourceMsgAuthorUsername,
		SourceMsgTimestamp:       req.SourceMsgTimestamp.AsTime(),
	}

	created, err := h.quoteService.CreateQuote(ctx, quote)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create quote: %v", err)
	}

	// Fetch the quote back to populate guild_nick fields from guild_members JOIN
	userDiscordID := h.getUserDiscordID(ctx, user)
	fetched, err := h.quoteService.GetQuote(ctx, created.ID, userDiscordID)
	if err != nil {
		h.log.Warn("Failed to fetch quote after creation, returning without guild nicks",
			slog.String("quote_id", created.ID),
			slog.String("error", err.Error()))
		return quoteToProto(created), nil
	}

	return quoteToProto(fetched), nil
}

// GetQuote retrieves a quote by ID
func (h *QuoteHandler) GetQuote(ctx context.Context, req *quotespb.GetQuoteRequest) (*quotespb.Quote, error) {
	userCtx, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "user context not found")
	}

	userDiscordID := h.getUserDiscordID(ctx, userCtx)

	quote, err := h.quoteService.GetQuote(ctx, req.Id, userDiscordID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "quote not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get quote: %v", err)
	}

	return quoteToProto(quote), nil
}

// DeleteQuote deletes a quote
func (h *QuoteHandler) DeleteQuote(ctx context.Context, req *quotespb.DeleteQuoteRequest) (*commonpb.SuccessResponse, error) {
	user, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "user context not found")
	}

	userDiscordID := h.getUserDiscordID(ctx, user)

	// Get existing quote to verify ownership
	existing, err := h.quoteService.GetQuote(ctx, req.Id, userDiscordID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "quote not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get quote: %v", err)
	}

	if existing.AuthorID != user.UserID {
		return nil, status.Error(codes.PermissionDenied, "you can only delete quotes you saved")
	}

	if err := h.quoteService.DeleteQuote(ctx, req.Id); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete quote: %v", err)
	}

	return &commonpb.SuccessResponse{Success: true}, nil
}

// UpdateQuote updates a quote's body and tags
func (h *QuoteHandler) UpdateQuote(ctx context.Context, req *quotespb.UpdateQuoteRequest) (*quotespb.Quote, error) {
	user, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "user context not found")
	}

	userDiscordID := h.getUserDiscordID(ctx, user)

	// Get existing quote to verify ownership
	existing, err := h.quoteService.GetQuote(ctx, req.Id, userDiscordID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "quote not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get quote: %v", err)
	}

	if existing.AuthorID != user.UserID {
		return nil, status.Error(codes.PermissionDenied, "you can only edit quotes you saved")
	}

	// Update the quote
	updated, err := h.quoteService.UpdateQuote(ctx, req.Id, req.Body, req.Tags, userDiscordID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update quote: %v", err)
	}

	return quoteToProto(updated), nil
}

// ListQuotes lists quotes in a guild
func (h *QuoteHandler) ListQuotes(ctx context.Context, req *quotespb.ListQuotesRequest) (*quotespb.ListQuotesResponse, error) {
	userCtx, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "user context not found")
	}

	userDiscordID := h.getUserDiscordID(ctx, userCtx)

	limit := int(req.Limit)
	if limit == 0 {
		limit = 20
	}

	orderBy := req.OrderBy
	if orderBy == "" {
		orderBy = "created_at"
	}

	quotes, total, err := h.quoteService.ListQuotes(ctx, req.GuildId, limit, int(req.Offset), orderBy, req.Ascending, userDiscordID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list quotes: %v", err)
	}

	protoQuotes := make([]*quotespb.Quote, len(quotes))
	for i, quote := range quotes {
		protoQuotes[i] = quoteToProto(quote)
	}

	return &quotespb.ListQuotesResponse{
		Quotes: protoQuotes,
		Total:  int32(total),
	}, nil
}

// SearchQuotes searches quotes by full-text query
func (h *QuoteHandler) SearchQuotes(ctx context.Context, req *quotespb.SearchQuotesRequest) (*quotespb.SearchQuotesResponse, error) {
	userCtx, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "user context not found")
	}

	userDiscordID := h.getUserDiscordID(ctx, userCtx)

	limit := int(req.Limit)
	if limit == 0 {
		limit = 20
	}

	quotes, total, err := h.quoteService.SearchQuotes(ctx, req.GuildId, req.Query, req.Tags, limit, int(req.Offset), userDiscordID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to search quotes: %v", err)
	}

	protoQuotes := make([]*quotespb.Quote, len(quotes))
	for i, quote := range quotes {
		protoQuotes[i] = quoteToProto(quote)
	}

	return &quotespb.SearchQuotesResponse{
		Quotes: protoQuotes,
		Total:  int32(total),
	}, nil
}

// GetRandomQuote retrieves a random quote from a guild
func (h *QuoteHandler) GetRandomQuote(ctx context.Context, req *quotespb.GetRandomQuoteRequest) (*quotespb.Quote, error) {
	_, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "user context not found")
	}

	quote, err := h.quoteService.GetRandomQuote(ctx, req.GuildId, req.Tags)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "no quotes found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get random quote: %v", err)
	}

	return quoteToProto(quote), nil
}

// quoteToProto converts a domain quote to protobuf
func quoteToProto(quote *entities.Quote) *quotespb.Quote {
	proto := &quotespb.Quote{
		Id:                       quote.ID,
		Body:                     quote.Body,
		Tags:                     quote.Tags,
		AuthorId:                 quote.AuthorID,
		AuthorDiscordId:          quote.AuthorDiscordID,
		AuthorUsername:           quote.AuthorDisplayName, // Use display name from view
		AuthorGuildNick:          quote.AuthorGuildNick,   // Keep for backward compatibility
		GuildId:                  quote.GuildID,
		GuildName:                quote.GuildName,
		SourceMsgId:              quote.SourceMsgID,
		SourceChannelId:          quote.SourceChannelID,
		SourceChannelName:        quote.SourceChannelName,
		SourceMsgAuthorDiscordId: quote.SourceMsgAuthorDiscordID,
		SourceMsgAuthorUsername:  quote.SourceMsgAuthorDisplayName, // Use display name from view
		SourceMsgAuthorGuildNick: quote.SourceMsgAuthorGuildNick,   // Keep for backward compatibility
		CreatedAt:                timestamppb.New(quote.CreatedAt),
	}
	if !quote.SourceMsgTimestamp.IsZero() {
		proto.SourceMsgTimestamp = timestamppb.New(quote.SourceMsgTimestamp)
	}
	return proto
}
