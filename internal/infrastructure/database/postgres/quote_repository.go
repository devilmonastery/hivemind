package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/lib/pq"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
	"github.com/devilmonastery/hivemind/internal/pkg/idgen"
)

type quoteRepository struct {
	db  *sql.DB
	log *slog.Logger
}

// NewQuoteRepository creates a new PostgreSQL quote repository
func NewQuoteRepository(db *sql.DB) repositories.QuoteRepository {
	return &quoteRepository{
		db:  db,
		log: slog.Default().With(slog.String("repo", "quote")),
	}
}

func (r *quoteRepository) Create(ctx context.Context, quote *entities.Quote) error {
	if quote.ID == "" {
		quote.ID = idgen.GenerateID()
	}
	quote.CreatedAt = time.Now()

	query := `
		INSERT INTO quotes (id, body, author_id, author_discord_id, guild_id, source_msg_id, source_channel_id, source_channel_name, source_msg_author_discord_id, source_msg_author_username, source_msg_timestamp, tags, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err := r.db.ExecContext(ctx, query,
		quote.ID, quote.Body, quote.AuthorID, quote.AuthorDiscordID, quote.GuildID,
		quote.SourceMsgID, quote.SourceChannelID, quote.SourceChannelName, quote.SourceMsgAuthorDiscordID,
		quote.SourceMsgAuthorUsername, quote.SourceMsgTimestamp, pq.Array(quote.Tags), quote.CreatedAt,
	)
	return err
}

func (r *quoteRepository) GetByID(ctx context.Context, id string, userDiscordID string) (*entities.Quote, error) {
	query := `
		SELECT q.id, q.body, q.author_id, q.author_discord_id, u.name, q.guild_id, dg.guild_name,
		       q.source_msg_id, q.source_channel_id, q.source_channel_name,
		       q.source_msg_author_discord_id, q.source_msg_author_username, q.source_msg_timestamp, q.tags, q.created_at, q.deleted_at
		FROM quotes q
		LEFT JOIN discord_guilds dg ON q.guild_id = dg.guild_id
		LEFT JOIN users u ON q.author_id = u.id`

	// Add ACL check if userDiscordID provided (non-admin)
	if userDiscordID != "" {
		query += `
		INNER JOIN guild_members gm ON q.guild_id = gm.guild_id AND gm.discord_id = $2`
	}

	query += `
		WHERE q.id = $1 AND q.deleted_at IS NULL`

	quote := &entities.Quote{}
	var tags pq.StringArray
	var guildName, authorUsername, sourceChannelName, sourceMsgAuthorUsername sql.NullString
	var deletedAt sql.NullTime

	var authorDiscordID sql.NullString
	var sourceMsgTimestamp sql.NullTime
	var err error
	if userDiscordID != "" {
		err = r.db.QueryRowContext(ctx, query, id, userDiscordID).Scan(
			&quote.ID, &quote.Body, &quote.AuthorID, &authorDiscordID, &authorUsername, &quote.GuildID, &guildName,
			&quote.SourceMsgID, &quote.SourceChannelID, &sourceChannelName,
			&quote.SourceMsgAuthorDiscordID, &sourceMsgAuthorUsername,
			&sourceMsgTimestamp, &tags, &quote.CreatedAt, &deletedAt,
		)
	} else {
		err = r.db.QueryRowContext(ctx, query, id).Scan(
			&quote.ID, &quote.Body, &quote.AuthorID, &authorDiscordID, &authorUsername, &quote.GuildID, &guildName,
			&quote.SourceMsgID, &quote.SourceChannelID, &sourceChannelName,
			&quote.SourceMsgAuthorDiscordID, &sourceMsgAuthorUsername,
			&sourceMsgTimestamp, &tags, &quote.CreatedAt, &deletedAt,
		)
	}

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("quote not found: %s", id)
	}
	if err != nil {
		return nil, err
	}

	quote.AuthorDiscordID = authorDiscordID.String
	quote.GuildName = guildName.String
	quote.AuthorUsername = authorUsername.String
	quote.SourceChannelName = sourceChannelName.String
	quote.SourceMsgAuthorUsername = sourceMsgAuthorUsername.String
	if sourceMsgTimestamp.Valid {
		quote.SourceMsgTimestamp = sourceMsgTimestamp.Time
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

func (r *quoteRepository) Update(ctx context.Context, id, body string, tags []string) error {
	query := `
		UPDATE quotes
		SET body = $2, tags = $3
		WHERE id = $1 AND deleted_at IS NULL
	`
	result, err := r.db.ExecContext(ctx, query, id, body, pq.Array(tags))
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

func (r *quoteRepository) List(ctx context.Context, guildID string, limit, offset int, orderBy string, ascending bool, userDiscordID string) ([]*entities.Quote, int, error) {
	if limit <= 0 {
		limit = 50
	}

	// Build WHERE conditions and args first
	conditions := []string{"q.deleted_at IS NULL"}
	args := []interface{}{}
	argCount := 0

	if guildID != "" {
		argCount++
		conditions = append(conditions, fmt.Sprintf("q.guild_id = $%d", argCount))
		args = append(args, guildID)
	}

	// Build base query with JOINs
	baseFrom := `FROM quotes q
		LEFT JOIN discord_guilds dg ON q.guild_id = dg.guild_id
		LEFT JOIN users u ON q.author_id = u.id`

	// Add ACL check if userDiscordID provided (non-admin)
	if userDiscordID != "" {
		argCount++
		baseFrom += fmt.Sprintf(`
		INNER JOIN guild_members gm ON q.guild_id = gm.guild_id AND gm.discord_id = $%d`, argCount)
		args = append(args, userDiscordID)
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
	countQuery := fmt.Sprintf("SELECT COUNT(*) %s WHERE %s", baseFrom, whereClause)
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get quotes
	query := fmt.Sprintf(`
		SELECT q.id, q.body, q.author_id, q.author_discord_id, u.name, q.guild_id, dg.guild_name, 
		       q.source_msg_id, q.source_channel_id, q.source_channel_name,
		       q.source_msg_author_discord_id, q.source_msg_author_username, q.source_msg_timestamp, q.tags, q.created_at
		%s
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, baseFrom, whereClause, orderClause, argCount+1, argCount+2)

	r.log.Debug("executing select query",
		slog.String("query", query),
		slog.Any("args", args))

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
		var guildName, authorDiscordID, authorUsername, sourceChannelName, sourceMsgAuthorUsername sql.NullString
		var sourceMsgTimestamp sql.NullTime

		err := rows.Scan(
			&quote.ID, &quote.Body, &quote.AuthorID, &authorDiscordID, &authorUsername, &quote.GuildID, &guildName,
			&quote.SourceMsgID, &quote.SourceChannelID, &sourceChannelName,
			&quote.SourceMsgAuthorDiscordID, &sourceMsgAuthorUsername,
			&sourceMsgTimestamp, &tags, &quote.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		quote.AuthorDiscordID = authorDiscordID.String
		quote.GuildName = guildName.String
		quote.AuthorUsername = authorUsername.String
		quote.SourceChannelName = sourceChannelName.String
		quote.SourceMsgAuthorUsername = sourceMsgAuthorUsername.String
		if sourceMsgTimestamp.Valid {
			quote.SourceMsgTimestamp = sourceMsgTimestamp.Time
		}
		quote.Tags = tags
		quote.SourceMsgAuthorUsername = sourceMsgAuthorUsername.String
		quote.Tags = tags
		quotes = append(quotes, quote)
	}

	return quotes, total, nil
}

