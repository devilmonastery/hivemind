package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
	"github.com/devilmonastery/hivemind/internal/pkg/idgen"
	"github.com/devilmonastery/hivemind/internal/pkg/metrics"
)

type noteMessageReferenceRepository struct {
	db  *sql.DB
	log *slog.Logger
}

// NewNoteMessageReferenceRepository creates a new PostgreSQL note message reference repository
func NewNoteMessageReferenceRepository(db *sql.DB) repositories.NoteMessageReferenceRepository {
	return &noteMessageReferenceRepository{
		db:  db,
		log: slog.Default().With(slog.String("repo", "note_message_reference")),
	}
}

func (r *noteMessageReferenceRepository) Create(ctx context.Context, ref *entities.NoteMessageReference) error {
	start := time.Now()
	var err error
	defer func() {
		metrics.RecordDBOperation("note_message_reference", "create", time.Since(start), 1, err)
	}()

	if ref.ID == "" {
		ref.ID = idgen.GenerateID()
	}
	ref.AddedAt = time.Now()

	r.log.Debug("creating note message reference",
		slog.String("note_id", ref.NoteID),
		slog.String("message_id", ref.MessageID))

	// Marshal attachments to JSON for JSONB column
	var attachmentMetadata interface{}
	if len(ref.Attachments) > 0 {
		jsonData, marshalErr := json.Marshal(ref.Attachments)
		if marshalErr != nil {
			err = marshalErr
			return err
		}
		attachmentMetadata = jsonData
	} else {
		// Use NULL for empty attachments
		attachmentMetadata = nil
	}

	query := `
		INSERT INTO note_message_references (
			id, note_id, message_id, channel_id, guild_id,
			content, author_id, author_username, author_display_name,
			message_timestamp, attachment_metadata, added_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (note_id, message_id) DO NOTHING
		RETURNING id, added_at
	`

	var returnedID string
	var returnedAddedAt time.Time
	err = r.db.QueryRowContext(ctx, query,
		ref.ID, ref.NoteID, ref.MessageID, ref.ChannelID, nullString(ref.GuildID),
		ref.Content, ref.AuthorID, ref.AuthorUsername, nullString(ref.AuthorDisplayName),
		ref.MessageTimestamp, attachmentMetadata, ref.AddedAt,
	).Scan(&returnedID, &returnedAddedAt)

	if err == sql.ErrNoRows {
		// ON CONFLICT DO NOTHING was triggered - reference already exists (no-op)
		return nil
	}
	if err != nil {
		return err
	}

	// Update with returned values
	ref.ID = returnedID
	ref.AddedAt = returnedAddedAt
	return nil
}

func (r *noteMessageReferenceRepository) GetByNoteID(ctx context.Context, noteID string) ([]*entities.NoteMessageReference, error) {
	start := time.Now()
	var err error
	var rowCount int64
	defer func() {
		metrics.RecordDBOperation("note_message_reference", "get_by_note_id", time.Since(start), rowCount, err)
	}()

	r.log.Debug("getting message references by note id", slog.String("note_id", noteID))

	query := `
		SELECT 
			nmr.id, nmr.note_id, nmr.message_id, nmr.channel_id, nmr.guild_id,
			nmr.content, nmr.author_id, nmr.author_username, nmr.author_display_name,
			nmr.message_timestamp, nmr.attachment_metadata, nmr.added_at,
			udn.guild_avatar_hash, udn.user_avatar_hash
		FROM note_message_references nmr
		LEFT JOIN user_display_names udn ON nmr.author_id = udn.discord_id AND nmr.guild_id = udn.guild_id
		WHERE nmr.note_id = $1
		ORDER BY nmr.added_at DESC
	`

	rows, queryErr := r.db.QueryContext(ctx, query, noteID)
	if queryErr != nil {
		err = queryErr
		return nil, err
	}
	defer rows.Close()

	var refs []*entities.NoteMessageReference
	for rows.Next() {
		ref := &entities.NoteMessageReference{}
		var authorDisplayName, guildID, guildAvatarHash, userAvatarHash sql.NullString
		var attachmentMetadata []byte

		scanErr := rows.Scan(
			&ref.ID, &ref.NoteID, &ref.MessageID, &ref.ChannelID, &guildID,
			&ref.Content, &ref.AuthorID, &ref.AuthorUsername, &authorDisplayName,
			&ref.MessageTimestamp, &attachmentMetadata, &ref.AddedAt,
			&guildAvatarHash, &userAvatarHash,
		)
		if scanErr != nil {
			err = scanErr
			return nil, err
		}

		ref.AuthorDisplayName = authorDisplayName.String
		ref.AuthorGuildAvatarHash = guildAvatarHash.String
		ref.AuthorUserAvatarHash = userAvatarHash.String
		ref.GuildID = guildID.String

		// Unmarshal attachment metadata if present
		if len(attachmentMetadata) > 0 {
			if unmarshalErr := json.Unmarshal(attachmentMetadata, &ref.Attachments); unmarshalErr != nil {
				// Log error but don't fail - attachments are optional
				ref.Attachments = []entities.AttachmentMetadata{}
			}
		}

		refs = append(refs, ref)
	}

	rowCount = int64(len(refs))
	err = rows.Err()
	return refs, err
}

