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

type noteRepository struct {
	db  *sql.DB
	log *slog.Logger
}

// NewNoteRepository creates a new PostgreSQL note repository
func NewNoteRepository(db *sql.DB) repositories.NoteRepository {
	return &noteRepository{
		db:  db,
		log: slog.Default().With(slog.String("repo", "note")),
	}
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

func (r *noteRepository) GetByID(ctx context.Context, id string, userDiscordID string) (*entities.Note, error) {
	// Notes are personal - no ACL check needed, just retrieve by ID
	// Ownership verification happens at the service layer (author_id check)
	query := `
		SELECT n.id, n.title, n.body, n.author_id, n.guild_id, n.channel_id, n.source_msg_id, n.source_channel_id, n.tags, n.created_at, n.updated_at, n.deleted_at,
		       udn.display_name
		FROM notes n
		LEFT JOIN users u ON n.author_id = u.id
		LEFT JOIN discord_users du ON u.id = du.user_id
		LEFT JOIN user_display_names udn ON du.discord_id = udn.discord_id AND n.guild_id = udn.guild_id
		WHERE n.id = $1 AND n.deleted_at IS NULL
	`

	note := &entities.Note{}
	var tags pq.StringArray
	var title, guildID, channelID, sourceMsgID, sourceChannelID, authorDisplayName sql.NullString
	var deletedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&note.ID, &title, &note.Body, &note.AuthorID, &guildID,
		&channelID, &sourceMsgID, &sourceChannelID, &tags,
		&note.CreatedAt, &note.UpdatedAt, &deletedAt,
		&authorDisplayName,
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
	note.AuthorDisplayName = authorDisplayName.String
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

func (r *noteRepository) List(ctx context.Context, authorID, guildID string, tags []string, limit, offset int, orderBy string, ascending bool, userDiscordID string) ([]*entities.Note, int, error) {
	if limit <= 0 {
		limit = 50
	}

	// Build FROM clause and WHERE conditions
	fromClause := "notes n"
	conditions := []string{"n.author_id = $1", "n.deleted_at IS NULL"}
	args := []interface{}{authorID}
	argCount := 1

	// Add guild filter if specified
	if guildID != "" {
		argCount++
		conditions = append(conditions, fmt.Sprintf("n.guild_id = $%d", argCount))
		args = append(args, guildID)

		// NO ACL CHECK: When listing your own notes (author_id = current_user),
		// you can always see them regardless of guild membership status.
		// ACL checks would only apply if we were querying OTHER users' notes.
	}
	// Note: If no guild_id filter, return all user's notes across all guilds

	if len(tags) > 0 {
		argCount++
		conditions = append(conditions, fmt.Sprintf("n.tags && $%d", argCount))
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
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", fromClause, whereClause)
	r.log.Debug("executing count query", slog.String("query", countQuery), slog.Any("args", args))
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		r.log.Debug("count query error", slog.String("error", err.Error()))
		return nil, 0, err
	}
	r.log.Debug("count query result", slog.Int("total", total))

	// Get notes
	query := fmt.Sprintf(`
		SELECT n.id, n.title, n.body, n.author_id, n.guild_id, dg.guild_name, n.channel_id, n.source_msg_id, n.source_channel_id, n.tags, n.created_at, n.updated_at,
		       udn.display_name
		FROM %s
		LEFT JOIN discord_guilds dg ON n.guild_id = dg.guild_id
		LEFT JOIN users u ON n.author_id = u.id
		LEFT JOIN discord_users du ON u.id = du.user_id
		LEFT JOIN user_display_names udn ON du.discord_id = udn.discord_id AND n.guild_id = udn.guild_id
		WHERE %s
		ORDER BY n.%s %s
		LIMIT $%d OFFSET $%d
	`, fromClause, whereClause, orderBy, direction, argCount+1, argCount+2)

	args = append(args, limit, offset)
	r.log.Debug("executing select query", slog.String("query", query), slog.Any("args", args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	notes := []*entities.Note{}
	for rows.Next() {
		note := &entities.Note{}
		var tags pq.StringArray
		var title, guildID, guildName, channelID, sourceMsgID, sourceChannelID, authorDisplayName sql.NullString

		err := rows.Scan(
			&note.ID, &title, &note.Body, &note.AuthorID, &guildID, &guildName,
			&channelID, &sourceMsgID, &sourceChannelID, &tags,
			&note.CreatedAt, &note.UpdatedAt,
			&authorDisplayName,
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
		note.AuthorDisplayName = authorDisplayName.String
		note.Tags = tags
		notes = append(notes, note)
	}

	return notes, total, nil
}

func (r *noteRepository) Search(ctx context.Context, authorID string, query, guildID string, tags []string, limit, offset int, userDiscordID string) ([]*entities.Note, int, error) {
	if limit <= 0 {
		limit = 10
	}

	// Build FROM clause and search conditions
	fromClause := "notes n"
	conditions := []string{"n.author_id = $1", "n.deleted_at IS NULL"}
	args := []interface{}{authorID}
	argCount := 1

	// Full-text search on title and body using ILIKE (search_vector column doesn't exist)
	if query != "" {
		argCount++
		conditions = append(conditions, fmt.Sprintf("(n.title ILIKE $%d OR n.body ILIKE $%d)", argCount, argCount))
		args = append(args, "%"+query+"%")
	}

	// Guild filtering
	if guildID != "" {
		argCount++
		conditions = append(conditions, fmt.Sprintf("n.guild_id = $%d", argCount))
		args = append(args, guildID)

		// Add ACL check only when filtering by guild (ensure user is a member)
		if userDiscordID != "" {
			fromClause += " INNER JOIN guild_members gm ON n.guild_id = gm.guild_id"
			argCount++
			conditions = append(conditions, fmt.Sprintf("gm.discord_id = $%d", argCount))
			args = append(args, userDiscordID)
		}
	}
	// Note: If no guild_id filter, return all user's notes across all guilds (no ACL check needed)

	// Tag filtering
	if len(tags) > 0 {
		argCount++
		conditions = append(conditions, fmt.Sprintf("n.tags && $%d", argCount))
		args = append(args, pq.Array(tags))
	}

	whereClause := strings.Join(conditions, " AND ")

	// Get total count
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", fromClause, whereClause)
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get notes (always order by created_at since we don't have search_vector for ranking)
	searchQuery := fmt.Sprintf(`
		SELECT n.id, n.title, n.body, n.author_id, n.guild_id, n.channel_id, n.source_msg_id, n.source_channel_id, n.tags, n.created_at, n.updated_at,
		       udn.display_name
		FROM %s
		LEFT JOIN users u ON n.author_id = u.id
		LEFT JOIN discord_users du ON u.id = du.user_id
		LEFT JOIN user_display_names udn ON du.discord_id = udn.discord_id AND n.guild_id = udn.guild_id
		WHERE %s
		ORDER BY n.created_at DESC
		LIMIT $%d OFFSET $%d
	`, fromClause, whereClause, argCount+1, argCount+2)

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
		var title, guildID, channelID, sourceMsgID, sourceChannelID, authorDisplayName sql.NullString

		err := rows.Scan(
			&note.ID, &title, &note.Body, &note.AuthorID, &guildID,
			&channelID, &sourceMsgID, &sourceChannelID, &tagArray,
			&note.CreatedAt, &note.UpdatedAt,
			&authorDisplayName,
		)
		if err != nil {
			return nil, 0, err
		}

		note.Title = title.String
		note.GuildID = guildID.String
		note.ChannelID = channelID.String
		note.SourceMsgID = sourceMsgID.String
		note.SourceChannelID = sourceChannelID.String
		note.AuthorDisplayName = authorDisplayName.String
		note.Tags = tagArray
		notes = append(notes, note)
	}

	return notes, total, nil
}

// GetTitlesForUser retrieves only the ID and title of all notes visible to a user in a guild (lightweight for autocomplete)
// Uses ACL filtering with guild_members JOIN
func (r *noteRepository) GetTitlesForUser(ctx context.Context, userDiscordID, guildID string) ([]struct {
	ID    string
	Title string
}, error,
) {
	var query string
	var args []interface{}

	if userDiscordID == "" {
		// Admin bypass: no ACL filtering
		query = `
			SELECT id, title
			FROM notes
			WHERE deleted_at IS NULL AND guild_id = $1
			ORDER BY updated_at DESC
		`
		args = []interface{}{guildID}
	} else {
		// Apply ACL filtering with guild_members JOIN
		query = `
			SELECT n.id, n.title
			FROM notes n
			INNER JOIN guild_members gm ON n.guild_id = gm.guild_id AND gm.discord_id = $1
			WHERE n.deleted_at IS NULL AND n.guild_id = $2
			ORDER BY n.updated_at DESC
		`
		args = []interface{}{userDiscordID, guildID}
	}

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
