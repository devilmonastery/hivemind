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

type noteRepository struct {
	db *sql.DB
}

// NewNoteRepository creates a new PostgreSQL note repository
func NewNoteRepository(db *sql.DB) repositories.NoteRepository {
	return &noteRepository{db: db}
}

func (r *noteRepository) Create(ctx context.Context, note *entities.Note) error {
	if note.ID == "" {
		note.ID = idgen.GenerateID()
	}
	note.CreatedAt = time.Now()
	note.UpdatedAt = time.Now()

	query := `
		INSERT INTO notes (id, title, body, author_id, guild_id, channel_id, source_msg_id, source_channel_id, tags, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.db.ExecContext(ctx, query,
		note.ID, nullString(note.Title), note.Body, note.AuthorID, nullString(note.GuildID),
		nullString(note.ChannelID), nullString(note.SourceMsgID), nullString(note.SourceChannelID),
		pq.Array(note.Tags), note.CreatedAt, note.UpdatedAt,
	)
	return err
}

func (r *noteRepository) GetByID(ctx context.Context, id string) (*entities.Note, error) {
	query := `
		SELECT id, title, body, author_id, guild_id, channel_id, source_msg_id, source_channel_id, tags, created_at, updated_at, deleted_at
		FROM notes
		WHERE id = $1 AND deleted_at IS NULL
	`
	note := &entities.Note{}
	var tags pq.StringArray
	var title, guildID, channelID, sourceMsgID, sourceChannelID sql.NullString
	var deletedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&note.ID, &title, &note.Body, &note.AuthorID, &guildID,
		&channelID, &sourceMsgID, &sourceChannelID, &tags,
		&note.CreatedAt, &note.UpdatedAt, &deletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("note not found: %s", id)
	}
	if err != nil {
		return nil, err
	}

	note.Title = title.String
	note.GuildID = guildID.String
	note.ChannelID = channelID.String
	note.SourceMsgID = sourceMsgID.String
	note.SourceChannelID = sourceChannelID.String
	note.Tags = tags
	if deletedAt.Valid {
		note.DeletedAt = &deletedAt.Time
	}

	return note, nil
}

func (r *noteRepository) Update(ctx context.Context, note *entities.Note) error {
	note.UpdatedAt = time.Now()

	query := `
		UPDATE notes
		SET title = $2, body = $3, tags = $4, updated_at = $5
		WHERE id = $1 AND deleted_at IS NULL
	`
	result, err := r.db.ExecContext(ctx, query,
		note.ID, nullString(note.Title), note.Body, pq.Array(note.Tags), note.UpdatedAt,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("note not found: %s", note.ID)
	}

	return nil
}

func (r *noteRepository) Delete(ctx context.Context, id string) error {
	query := `
		UPDATE notes
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
		return fmt.Errorf("note not found: %s", id)
	}

	return nil
}

