package handlers

import (
	"context"
	"database/sql"
	"errors"

	"github.com/devilmonastery/hivemind/api/generated/go/commonpb"
	"github.com/devilmonastery/hivemind/api/generated/go/quotespb"
	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/services"
	"github.com/devilmonastery/hivemind/server/internal/grpc/interceptors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// QuoteHandler implements the QuoteService gRPC handler
type QuoteHandler struct {
	quotespb.UnimplementedQuoteServiceServer
	quoteService *services.QuoteService
}

// NewQuoteHandler creates a new quote handler
func NewQuoteHandler(quoteService *services.QuoteService) *QuoteHandler {
	return &QuoteHandler{
		quoteService: quoteService,
	}
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
		SourceMsgID:              req.SourceMsgId,
		SourceChannelID:          req.SourceChannelId,
		SourceMsgAuthorDiscordID: req.SourceMsgAuthorDiscordId,
	}

	created, err := h.quoteService.CreateQuote(ctx, quote)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create quote: %v", err)
	}

	return quoteToProto(created), nil
}

// GetQuote retrieves a quote by ID
func (h *QuoteHandler) GetQuote(ctx context.Context, req *quotespb.GetQuoteRequest) (*quotespb.Quote, error) {
	_, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "user context not found")
	}

	quote, err := h.quoteService.GetQuote(ctx, req.Id)
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

	// Get existing quote to verify ownership
	existing, err := h.quoteService.GetQuote(ctx, req.Id)
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

// ListQuotes lists quotes in a guild
func (h *QuoteHandler) ListQuotes(ctx context.Context, req *quotespb.ListQuotesRequest) (*quotespb.ListQuotesResponse, error) {
	_, err := interceptors.GetUserFromContext(ctx)
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

	quotes, total, err := h.quoteService.ListQuotes(ctx, req.GuildId, req.SourceMsgAuthorDiscordId, req.Tags, limit, int(req.Offset), orderBy, req.Ascending)
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
	_, err := interceptors.GetUserFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "user context not found")
	}

	limit := int(req.Limit)
	if limit == 0 {
		limit = 20
	}

	quotes, total, err := h.quoteService.SearchQuotes(ctx, req.GuildId, req.Query, req.Tags, limit, int(req.Offset))
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
	return &quotespb.Quote{
		Id:                       quote.ID,
		Body:                     quote.Body,
		Tags:                     quote.Tags,
		AuthorId:                 quote.AuthorID,
		GuildId:                  quote.GuildID,
		GuildName:                quote.GuildName,
		SourceMsgId:              quote.SourceMsgID,
		SourceChannelId:          quote.SourceChannelID,
		SourceMsgAuthorDiscordId: quote.SourceMsgAuthorDiscordID,
		CreatedAt:                timestamppb.New(quote.CreatedAt),
	}
}
