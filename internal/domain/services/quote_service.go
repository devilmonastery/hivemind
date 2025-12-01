package services

import (
	"context"
	"fmt"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
)

// QuoteService handles business logic for quotes
type QuoteService struct {
	quoteRepo repositories.QuoteRepository
}

// NewQuoteService creates a new quote service
func NewQuoteService(quoteRepo repositories.QuoteRepository) *QuoteService {
	return &QuoteService{
		quoteRepo: quoteRepo,
	}
}

// CreateQuote creates a new quote
func (s *QuoteService) CreateQuote(ctx context.Context, quote *entities.Quote) (*entities.Quote, error) {
	if err := s.quoteRepo.Create(ctx, quote); err != nil {
		return nil, fmt.Errorf("failed to create quote: %w", err)
	}
	return quote, nil
}

// GetQuote retrieves a quote by ID
func (s *QuoteService) GetQuote(ctx context.Context, id string, userDiscordID string) (*entities.Quote, error) {
	quote, err := s.quoteRepo.GetByID(ctx, id, userDiscordID)
	if err != nil {
		return nil, fmt.Errorf("failed to get quote: %w", err)
	}
	return quote, nil
}

// DeleteQuote soft-deletes a quote
func (s *QuoteService) DeleteQuote(ctx context.Context, id string) error {
	if err := s.quoteRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete quote: %w", err)
	}
	return nil
}

// UpdateQuote updates a quote's body and tags
func (s *QuoteService) UpdateQuote(ctx context.Context, id, body string, tags []string, userDiscordID string) (*entities.Quote, error) {
	if err := s.quoteRepo.Update(ctx, id, body, tags); err != nil {
		return nil, fmt.Errorf("failed to update quote: %w", err)
	}

	// Fetch and return the updated quote
	quote, err := s.quoteRepo.GetByID(ctx, id, userDiscordID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated quote: %w", err)
	}
	return quote, nil
}

// ListQuotes lists quotes in a guild
func (s *QuoteService) ListQuotes(ctx context.Context, guildID string, limit, offset int, orderBy string, ascending bool, userDiscordID string) ([]*entities.Quote, int, error) {
	quotes, total, err := s.quoteRepo.List(ctx, guildID, limit, offset, orderBy, ascending, userDiscordID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list quotes: %w", err)
	}
	return quotes, total, nil
}

// SearchQuotes searches quotes by full-text query
func (s *QuoteService) SearchQuotes(ctx context.Context, guildID, query string, tags []string, limit, offset int, userDiscordID string) ([]*entities.Quote, int, error) {
	quotes, total, err := s.quoteRepo.Search(ctx, guildID, query, tags, limit, offset, userDiscordID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search quotes: %w", err)
	}
	return quotes, total, nil
}

// GetRandomQuote retrieves a random quote from a guild
func (s *QuoteService) GetRandomQuote(ctx context.Context, guildID string, tags []string) (*entities.Quote, error) {
	quote, err := s.quoteRepo.GetRandom(ctx, guildID, tags)
	if err != nil {
		return nil, fmt.Errorf("failed to get random quote: %w", err)
	}
	return quote, nil
}
