package services

import (
	"context"
	"fmt"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
)

// WikiService handles business logic for wiki pages
type WikiService struct {
	wikiRepo repositories.WikiPageRepository
}

// NewWikiService creates a new wiki service
func NewWikiService(wikiRepo repositories.WikiPageRepository) *WikiService {
	return &WikiService{
		wikiRepo: wikiRepo,
	}
}

// CreateWikiPage creates a new wiki page
func (s *WikiService) CreateWikiPage(ctx context.Context, page *entities.WikiPage) (*entities.WikiPage, error) {
	// Check for duplicate title in guild
	existing, err := s.wikiRepo.GetByGuildAndTitle(ctx, page.GuildID, page.Title)
	if err != nil {
		return nil, fmt.Errorf("failed to check for duplicate: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("wiki page with title '%s' already exists in this guild", page.Title)
	}

	// Create the page
	if err := s.wikiRepo.Create(ctx, page); err != nil {
		return nil, fmt.Errorf("failed to create wiki page: %w", err)
	}

	return page, nil
}

// GetWikiPage retrieves a wiki page by ID
func (s *WikiService) GetWikiPage(ctx context.Context, id string) (*entities.WikiPage, error) {
	page, err := s.wikiRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get wiki page: %w", err)
	}
	return page, nil
}

// UpdateWikiPage updates an existing wiki page
func (s *WikiService) UpdateWikiPage(ctx context.Context, page *entities.WikiPage) (*entities.WikiPage, error) {
	if err := s.wikiRepo.Update(ctx, page); err != nil {
		return nil, fmt.Errorf("failed to update wiki page: %w", err)
	}

	// Fetch updated page
	return s.wikiRepo.GetByID(ctx, page.ID)
}

// DeleteWikiPage soft-deletes a wiki page
func (s *WikiService) DeleteWikiPage(ctx context.Context, id string) error {
	if err := s.wikiRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete wiki page: %w", err)
	}
	return nil
}

// ListWikiPages lists wiki pages in a guild
func (s *WikiService) ListWikiPages(ctx context.Context, guildID string, limit, offset int, orderBy string, ascending bool) ([]*entities.WikiPage, int, error) {
	pages, total, err := s.wikiRepo.List(ctx, guildID, limit, offset, orderBy, ascending)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list wiki pages: %w", err)
	}
	return pages, total, nil
}

// SearchWikiPages searches wiki pages in a guild
func (s *WikiService) SearchWikiPages(ctx context.Context, guildID, query string, tags []string, limit, offset int) ([]*entities.WikiPage, int, error) {
	pages, total, err := s.wikiRepo.Search(ctx, guildID, query, tags, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search wiki pages: %w", err)
	}
	return pages, total, nil
}
