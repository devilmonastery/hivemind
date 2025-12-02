package postgres

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/gosimple/slug"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
	"github.com/devilmonastery/hivemind/internal/pkg/idgen"
)

type wikiTitleRepository struct {
	db  *sql.DB
	log *slog.Logger
}

// NewWikiTitleRepository creates a new PostgreSQL wiki title repository
func NewWikiTitleRepository(db *sql.DB) repositories.WikiTitleRepository {
	return &wikiTitleRepository{
		db:  db,
		log: slog.Default().With(slog.String("repo", "wiki_title")),
	}
}

func (r *wikiTitleRepository) Create(ctx context.Context, title *entities.WikiTitle) error {
	if title.ID == "" {
		title.ID = idgen.GenerateID()
	}
	if title.CreatedAt.IsZero() {
		title.CreatedAt = time.Now()
	}

	r.log.Debug("creating wiki title",
		slog.String("display_title", title.DisplayTitle),
		slog.String("page_slug", title.PageSlug),
		slog.String("page_id", title.PageID),
		slog.Bool("is_canonical", title.IsCanonical))

	query := `
		INSERT INTO wiki_titles (id, guild_id, display_title, page_slug, page_id, is_canonical, created_at, created_by_user_id, created_by_merge)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.ExecContext(ctx, query,
		title.ID,
		title.GuildID,
		title.DisplayTitle,
		title.PageSlug,
		title.PageID,
		title.IsCanonical,
		title.CreatedAt,
		nullString(title.CreatedByUserID),
		title.CreatedByMerge,
	)
	return err
}

func (r *wikiTitleRepository) GetByGuildAndSlug(ctx context.Context, guildID, inputSlug string) (*entities.WikiTitle, error) {
	// Normalize slug for lookup
	normalizedSlug := slug.Make(inputSlug)

	r.log.Debug("getting wiki title by guild and slug",
		slog.String("guild_id", guildID),
		slog.String("input_slug", inputSlug),
		slog.String("normalized_slug", normalizedSlug))

	query := `
		SELECT id, guild_id, display_title, page_slug, page_id, is_canonical, created_at, created_by_user_id, created_by_merge
		FROM wiki_titles
		WHERE guild_id = $1 AND page_slug = $2
	`

	var wt entities.WikiTitle
	var createdByUserID sql.NullString

	err := r.db.QueryRowContext(ctx, query, guildID, normalizedSlug).Scan(
		&wt.ID,
		&wt.GuildID,
		&wt.DisplayTitle,
		&wt.PageSlug,
		&wt.PageID,
		&wt.IsCanonical,
		&wt.CreatedAt,
		&createdByUserID,
		&wt.CreatedByMerge,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Not found is not an error for this method
	}
	if err != nil {
		return nil, err
	}

	wt.CreatedByUserID = createdByUserID.String

	return &wt, nil
}

func (r *wikiTitleRepository) GetCanonicalTitle(ctx context.Context, pageID string) (*entities.WikiTitle, error) {
	r.log.Debug("getting canonical title", slog.String("page_id", pageID))

	query := `
		SELECT id, guild_id, display_title, page_slug, page_id, is_canonical, created_at, created_by_user_id, created_by_merge
		FROM wiki_titles
		WHERE page_id = $1 AND is_canonical = TRUE
	`

	var wt entities.WikiTitle
	var createdByUserID sql.NullString

	err := r.db.QueryRowContext(ctx, query, pageID).Scan(
		&wt.ID,
		&wt.GuildID,
		&wt.DisplayTitle,
		&wt.PageSlug,
		&wt.PageID,
		&wt.IsCanonical,
		&wt.CreatedAt,
		&createdByUserID,
		&wt.CreatedByMerge,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Not found is not an error for this method
	}
	if err != nil {
		return nil, err
	}

	wt.CreatedByUserID = createdByUserID.String

	return &wt, nil
}

func (r *wikiTitleRepository) ListByPageID(ctx context.Context, pageID string) ([]*entities.WikiTitle, error) {
	r.log.Debug("listing titles by page id", slog.String("page_id", pageID))

	query := `
		SELECT id, guild_id, display_title, page_slug, page_id, is_canonical, created_at, created_by_user_id, created_by_merge
		FROM wiki_titles
		WHERE page_id = $1
		ORDER BY is_canonical DESC, created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var titles []*entities.WikiTitle
	for rows.Next() {
		var wt entities.WikiTitle
		var createdByUserID sql.NullString

		if err := rows.Scan(
			&wt.ID,
			&wt.GuildID,
			&wt.DisplayTitle,
			&wt.PageSlug,
			&wt.PageID,
			&wt.IsCanonical,
			&wt.CreatedAt,
			&createdByUserID,
			&wt.CreatedByMerge,
		); err != nil {
			return nil, err
		}

		wt.CreatedByUserID = createdByUserID.String
		titles = append(titles, &wt)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return titles, nil
}

func (r *wikiTitleRepository) UpdatePageID(ctx context.Context, oldPageID, newPageID string) (int, error) {
	r.log.Debug("updating page id for non-canonical titles",
		slog.String("old_page_id", oldPageID),
		slog.String("new_page_id", newPageID))

	// Update all non-canonical titles pointing to old page (flattening)
	query := `
		UPDATE wiki_titles
		SET page_id = $2
		WHERE page_id = $1 AND is_canonical = FALSE
	`

	result, err := r.db.ExecContext(ctx, query, oldPageID, newPageID)
	if err != nil {
		return 0, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(rows), nil
}

func (r *wikiTitleRepository) ConvertToAlias(ctx context.Context, oldPageID, newPageID string) (int, error) {
	r.log.Debug("converting canonical title to alias",
		slog.String("old_page_id", oldPageID),
		slog.String("new_page_id", newPageID))

	// Convert canonical title to alias pointing to new page
	// This is used during merge to redirect the old page name to the merged page
	query := `
		UPDATE wiki_titles
		SET page_id = $2, is_canonical = FALSE
		WHERE page_id = $1 AND is_canonical = TRUE
	`

	result, err := r.db.ExecContext(ctx, query, oldPageID, newPageID)
	if err != nil {
		return 0, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(rows), nil
}