func (r *noteRepository) List(ctx context.Context, authorID, guildID string, tags []string, limit, offset int, orderBy string, ascending bool) ([]*entities.Note, int, error) {
	if limit <= 0 {
		limit = 50
	}

	// Build WHERE conditions
	conditions := []string{"author_id = $1", "deleted_at IS NULL"}
	args := []interface{}{authorID}
	argCount := 1

	if guildID != "" {
		argCount++
		conditions = append(conditions, fmt.Sprintf("guild_id = $%d", argCount))
		args = append(args, guildID)
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
		"updated_at": true,
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
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM notes WHERE %s", whereClause)
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get notes
	query := fmt.Sprintf(`
		SELECT n.id, n.title, n.body, n.author_id, n.guild_id, dg.guild_name, n.channel_id, n.source_msg_id, n.source_channel_id, n.tags, n.created_at, n.updated_at
		FROM notes n
		LEFT JOIN discord_guilds dg ON n.guild_id = dg.guild_id
		WHERE %s
		ORDER BY n.%s %s
		LIMIT $%d OFFSET $%d
	`, whereClause, orderBy, direction, argCount+1, argCount+2)

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	notes := []*entities.Note{}
	for rows.Next() {
		note := &entities.Note{}
		var tags pq.StringArray
		var title, guildID, guildName, channelID, sourceMsgID, sourceChannelID sql.NullString

		err := rows.Scan(
			&note.ID, &title, &note.Body, &note.AuthorID, &guildID, &guildName,
			&channelID, &sourceMsgID, &sourceChannelID, &tags,
			&note.CreatedAt, &note.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		note.Title = title.String
		note.GuildID = guildID.String
		note.GuildName = guildName.String
		note.ChannelID = channelID.String
		note.SourceMsgID = sourceMsgID.String
		note.SourceChannelID = sourceChannelID.String
		note.Tags = tags
		notes = append(notes, note)
	}

	return notes, total, nil
}

func (r *noteRepository) Search(ctx context.Context, authorID string, query, guildID string, tags []string, limit, offset int) ([]*entities.Note, int, error) {
	if limit <= 0 {
		limit = 10
	}

	// Build search conditions
	conditions := []string{"author_id = $1", "deleted_at IS NULL"}
	args := []interface{}{authorID}
	argCount := 1

	// Full-text search on title and body
	if query != "" {
		argCount++
		// Use hybrid search vector: searches both english and simple dictionaries
		conditions = append(conditions, fmt.Sprintf("(search_vector @@ websearch_to_tsquery('english', $%d) OR search_vector @@ websearch_to_tsquery('simple', $%d))", argCount, argCount))
		args = append(args, query)
	}

	// Guild filtering
	if guildID != "" {
		argCount++
		conditions = append(conditions, fmt.Sprintf("guild_id = $%d", argCount))
		args = append(args, guildID)
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
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM notes WHERE %s", whereClause)
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get notes with ranking if query provided
	var searchQuery string
	if query != "" {
		searchQuery = fmt.Sprintf(`
			SELECT id, title, body, author_id, guild_id, channel_id, source_msg_id, source_channel_id, tags, created_at, updated_at
			FROM notes
			WHERE %s
			ORDER BY ts_rank(search_vector, websearch_to_tsquery('english', $2)) DESC
			LIMIT $%d OFFSET $%d
		`, whereClause, argCount+1, argCount+2)
	} else {
		searchQuery = fmt.Sprintf(`
			SELECT id, title, body, author_id, guild_id, channel_id, source_msg_id, source_channel_id, tags, created_at, updated_at
			FROM notes
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

	notes := []*entities.Note{}
	for rows.Next() {
		note := &entities.Note{}
		var tagArray pq.StringArray
		var title, guildID, channelID, sourceMsgID, sourceChannelID sql.NullString

		err := rows.Scan(
			&note.ID, &title, &note.Body, &note.AuthorID, &guildID,
			&channelID, &sourceMsgID, &sourceChannelID, &tagArray,
			&note.CreatedAt, &note.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		note.Title = title.String
		note.GuildID = guildID.String
		note.ChannelID = channelID.String
		note.SourceMsgID = sourceMsgID.String
		note.SourceChannelID = sourceChannelID.String
		note.Tags = tagArray
		notes = append(notes, note)
	}

	return notes, total, nil
}

// GetTitlesForUser retrieves only the ID and title of all notes for a user in a guild (lightweight for autocomplete)
func (r *noteRepository) GetTitlesForUser(ctx context.Context, authorID, guildID string) ([]struct {
	ID    string
	Title string
}, error,
) {
	conditions := []string{"author_id = $1", "deleted_at IS NULL"}
	args := []interface{}{authorID}

	if guildID != "" {
		conditions = append(conditions, "guild_id = $2")
		args = append(args, guildID)
	}

	whereClause := strings.Join(conditions, " AND ")

	query := fmt.Sprintf(`
		SELECT id, title
		FROM notes
		WHERE %s
		ORDER BY updated_at DESC
	`, whereClause)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []struct {
		ID    string
		Title string
	}

	for rows.Next() {
		var id string
		var title sql.NullString

		if err := rows.Scan(&id, &title); err != nil {
			return nil, err
		}

		results = append(results, struct {
			ID    string
			Title string
		}{
			ID:    id,
			Title: title.String,
		})
	}

	return results, rows.Err()
}
