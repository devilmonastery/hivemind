package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
	"github.com/devilmonastery/hivemind/internal/pkg/idgen"
)

type wikiPageRepository struct {
	db *sql.DB
}

// NewWikiPageRepository creates a new PostgreSQL wiki page repository
func NewWikiPageRepository(db *sql.DB) repositories.WikiPageRepository {
	return &wikiPageRepository{db: db}
}

func (r *wikiPageRepository) Create(ctx context.Context, page *entities.WikiPage) error {
	if page.ID == "" {
		page.ID = idgen.GenerateID()
	}
	page.CreatedAt = time.Now()
	page.UpdatedAt = time.Now()

	query := `
		INSERT INTO wiki_pages (id, title, body, author_id, guild_id, channel_id, channel_name, tags, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.ExecContext(ctx, query,
		page.ID, page.Title, page.Body, page.AuthorID, page.GuildID,
		nullString(page.ChannelID), "", pq.Array(page.Tags),
		page.CreatedAt, page.UpdatedAt,
	)
	return err
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
	if deletedAt.Valid {
		page.DeletedAt = &deletedAt.Time
	}

	return page, nil
}

func (r *wikiPageRepository) GetByGuildAndTitle(ctx context.Context, guildID, title string) (*entities.WikiPage, error) {
	query := `
		SELECT id, title, body, author_id, guild_id, channel_id, tags, created_at, updated_at, deleted_at
		FROM wiki_pages
		WHERE guild_id = $1 AND LOWER(title) = LOWER($2) AND deleted_at IS NULL
	`
	page := &entities.WikiPage{}
	var tags pq.StringArray
	var channelID sql.NullString
	var deletedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, guildID, title).Scan(
		&page.ID, &page.Title, &page.Body, &page.AuthorID, &page.GuildID,
		&channelID, &tags, &page.CreatedAt, &page.UpdatedAt, &deletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil // Not found is not an error for this method
	}
	if err != nil {
		return nil, err
	}

	page.ChannelID = channelID.String
	page.Tags = tags
	if deletedAt.Valid {
		page.DeletedAt = &deletedAt.Time
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

	// Get total count
	var total int
	countQuery := `SELECT COUNT(*) FROM wiki_pages WHERE guild_id = $1 AND deleted_at IS NULL`
	err := r.db.QueryRowContext(ctx, countQuery, guildID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get pages
	query := fmt.Sprintf(`
		SELECT id, title, body, author_id, guild_id, channel_id, tags, created_at, updated_at
		FROM wiki_pages
		WHERE guild_id = $1 AND deleted_at IS NULL
		ORDER BY %s %s
		LIMIT $2 OFFSET $3
	`, orderBy, direction)

	rows, err := r.db.QueryContext(ctx, query, guildID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	pages := []*entities.WikiPage{}
	for rows.Next() {
		page := &entities.WikiPage{}
		var tags pq.StringArray
		var channelID sql.NullString

		err := rows.Scan(
			&page.ID, &page.Title, &page.Body, &page.AuthorID, &page.GuildID,
			&channelID, &tags, &page.CreatedAt, &page.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		page.ChannelID = channelID.String
		page.Tags = tags
		pages = append(pages, page)
	}

	return pages, total, nil
}

func (r *wikiPageRepository) Search(ctx context.Context, guildID, query string, tags []string, limit, offset int) ([]*entities.WikiPage, int, error) {
	if limit <= 0 {
		limit = 10
	}

	// Build search conditions
	conditions := []string{"guild_id = $1", "deleted_at IS NULL"}
	args := []interface{}{guildID}
	argCount := 1

	// Full-text search on title and body
	if query != "" {
		argCount++
		conditions = append(conditions, fmt.Sprintf("(to_tsvector('english', title || ' ' || body) @@ plainto_tsquery('english', $%d))", argCount))
		args = append(args, query)
	}

	// Tag filtering
	if len(tags) > 0 {
		argCount++
		conditions = append(conditions, fmt.Sprintf("tags && $%d", argCount))
		args = append(args, pq.Array(tags))
	}

	whereClause := strings.Join(conditions, " AND ")

	// Get total count
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM wiki_pages WHERE %s", whereClause)
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get pages with ranking
	searchQuery := fmt.Sprintf(`
		SELECT id, title, body, author_id, guild_id, channel_id, tags, created_at, updated_at
		FROM wiki_pages
		WHERE %s
		ORDER BY ts_rank(to_tsvector('english', title || ' ' || body), plainto_tsquery('english', $%d)) DESC
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

		err := rows.Scan(
			&page.ID, &page.Title, &page.Body, &page.AuthorID, &page.GuildID,
			&channelID, &tagArray, &page.CreatedAt, &page.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		page.ChannelID = channelID.String
		page.Tags = tagArray
		pages = append(pages, page)
	}

	return pages, total, nil
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}
