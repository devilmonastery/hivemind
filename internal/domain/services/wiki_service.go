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
	titles []struct {
		ID    string
		Title string
		Slug  string
	}
	expiresAt time.Time
}

// WikiService handles business logic for wiki pages
type WikiService struct {
	wikiRepo       repositories.WikiPageRepository
	wikiRefRepo    repositories.WikiMessageReferenceRepository
	wikiTitleRepo  repositories.WikiTitleRepository
	titlesCache    sync.Map // map[guildID]wikiTitlesCacheEntry
	titlesCacheTTL time.Duration
}

// NewWikiService creates a new wiki service
func NewWikiService(wikiRepo repositories.WikiPageRepository, wikiRefRepo repositories.WikiMessageReferenceRepository, wikiTitleRepo repositories.WikiTitleRepository) *WikiService {
	return &WikiService{
		wikiRepo:       wikiRepo,
		wikiRefRepo:    wikiRefRepo,
		wikiTitleRepo:  wikiTitleRepo,
		titlesCacheTTL: 1 * time.Minute,
	}
}

// CreateWikiPage creates a new wiki page
func (s *WikiService) CreateWikiPage(ctx context.Context, page *entities.WikiPage) (*entities.WikiPage, error) {
	// Check for duplicate title in guild - keeps explicit error message
	existing, err := s.wikiRepo.GetByGuildAndSlug(ctx, page.GuildID, page.Title)
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
	existing, err := s.wikiRepo.GetByGuildAndSlug(ctx, page.GuildID, page.Title)
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

// GetWikiPageByTitle retrieves a wiki page by guild ID and slug (normalized for lookup)
func (s *WikiService) GetWikiPageByTitle(ctx context.Context, guildID, slug string) (*entities.WikiPage, error) {
	page, err := s.wikiRepo.GetByGuildAndSlug(ctx, guildID, slug)
	if err != nil {
		return nil, fmt.Errorf("failed to get wiki page by title: %w", err)
	}
	return page, nil
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
func (s *WikiService) AutocompleteWikiTitles(ctx context.Context, guildID string) ([]struct {
	ID    string
	Title string
	Slug  string
}, error,
) {
	// Get all titles for the guild (with caching)
	titles, err := s.getWikiTitlesForGuild(ctx, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get wiki titles: %w", err)
	}

	return titles, nil
}

// getWikiTitlesForGuild retrieves wiki titles with caching
func (s *WikiService) getWikiTitlesForGuild(ctx context.Context, guildID string) ([]struct {
	ID    string
	Title string
	Slug  string
}, error,
) {
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

// MergeWikiPages merges source wiki page into target wiki page
// - Appends source body to target body (with separator)
// - Merges tags (union, deduplicated)
// - Transfers all message references from source to target
// - Soft-deletes source page
// - Creates alias title for source page pointing to target
// - Flattens any existing aliases pointing to source (redirects them to target)
// - Invalidates title cache for guild
func (s *WikiService) MergeWikiPages(ctx context.Context, sourcePageID, targetPageID, mergedByUserID string) (*entities.WikiPage, error) {
	// Fetch both pages
	sourcePage, err := s.wikiRepo.GetByID(ctx, sourcePageID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch source page: %w", err)
	}
	if sourcePage == nil {
		return nil, fmt.Errorf("source page not found: %s", sourcePageID)
	}

	targetPage, err := s.wikiRepo.GetByID(ctx, targetPageID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch target page: %w", err)
	}
	if targetPage == nil {
		return nil, fmt.Errorf("target page not found: %s", targetPageID)
	}

	// Validate: must be in same guild
	if sourcePage.GuildID != targetPage.GuildID {
		return nil, fmt.Errorf("cannot merge pages from different guilds")
	}

	// Validate: cannot merge page into itself
	if sourcePageID == targetPageID {
		return nil, fmt.Errorf("cannot merge page into itself")
	}

	// 1. Merge content: append source body to target body (with separator)
	separator := "\n\n---\n\n"
	targetPage.Body = targetPage.Body + separator + sourcePage.Body

	// 2. Merge tags: union of both sets (deduplicate)
	tagSet := make(map[string]bool)
	for _, tag := range targetPage.Tags {
		tagSet[tag] = true
	}
	for _, tag := range sourcePage.Tags {
		tagSet[tag] = true
	}
	mergedTags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		mergedTags = append(mergedTags, tag)
	}
	targetPage.Tags = mergedTags

	// 3. Update target page with merged content
	if err := s.wikiRepo.Update(ctx, targetPage); err != nil {
		return nil, fmt.Errorf("failed to update target page: %w", err)
	}

	// 4. Transfer all message references from source to target
	transferred, err := s.wikiRefRepo.TransferReferences(ctx, sourcePageID, targetPageID)
	if err != nil {
		return nil, fmt.Errorf("failed to transfer references: %w", err)
	}
	_ = transferred // Track for logging if needed

	// 5. Flatten aliases: Update all non-canonical titles pointing to source → point to target
	// This ensures no title chains: aliases always point directly to canonical page
	flattened, err := s.wikiTitleRepo.UpdatePageID(ctx, sourcePageID, targetPageID)
	if err != nil {
		return nil, fmt.Errorf("failed to flatten aliases: %w", err)
	}
	_ = flattened // Track for logging if needed

	// 6. Soft-delete source page
	if err := s.wikiRepo.Delete(ctx, sourcePageID); err != nil {
		return nil, fmt.Errorf("failed to delete source page: %w", err)
	}

	// 7. Invalidate title cache for guild
	s.titlesCache.Delete(sourcePage.GuildID)

	// Return merged target page
	return targetPage, nil
}
