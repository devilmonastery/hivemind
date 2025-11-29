package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
)

// wikiTitlesCacheEntry holds cached wiki titles for a guild
type wikiTitlesCacheEntry struct {
	titles    []struct{ ID, Title string }
	expiresAt time.Time
}

// WikiService handles business logic for wiki pages
type WikiService struct {
	wikiRepo       repositories.WikiPageRepository
	wikiRefRepo    repositories.WikiMessageReferenceRepository
	titlesCache    sync.Map // map[guildID]wikiTitlesCacheEntry
	titlesCacheTTL time.Duration
}

// NewWikiService creates a new wiki service
func NewWikiService(wikiRepo repositories.WikiPageRepository, wikiRefRepo repositories.WikiMessageReferenceRepository) *WikiService {
	return &WikiService{
		wikiRepo:       wikiRepo,
		wikiRefRepo:    wikiRefRepo,
		titlesCacheTTL: 1 * time.Minute,
	}
}

// CreateWikiPage creates a new wiki page
func (s *WikiService) CreateWikiPage(ctx context.Context, page *entities.WikiPage) (*entities.WikiPage, error) {
	// Check for duplicate title in guild - keeps explicit error message
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

	// Invalidate cache for this guild
	s.titlesCache.Delete(page.GuildID)

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

	// Invalidate cache for this guild
	s.titlesCache.Delete(page.GuildID)

	// Fetch updated page
	return s.wikiRepo.GetByID(ctx, page.ID)
}

// UpsertWikiPage creates a new wiki page or updates an existing one with the same title
func (s *WikiService) UpsertWikiPage(ctx context.Context, page *entities.WikiPage) (*entities.WikiPage, bool, error) {
	// Check if a page with this title already exists in the guild
	existing, err := s.wikiRepo.GetByGuildAndTitle(ctx, page.GuildID, page.Title)
	if err != nil {
		return nil, false, fmt.Errorf("failed to check for existing page: %w", err)
	}

	if existing != nil {
		// Update existing page
		page.ID = existing.ID
		page.CreatedAt = existing.CreatedAt
		page.AuthorID = existing.AuthorID

		if err := s.wikiRepo.Update(ctx, page); err != nil {
			return nil, false, fmt.Errorf("failed to update wiki page: %w", err)
		}

		// Invalidate cache for this guild
		s.titlesCache.Delete(page.GuildID)

		// Fetch updated page
		updated, err := s.wikiRepo.GetByID(ctx, page.ID)
		if err != nil {
			return nil, false, fmt.Errorf("failed to fetch updated page: %w", err)
		}
		return updated, false, nil
	}

	// Create new page
	if err := s.wikiRepo.Create(ctx, page); err != nil {
		return nil, false, fmt.Errorf("failed to create wiki page: %w", err)
	}

	// Invalidate cache for this guild
	s.titlesCache.Delete(page.GuildID)

	return page, true, nil
}

// DeleteWikiPage soft-deletes a wiki page
func (s *WikiService) DeleteWikiPage(ctx context.Context, id string) error {
	// Fetch the page to get its guild ID
	page, err := s.wikiRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get wiki page: %w", err)
	}

	if err := s.wikiRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete wiki page: %w", err)
	}

	// Invalidate cache for this guild
	s.titlesCache.Delete(page.GuildID)

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

// AddWikiMessageReference adds a Discord message reference to a wiki page
func (s *WikiService) AddWikiMessageReference(ctx context.Context, ref *entities.WikiMessageReference) error {
	if err := s.wikiRefRepo.Create(ctx, ref); err != nil {
		return fmt.Errorf("failed to add wiki message reference: %w", err)
	}
	return nil
}

// ListWikiMessageReferences retrieves all message references for a wiki page
func (s *WikiService) ListWikiMessageReferences(ctx context.Context, pageID string) ([]*entities.WikiMessageReference, error) {
	refs, err := s.wikiRefRepo.GetByPageID(ctx, pageID)
	if err != nil {
		return nil, fmt.Errorf("failed to list wiki message references: %w", err)
	}
	return refs, nil
}

// AutocompleteWikiTitles returns all wiki page titles for a guild (lightweight for autocomplete)
func (s *WikiService) AutocompleteWikiTitles(ctx context.Context, guildID string) ([]struct{ ID, Title string }, error) {
	// Get all titles for the guild (with caching)
	titles, err := s.getWikiTitlesForGuild(ctx, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get wiki titles: %w", err)
	}

	return titles, nil
}

// getWikiTitlesForGuild retrieves wiki titles with caching
func (s *WikiService) getWikiTitlesForGuild(ctx context.Context, guildID string) ([]struct{ ID, Title string }, error) {
	// Check cache
	if entry, ok := s.titlesCache.Load(guildID); ok {
		cached := entry.(wikiTitlesCacheEntry)
		if time.Now().Before(cached.expiresAt) {
			return cached.titles, nil
		}
		// Cache expired, remove it
		s.titlesCache.Delete(guildID)
	}

	// Cache miss or expired, fetch from database
	titles, err := s.wikiRepo.GetTitlesForGuild(ctx, guildID)
	if err != nil {
		return nil, err
	}

	// Store in cache
	s.titlesCache.Store(guildID, wikiTitlesCacheEntry{
		titles:    titles,
		expiresAt: time.Now().Add(s.titlesCacheTTL),
	})

	return titles, nil
}
