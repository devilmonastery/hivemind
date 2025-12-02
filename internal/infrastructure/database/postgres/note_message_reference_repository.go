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
		jsonData, err := json.Marshal(ref.Attachments)
		if err != nil {
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
	err := r.db.QueryRowContext(ctx, query,
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
	r.log.Debug("getting message references by note id", slog.String("note_id", noteID))

	query := `
		SELECT 
			nmr.id, nmr.note_id, nmr.message_id, nmr.channel_id, nmr.guild_id,
			nmr.content, nmr.author_id, nmr.author_username, nmr.author_display_name,
			nmr.message_timestamp, nmr.attachment_metadata, nmr.added_at,
			du.avatar_url
		FROM note_message_references nmr
		LEFT JOIN discord_users du ON nmr.author_id = du.discord_id
		WHERE nmr.note_id = $1
		ORDER BY nmr.added_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []*entities.NoteMessageReference
	for rows.Next() {
		ref := &entities.NoteMessageReference{}
		var authorDisplayName, authorAvatarURL, guildID sql.NullString
		var attachmentMetadata []byte

		err := rows.Scan(
			&ref.ID, &ref.NoteID, &ref.MessageID, &ref.ChannelID, &guildID,
			&ref.Content, &ref.AuthorID, &ref.AuthorUsername, &authorDisplayName,
			&ref.MessageTimestamp, &attachmentMetadata, &ref.AddedAt,
			&authorAvatarURL,
		)
		if err != nil {
			return nil, err
		}

		ref.AuthorDisplayName = authorDisplayName.String
		ref.AuthorAvatarURL = authorAvatarURL.String
		ref.GuildID = guildID.String

		// Unmarshal attachment metadata if present
		if len(attachmentMetadata) > 0 {
			if err := json.Unmarshal(attachmentMetadata, &ref.Attachments); err != nil {
				// Log error but don't fail - attachments are optional
				ref.Attachments = []entities.AttachmentMetadata{}
			}
		}

		refs = append(refs, ref)
	}

	return refs, rows.Err()
}

func (r *noteMessageReferenceRepository) GetByMessageID(ctx context.Context, messageID string) ([]*entities.NoteMessageReference, error) {
	query := `
		SELECT 
			nmr.id, nmr.note_id, nmr.message_id, nmr.channel_id, nmr.guild_id,
			nmr.content, nmr.author_id, nmr.author_username, nmr.author_display_name,
			nmr.message_timestamp, nmr.attachment_metadata, nmr.added_at,
			du.avatar_url
		FROM note_message_references nmr
		LEFT JOIN discord_users du ON nmr.author_id = du.discord_id
		WHERE nmr.message_id = $1
		ORDER BY nmr.added_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []*entities.NoteMessageReference
	for rows.Next() {
		ref := &entities.NoteMessageReference{}
		var authorDisplayName, authorAvatarURL, guildID sql.NullString
		var attachmentMetadata []byte

		err := rows.Scan(
			&ref.ID, &ref.NoteID, &ref.MessageID, &ref.ChannelID, &guildID,
			&ref.Content, &ref.AuthorID, &ref.AuthorUsername, &authorDisplayName,
			&ref.MessageTimestamp, &attachmentMetadata, &ref.AddedAt,
			&authorAvatarURL,
		)
		if err != nil {
			return nil, err
		}

		ref.AuthorDisplayName = authorDisplayName.String
		ref.AuthorAvatarURL = authorAvatarURL.String
		ref.GuildID = guildID.String

		// Unmarshal attachment metadata if present
		if len(attachmentMetadata) > 0 {
			if err := json.Unmarshal(attachmentMetadata, &ref.Attachments); err != nil {
				ref.Attachments = []entities.AttachmentMetadata{}
			}
		}

		refs = append(refs, ref)
	}

	return refs, rows.Err()
}

func (r *noteMessageReferenceRepository) Delete(ctx context.Context, id string) error {
	r.log.Debug("deleting note message reference", slog.String("id", id))

	query := `DELETE FROM note_message_references WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *noteMessageReferenceRepository) DeleteByMessageID(ctx context.Context, messageID string) error {
	query := `DELETE FROM note_message_references WHERE message_id = $1`
	_, err := r.db.ExecContext(ctx, query, messageID)
	return err
}