func (r *quoteRepository) Search(ctx context.Context, guildID, query string, tags []string, limit, offset int, userDiscordID string) ([]*entities.Quote, int, error) {
	if limit <= 0 {
		limit = 10
	}

	// Build search conditions and args first
	conditions := []string{"q.guild_id = $1", "q.deleted_at IS NULL"}
	args := []interface{}{guildID}
	argCount := 1

	// Build base FROM clause with JOINs
	baseFrom := `FROM quotes q
		LEFT JOIN discord_guilds dg ON q.guild_id = dg.guild_id
		LEFT JOIN users u ON q.author_id = u.id`

	// Add ACL check if userDiscordID provided (non-admin)
	if userDiscordID != "" {
		argCount++
		baseFrom += fmt.Sprintf(`
		INNER JOIN guild_members gm ON q.guild_id = gm.guild_id AND gm.discord_id = $%d`, argCount)
		args = append(args, userDiscordID)
	}

	// Full-text search on body
	var queryParamPos int
	if query != "" {
		argCount++
		queryParamPos = argCount
		// Use hybrid search vector: searches both english (stemmed, weight A) and simple (literal, weight B)
		conditions = append(conditions, fmt.Sprintf("(q.search_vector @@ websearch_to_tsquery('english', $%d) OR q.search_vector @@ websearch_to_tsquery('simple', $%d))", argCount, argCount))
		args = append(args, query)
	}

	// Tag filtering
	if len(tags) > 0 {
		argCount++
		conditions = append(conditions, fmt.Sprintf("q.tags && $%d", argCount))
		args = append(args, pq.Array(tags))
	}

	whereClause := strings.Join(conditions, " AND ")

	// Get total count
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) %s WHERE %s", baseFrom, whereClause)
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get quotes with ranking
	var searchQuery string
	if query != "" {
		searchQuery = fmt.Sprintf(`
			SELECT q.id, q.body, q.author_id, q.author_discord_id, u.name, q.guild_id, dg.guild_name,
			       q.source_msg_id, q.source_channel_id, q.source_channel_name,
			       q.source_msg_author_discord_id, q.source_msg_author_username, q.source_msg_timestamp, q.tags, q.created_at
			%s
			WHERE %s
			ORDER BY ts_rank(q.search_vector, websearch_to_tsquery('english', $%d)) DESC
			LIMIT $%d OFFSET $%d
		`, baseFrom, whereClause, queryParamPos, argCount+1, argCount+2)
	} else {
		searchQuery = fmt.Sprintf(`
			SELECT q.id, q.body, q.author_id, q.author_discord_id, u.name, q.guild_id, dg.guild_name,
			       q.source_msg_id, q.source_channel_id, q.source_channel_name,
			       q.source_msg_author_discord_id, q.source_msg_author_username, q.source_msg_timestamp, q.tags, q.created_at
			%s
			WHERE %s
			ORDER BY created_at DESC
			LIMIT $%d OFFSET $%d
		`, baseFrom, whereClause, argCount+1, argCount+2)
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
		var guildName, authorDiscordID, authorUsername, sourceChannelName, sourceMsgAuthorUsername sql.NullString
		var sourceMsgTimestamp sql.NullTime

		err := rows.Scan(
			&quote.ID, &quote.Body, &quote.AuthorID, &authorDiscordID, &authorUsername, &quote.GuildID, &guildName,
			&quote.SourceMsgID, &quote.SourceChannelID, &sourceChannelName,
			&quote.SourceMsgAuthorDiscordID, &sourceMsgAuthorUsername,
			&sourceMsgTimestamp, &tagArray, &quote.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		quote.AuthorDiscordID = authorDiscordID.String
		quote.GuildName = guildName.String
		quote.AuthorUsername = authorUsername.String
		quote.SourceChannelName = sourceChannelName.String
		quote.SourceMsgAuthorUsername = sourceMsgAuthorUsername.String
		if sourceMsgTimestamp.Valid {
			quote.SourceMsgTimestamp = sourceMsgTimestamp.Time
		}
		quote.Tags = tagArray
		quotes = append(quotes, quote)
	}

	return quotes, total, nil
}

