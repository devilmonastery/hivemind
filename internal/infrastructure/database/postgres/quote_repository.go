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

type quoteRepository struct {
	db *sql.DB
}

// NewQuoteRepository creates a new PostgreSQL quote repository
func NewQuoteRepository(db *sql.DB) repositories.QuoteRepository {
	return &quoteRepository{db: db}
}

func (r *quoteRepository) Create(ctx context.Context, quote *entities.Quote) error {
	if quote.ID == "" {
		quote.ID = idgen.GenerateID()
	}
	quote.CreatedAt = time.Now()

	query := `
		INSERT INTO quotes (id, body, author_id, guild_id, source_msg_id, source_channel_id, source_msg_author_discord_id, tags, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.ExecContext(ctx, query,
		quote.ID, quote.Body, quote.AuthorID, quote.GuildID,
		quote.SourceMsgID, quote.SourceChannelID, quote.SourceMsgAuthorDiscordID,
		pq.Array(quote.Tags), quote.CreatedAt,
	)
	return err
}

func (r *quoteRepository) GetByID(ctx context.Context, id string) (*entities.Quote, error) {
	query := `
		SELECT id, body, author_id, guild_id, source_msg_id, source_channel_id, source_msg_author_discord_id, tags, created_at, deleted_at
		FROM quotes
		WHERE id = $1 AND deleted_at IS NULL
	`
	quote := &entities.Quote{}
	var tags pq.StringArray
	var deletedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&quote.ID, &quote.Body, &quote.AuthorID, &quote.GuildID,
		&quote.SourceMsgID, &quote.SourceChannelID, &quote.SourceMsgAuthorDiscordID,
		&tags, &quote.CreatedAt, &deletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("quote not found: %s", id)
	}
	if err != nil {
		return nil, err
	}

	quote.Tags = tags
	if deletedAt.Valid {
		quote.DeletedAt = &deletedAt.Time
	}

	return quote, nil
}

func (r *quoteRepository) Delete(ctx context.Context, id string) error {
	query := `
		UPDATE quotes
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
		return fmt.Errorf("quote not found: %s", id)
	}

	return nil
}

func (r *quoteRepository) List(ctx context.Context, guildID string, authorDiscordID string, tags []string, limit, offset int, orderBy string, ascending bool) ([]*entities.Quote, int, error) {
	if limit <= 0 {
		limit = 50
	}

	// Build WHERE conditions
	conditions := []string{"guild_id = $1", "deleted_at IS NULL"}
	args := []interface{}{guildID}
	argCount := 1

	if authorDiscordID != "" {
		argCount++
		conditions = append(conditions, fmt.Sprintf("source_msg_author_discord_id = $%d", argCount))
		args = append(args, authorDiscordID)
	}

	if len(tags) > 0 {
		argCount++
		conditions = append(conditions, fmt.Sprintf("tags && $%d", argCount))
		args = append(args, pq.Array(tags))
	}

	whereClause := strings.Join(conditions, " AND ")

	// Validate orderBy
	validOrderBy := map[string]bool{
		"created_at": true,
		"random":     true,
	}
	if !validOrderBy[orderBy] {
		orderBy = "created_at"
	}

	direction := "DESC"
	if ascending {
		direction = "ASC"
	}

	orderClause := fmt.Sprintf("%s %s", orderBy, direction)
	if orderBy == "random" {
		orderClause = "RANDOM()"
	}

	// Get total count
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM quotes WHERE %s", whereClause)
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get quotes
	query := fmt.Sprintf(`
		SELECT id, body, author_id, guild_id, source_msg_id, source_channel_id, source_msg_author_discord_id, tags, created_at
		FROM quotes
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, whereClause, orderClause, argCount+1, argCount+2)

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	quotes := []*entities.Quote{}
	for rows.Next() {
		quote := &entities.Quote{}
		var tags pq.StringArray

		err := rows.Scan(
			&quote.ID, &quote.Body, &quote.AuthorID, &quote.GuildID,
			&quote.SourceMsgID, &quote.SourceChannelID, &quote.SourceMsgAuthorDiscordID,
			&tags, &quote.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		quote.Tags = tags
		quotes = append(quotes, quote)
	}

	return quotes, total, nil
}

func (r *quoteRepository) Search(ctx context.Context, guildID, query string, tags []string, limit, offset int) ([]*entities.Quote, int, error) {
	if limit <= 0 {
		limit = 10
	}

	// Build search conditions
	conditions := []string{"guild_id = $1", "deleted_at IS NULL"}
	args := []interface{}{guildID}
	argCount := 1

	// Full-text search on body
	if query != "" {
		argCount++
		conditions = append(conditions, fmt.Sprintf("(to_tsvector('english', body) @@ plainto_tsquery('english', $%d))", argCount))
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
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM quotes WHERE %s", whereClause)
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get quotes with ranking
	var searchQuery string
	if query != "" {
		searchQuery = fmt.Sprintf(`
			SELECT id, body, author_id, guild_id, source_msg_id, source_channel_id, source_msg_author_discord_id, tags, created_at
			FROM quotes
			WHERE %s
			ORDER BY ts_rank(to_tsvector('english', body), plainto_tsquery('english', $2)) DESC
			LIMIT $%d OFFSET $%d
		`, whereClause, argCount+1, argCount+2)
	} else {
		searchQuery = fmt.Sprintf(`
			SELECT id, body, author_id, guild_id, source_msg_id, source_channel_id, source_msg_author_discord_id, tags, created_at
			FROM quotes
			WHERE %s
			ORDER BY created_at DESC
			LIMIT $%d OFFSET $%d
		`, whereClause, argCount+1, argCount+2)
	}

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, searchQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	quotes := []*entities.Quote{}
	for rows.Next() {
		quote := &entities.Quote{}
		var tagArray pq.StringArray

		err := rows.Scan(
			&quote.ID, &quote.Body, &quote.AuthorID, &quote.GuildID,
			&quote.SourceMsgID, &quote.SourceChannelID, &quote.SourceMsgAuthorDiscordID,
			&tagArray, &quote.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		quote.Tags = tagArray
		quotes = append(quotes, quote)
	}

	return quotes, total, nil
}

func (r *quoteRepository) GetRandom(ctx context.Context, guildID string, tags []string) (*entities.Quote, error) {
	// Build WHERE conditions
	conditions := []string{"guild_id = $1", "deleted_at IS NULL"}
	args := []interface{}{guildID}
	argCount := 1

	if len(tags) > 0 {
		argCount++
		conditions = append(conditions, fmt.Sprintf("tags && $%d", argCount))
		args = append(args, pq.Array(tags))
	}

	whereClause := strings.Join(conditions, " AND ")

	query := fmt.Sprintf(`
		SELECT id, body, author_id, guild_id, source_msg_id, source_channel_id, source_msg_author_discord_id, tags, created_at
		FROM quotes
		WHERE %s
		ORDER BY RANDOM()
		LIMIT 1
	`, whereClause)

	quote := &entities.Quote{}
	var tagArray pq.StringArray

	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&quote.ID, &quote.Body, &quote.AuthorID, &quote.GuildID,
		&quote.SourceMsgID, &quote.SourceChannelID, &quote.SourceMsgAuthorDiscordID,
		&tagArray, &quote.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no quotes found")
	}
	if err != nil {
		return nil, err
	}

	quote.Tags = tagArray
	return quote, nil
}