func (r *noteMessageReferenceRepository) GetByMessageID(ctx context.Context, messageID string) ([]*entities.NoteMessageReference, error) {
	start := time.Now()
	var err error
	var rowCount int64
	defer func() {
		metrics.RecordDBOperation("note_message_reference", "get_by_message_id", time.Since(start), rowCount, err)
	}()

	query := `
		SELECT 
			nmr.id, nmr.note_id, nmr.message_id, nmr.channel_id, nmr.guild_id,
			nmr.content, nmr.author_id, nmr.author_username, nmr.author_display_name,
			nmr.message_timestamp, nmr.attachment_metadata, nmr.added_at,
			udn.guild_avatar_hash, udn.user_avatar_hash
		FROM note_message_references nmr
		LEFT JOIN user_display_names udn ON nmr.author_id = udn.discord_id AND nmr.guild_id = udn.guild_id
		WHERE nmr.message_id = $1
		ORDER BY nmr.added_at DESC
	`

	rows, queryErr := r.db.QueryContext(ctx, query, messageID)
	if queryErr != nil {
		err = queryErr
		return nil, err
	}
	defer rows.Close()

	var refs []*entities.NoteMessageReference
	for rows.Next() {
		ref := &entities.NoteMessageReference{}
		var authorDisplayName, guildID, guildAvatarHash, userAvatarHash sql.NullString
		var attachmentMetadata []byte

		scanErr := rows.Scan(
			&ref.ID, &ref.NoteID, &ref.MessageID, &ref.ChannelID, &guildID,
			&ref.Content, &ref.AuthorID, &ref.AuthorUsername, &authorDisplayName,
			&ref.MessageTimestamp, &attachmentMetadata, &ref.AddedAt,
			&guildAvatarHash, &userAvatarHash,
		)
		if scanErr != nil {
			err = scanErr
			return nil, err
		}

		ref.AuthorDisplayName = authorDisplayName.String
		ref.AuthorGuildAvatarHash = guildAvatarHash.String
		ref.AuthorUserAvatarHash = userAvatarHash.String
		ref.GuildID = guildID.String

		// Unmarshal attachment metadata if present
		if len(attachmentMetadata) > 0 {
			if unmarshalErr := json.Unmarshal(attachmentMetadata, &ref.Attachments); unmarshalErr != nil {
				ref.Attachments = []entities.AttachmentMetadata{}
			}
		}

		refs = append(refs, ref)
	}

	rowCount = int64(len(refs))
	err = rows.Err()
	return refs, err
}

func (r *noteMessageReferenceRepository) Delete(ctx context.Context, id string) error {
	start := time.Now()
	var err error
	var rowsAffected int64
	defer func() {
		metrics.RecordDBOperation("note_message_reference", "delete", time.Since(start), rowsAffected, err)
	}()

	r.log.Debug("deleting note message reference", slog.String("id", id))

	query := `DELETE FROM note_message_references WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err = result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		err = sql.ErrNoRows
		return err
	}

	return nil
}

func (r *noteMessageReferenceRepository) DeleteByMessageID(ctx context.Context, messageID string) error {
	start := time.Now()
	var err error
	var rowsAffected int64
	defer func() {
		metrics.RecordDBOperation("note_message_reference", "delete_by_message_id", time.Since(start), rowsAffected, err)
	}()

	query := `DELETE FROM note_message_references WHERE message_id = $1`
	result, err := r.db.ExecContext(ctx, query, messageID)
	if err != nil {
		return err
	}

	rowsAffected, err = result.RowsAffected()
	return err
}