func (r *quoteRepository) GetRandom(ctx context.Context, guildID string, tags []string) (*entities.Quote, error) {
	// Build WHERE conditions
	conditions := []string{"q.guild_id = $1", "q.deleted_at IS NULL"}
	args := []interface{}{guildID}
	argCount := 1

	if len(tags) > 0 {
		argCount++
		conditions = append(conditions, fmt.Sprintf("q.tags && $%d", argCount))
		args = append(args, pq.Array(tags))
	}

	whereClause := strings.Join(conditions, " AND ")

	query := fmt.Sprintf(`
		SELECT q.id, q.body, q.author_id, q.author_discord_id, u.name, q.guild_id, dg.guild_name,
		       q.source_msg_id, q.source_channel_id, q.source_channel_name,
		       q.source_msg_author_discord_id, q.source_msg_author_username, q.source_msg_timestamp, q.tags, q.created_at
		FROM quotes q
		LEFT JOIN discord_guilds dg ON q.guild_id = dg.guild_id
		LEFT JOIN users u ON q.author_id = u.id
		WHERE %s
		ORDER BY RANDOM()
		LIMIT 1
	`, whereClause)

	quote := &entities.Quote{}
	var tagArray pq.StringArray
	var guildName, authorDiscordID, authorUsername, sourceChannelName, sourceMsgAuthorUsername sql.NullString
	var sourceMsgTimestamp sql.NullTime

	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&quote.ID, &quote.Body, &quote.AuthorID, &authorDiscordID, &authorUsername, &quote.GuildID, &guildName,
		&quote.SourceMsgID, &quote.SourceChannelID, &sourceChannelName,
		&quote.SourceMsgAuthorDiscordID, &sourceMsgAuthorUsername,
		&sourceMsgTimestamp, &tagArray, &quote.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no quotes found")
	}
	if err != nil {
		return nil, err
	}

	quote.AuthorDiscordID = authorDiscordID.String
	quote.GuildName = guildName.String
	quote.AuthorUsername = authorUsername.String
	quote.SourceChannelName = sourceChannelName.String
	quote.SourceMsgAuthorUsername = sourceMsgAuthorUsername.String
	if sourceMsgTimestamp.Valid {
		quote.SourceMsgTimestamp = sourceMsgTimestamp.Time
	}
	quote.Tags = tagArray
	return quote, nil
}
