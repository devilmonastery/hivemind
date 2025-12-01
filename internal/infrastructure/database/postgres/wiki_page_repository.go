package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/gosimple/slug"
	"github.com/lib/pq"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
	"github.com/devilmonastery/hivemind/internal/pkg/idgen"
)

type wikiPageRepository struct {
	db        *sql.DB
	titleRepo repositories.WikiTitleRepository
}

// NewWikiPageRepository creates a new PostgreSQL wiki page repository
func NewWikiPageRepository(db *sql.DB, titleRepo repositories.WikiTitleRepository) repositories.WikiPageRepository {
	return &wikiPageRepository{
		db:        db,
		titleRepo: titleRepo,
	}
}

func (r *wikiPageRepository) Create(ctx context.Context, page *entities.WikiPage) error {
	if page.ID == "" {
		page.ID = idgen.GenerateID()
	}
	page.CreatedAt = time.Now()
	page.UpdatedAt = time.Now()

	// Generate slug from title
	page.Slug = slug.Make(page.Title)

	// Create the page
	query := `
		INSERT INTO wiki_pages (id, title, body, author_id, guild_id, channel_id, channel_name, tags, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.ExecContext(ctx, query,
		page.ID, page.Title, page.Body, page.AuthorID, page.GuildID,
		nullString(page.ChannelID), "", pq.Array(page.Tags),
		page.CreatedAt, page.UpdatedAt,
	)
	if err != nil {
		return err
	}

	// Create canonical wiki_title entry
	wikiTitle := &entities.WikiTitle{
		GuildID:      page.GuildID,
		DisplayTitle: page.Title,
		PageSlug:     page.Slug,
		PageID:       page.ID,
		IsCanonical:  true,
		CreatedAt:    page.CreatedAt,
	}

	return r.titleRepo.Create(ctx, wikiTitle)
}

func (r *wikiPageRepository) GetByID(ctx context.Context, id string) (*entities.WikiPage, error) {
	query := `
		SELECT id, title, body, author_id, guild_id, channel_id, tags, created_at, updated_at, deleted_at
		FROM wiki_pages
		WHERE id = $1 AND deleted_at IS NULL
	`
	page := &entities.WikiPage{}
	var tags pq.StringArray
	var channelID sql.NullString
	var deletedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&page.ID, &page.Title, &page.Body, &page.AuthorID, &page.GuildID,
		&channelID, &tags, &page.CreatedAt, &page.UpdatedAt, &deletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("wiki page not found: %s", id)
	}
	if err != nil {
		return nil, err
	}

	page.ChannelID = channelID.String
	page.Tags = tags
	page.Slug = slug.Make(page.Title)
	if deletedAt.Valid {
		page.DeletedAt = &deletedAt.Time
	}

	return page, nil
}

func (r *wikiPageRepository) GetByGuildAndSlug(ctx context.Context, guildID, pageSlug string) (*entities.WikiPage, error) {
	// Normalize slug for lookup
	titleSlug := slug.Make(pageSlug)

	// Look up the title (canonical or alias) in wiki_titles
	wikiTitle, err := r.titleRepo.GetByGuildAndSlug(ctx, guildID, titleSlug)
	if err != nil {
		return nil, err
	}
	if wikiTitle == nil {
		return nil, nil // Title not found
	}

	// Get the page by ID
	page, err := r.GetByID(ctx, wikiTitle.PageID)
	if err != nil {
		return nil, err
	}

	// Populate the Title field with the display_title for backward compatibility
	if page != nil {
		page.Title = wikiTitle.DisplayTitle
		page.Slug = wikiTitle.PageSlug
	}

	return page, nil
}

func (r *wikiPageRepository) Update(ctx context.Context, page *entities.WikiPage) error {
	page.UpdatedAt = time.Now()

	query := `
		UPDATE wiki_pages
		SET title = $2, body = $3, tags = $4, updated_at = $5
		WHERE id = $1 AND deleted_at IS NULL
	`
	result, err := r.db.ExecContext(ctx, query,
		page.ID, page.Title, page.Body, pq.Array(page.Tags), page.UpdatedAt,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("wiki page not found: %s", page.ID)
	}

	return nil
}

func (r *wikiPageRepository) Delete(ctx context.Context, id string) error {
	query := `
		UPDATE wiki_pages
		SET deleted_at = $2
		WHERE id = $1 AND deleted_at IS NULL
	`
	result, err := r.db.ExecContext(ctx, query, id, time.Now())
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("wiki page not found: %s", id)
	}

	return nil
}

func (r *wikiPageRepository) List(ctx context.Context, guildID string, limit, offset int, orderBy string, ascending bool) ([]*entities.WikiPage, int, error) {
	if limit <= 0 {
		limit = 50
	}

	// Validate orderBy
	validOrderBy := map[string]bool{
		"created_at": true,
		"updated_at": true,
		"title":      true,
	}
	if !validOrderBy[orderBy] {
		orderBy = "created_at"
	}

	direction := "DESC"
	if ascending {
		direction = "ASC"
	}

	// Build WHERE clause - if guildID is empty, get from all guilds
	whereClause := "wp.deleted_at IS NULL"
	args := []interface{}{}
	if guildID != "" {
		whereClause = "wp.guild_id = $1 AND wp.deleted_at IS NULL"
		args = append(args, guildID)
	}

	// Get total count
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM wiki_pages wp WHERE %s", whereClause)
	var err error
	if guildID != "" {
		err = r.db.QueryRowContext(ctx, countQuery, guildID).Scan(&total)
	} else {
		err = r.db.QueryRowContext(ctx, countQuery).Scan(&total)
	}
	if err != nil {
		return nil, 0, err
	}

	// Get pages with canonical slug from wiki_titles
	query := fmt.Sprintf(`
		SELECT wp.id, wt.display_title, wp.body, wp.author_id, wp.guild_id, dg.guild_name, wp.channel_id, wp.tags, wp.created_at, wp.updated_at, wt.page_slug
		FROM wiki_pages wp
		LEFT JOIN discord_guilds dg ON wp.guild_id = dg.guild_id
		LEFT JOIN wiki_titles wt ON wp.id = wt.page_id AND wt.is_canonical = TRUE
		WHERE %s
		ORDER BY wp.%s %s
		LIMIT $%d OFFSET $%d
	`, whereClause, orderBy, direction, len(args)+1, len(args)+2)

	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	pages := []*entities.WikiPage{}
	for rows.Next() {
		page := &entities.WikiPage{}
		var tags pq.StringArray
		var guildName, channelID sql.NullString
		var pageSlug sql.NullString

		err := rows.Scan(
			&page.ID, &page.Title, &page.Body, &page.AuthorID, &page.GuildID, &guildName,
			&channelID, &tags, &page.CreatedAt, &page.UpdatedAt, &pageSlug,
		)
		if err != nil {
			return nil, 0, err
		}

		page.GuildName = guildName.String
		page.ChannelID = channelID.String
		page.Tags = tags
		if pageSlug.Valid {
			page.Slug = pageSlug.String
		} else {
			// Fallback if wiki_titles entry missing
			page.Slug = slug.Make(page.Title)
		}
		pages = append(pages, page)
	}

	return pages, total, nil
}

func (r *wikiPageRepository) Search(ctx context.Context, guildID, query string, tags []string, limit, offset int) ([]*entities.WikiPage, int, error) {
	if limit <= 0 {
		limit = 10
	}

	// Build search conditions
	conditions := []string{"wp.guild_id = $1", "wp.deleted_at IS NULL"}
	args := []interface{}{guildID}
	argCount := 1

	// Full-text search on title and body
	if query != "" {
		argCount++
		// Use hybrid search vector: searches both english and simple dictionaries
		conditions = append(conditions, fmt.Sprintf("(wp.search_vector @@ websearch_to_tsquery('english', $%d) OR wp.search_vector @@ websearch_to_tsquery('simple', $%d))", argCount, argCount))
		args = append(args, query)
	}

	// Tag filtering
	if len(tags) > 0 {
		argCount++
		conditions = append(conditions, fmt.Sprintf("wp.tags && $%d", argCount))
		args = append(args, pq.Array(tags))
	}

	whereClause := strings.Join(conditions, " AND ")

	// Get total count
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM wiki_pages wp WHERE %s", whereClause)
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get pages with ranking and canonical slug from wiki_titles
	searchQuery := fmt.Sprintf(`
		SELECT wp.id, wt.display_title, wp.body, wp.author_id, wp.guild_id, dg.guild_name, wp.channel_id, wp.tags, wp.created_at, wp.updated_at, wt.page_slug
		FROM wiki_pages wp
		LEFT JOIN discord_guilds dg ON wp.guild_id = dg.guild_id
		LEFT JOIN wiki_titles wt ON wp.id = wt.page_id AND wt.is_canonical = TRUE
		WHERE %s
		ORDER BY ts_rank(wp.search_vector, websearch_to_tsquery('english', $%d)) DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argCount, argCount+1, argCount+2)

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, searchQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	pages := []*entities.WikiPage{}
	for rows.Next() {
		page := &entities.WikiPage{}
		var tagArray pq.StringArray
		var channelID sql.NullString
		var guildName sql.NullString
		var pageSlug sql.NullString

		err := rows.Scan(
			&page.ID, &page.Title, &page.Body, &page.AuthorID, &page.GuildID,
			&guildName, &channelID, &tagArray, &page.CreatedAt, &page.UpdatedAt, &pageSlug,
		)
		if err != nil {
			return nil, 0, err
		}

		page.ChannelID = channelID.String
		page.GuildName = guildName.String
		page.Tags = tagArray
		if pageSlug.Valid {
			page.Slug = pageSlug.String
		} else {
			// Fallback if wiki_titles entry missing
			page.Slug = slug.Make(page.Title)
		}
		pages = append(pages, page)
	}

	return pages, total, nil
}

// GetTitlesForGuild returns only ID, Title, and Slug for all pages in a guild (lightweight for autocomplete)
func (r *wikiPageRepository) GetTitlesForGuild(ctx context.Context, guildID string) ([]struct {
	ID    string
	Title string
	Slug  string
}, error,
) {
	query := `
		SELECT wp.id, wt.display_title, wt.page_slug
		FROM wiki_pages wp
		LEFT JOIN wiki_titles wt ON wp.id = wt.page_id AND wt.is_canonical = TRUE
		WHERE wp.guild_id = $1 AND wp.deleted_at IS NULL
		ORDER BY wp.updated_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var titles []struct {
		ID    string
		Title string
		Slug  string
	}
	for rows.Next() {
		var t struct {
			ID    string
			Title string
			Slug  string
		}
		var titleStr, slugStr sql.NullString
		if err := rows.Scan(&t.ID, &titleStr, &slugStr); err != nil {
			return nil, err
		}
		t.Title = titleStr.String
		if slugStr.Valid {
			t.Slug = slugStr.String
		} else {
			// Fallback if wiki_titles entry missing
			t.Slug = slug.Make(t.Title)
		}
		titles = append(titles, t)
	}

	return titles, rows.Err()
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}
