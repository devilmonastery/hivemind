package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/lib/pq"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
	"github.com/devilmonastery/hivemind/internal/pkg/idgen"
)

type wikiMessageReferenceRepository struct {
	db *sql.DB
}

// NewWikiMessageReferenceRepository creates a new PostgreSQL wiki message reference repository
func NewWikiMessageReferenceRepository(db *sql.DB) repositories.WikiMessageReferenceRepository {
	return &wikiMessageReferenceRepository{db: db}
}

func (r *wikiMessageReferenceRepository) Create(ctx context.Context, ref *entities.WikiMessageReference) error {
	if ref.ID == "" {
		ref.ID = idgen.GenerateID()
	}
	ref.AddedAt = time.Now()

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
		INSERT INTO wiki_message_references (
			id, wiki_page_id, message_id, channel_id, guild_id,
			content, author_id, author_username, author_display_name,
			message_timestamp, attachment_urls, attachment_metadata, added_at, added_by_user_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (wiki_page_id, message_id) DO NOTHING
		RETURNING id, added_at
	`

	var returnedID string
	var returnedAddedAt time.Time
	err := r.db.QueryRowContext(ctx, query,
		ref.ID, ref.WikiPageID, ref.MessageID, ref.ChannelID, ref.GuildID,
		ref.Content, ref.AuthorID, ref.AuthorUsername, nullString(ref.AuthorDisplayName),
		ref.MessageTimestamp, pq.Array(ref.AttachmentURLs), attachmentMetadata, ref.AddedAt, nullString(ref.AddedByUserID),
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

func (r *wikiMessageReferenceRepository) GetByPageID(ctx context.Context, pageID string) ([]*entities.WikiMessageReference, error) {
	query := `
		SELECT 
			wmr.id, wmr.wiki_page_id, wmr.message_id, wmr.channel_id, wmr.guild_id,
			wmr.content, wmr.author_id, wmr.author_username, wmr.author_display_name,
			wmr.message_timestamp, wmr.attachment_urls, wmr.attachment_metadata, 
			wmr.added_at, wmr.added_by_user_id,
			du.avatar_url
		FROM wiki_message_references wmr
		LEFT JOIN discord_users du ON wmr.author_id = du.discord_id
		WHERE wmr.wiki_page_id = $1
		ORDER BY wmr.message_timestamp DESC
	`

	rows, err := r.db.QueryContext(ctx, query, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []*entities.WikiMessageReference
	for rows.Next() {
		ref := &entities.WikiMessageReference{}
		var authorDisplayName, authorAvatarURL, addedByUserID sql.NullString
		var attachmentURLs pq.StringArray
		var attachmentMetadata []byte

		err := rows.Scan(
			&ref.ID, &ref.WikiPageID, &ref.MessageID, &ref.ChannelID, &ref.GuildID,
			&ref.Content, &ref.AuthorID, &ref.AuthorUsername, &authorDisplayName,
			&ref.MessageTimestamp, &attachmentURLs, &attachmentMetadata, &ref.AddedAt, &addedByUserID,
			&authorAvatarURL,
		)
		if err != nil {
			return nil, err
		}

		ref.AuthorDisplayName = authorDisplayName.String
		ref.AuthorAvatarURL = authorAvatarURL.String
		ref.AddedByUserID = addedByUserID.String
		ref.AttachmentURLs = attachmentURLs

		// Unmarshal attachment metadata if present
		if len(attachmentMetadata) > 0 {
			if err := json.Unmarshal(attachmentMetadata, &ref.Attachments); err != nil {
				// Log error but don't fail - attachments are optional
				// We still have attachment_urls as fallback
			}
		}

		refs = append(refs, ref)
	}

	return refs, rows.Err()
}

func (r *wikiMessageReferenceRepository) GetByMessageID(ctx context.Context, messageID string) ([]*entities.WikiMessageReference, error) {
	query := `
		SELECT id, wiki_page_id, message_id, channel_id, guild_id,
			   content, author_id, author_username, author_display_name,
			   message_timestamp, attachment_urls, added_at, added_by_user_id
		FROM wiki_message_references
		WHERE message_id = $1
		ORDER BY message_timestamp DESC
	`

	rows, err := r.db.QueryContext(ctx, query, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []*entities.WikiMessageReference
	for rows.Next() {
		ref := &entities.WikiMessageReference{}
		var authorDisplayName, addedByUserID sql.NullString
		var attachmentURLs pq.StringArray

		err := rows.Scan(
			&ref.ID, &ref.WikiPageID, &ref.MessageID, &ref.ChannelID, &ref.GuildID,
			&ref.Content, &ref.AuthorID, &ref.AuthorUsername, &authorDisplayName,
			&ref.MessageTimestamp, &attachmentURLs, &ref.AddedAt, &addedByUserID,
		)
		if err != nil {
			return nil, err
		}

		ref.AuthorDisplayName = authorDisplayName.String
		ref.AddedByUserID = addedByUserID.String
		ref.AttachmentURLs = attachmentURLs
		refs = append(refs, ref)
	}

	return refs, rows.Err()
}

func (r *wikiMessageReferenceRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM wiki_message_references WHERE id = $1`
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

func (r *wikiMessageReferenceRepository) DeleteByMessageID(ctx context.Context, messageID string) error {
	query := `DELETE FROM wiki_message_references WHERE message_id = $1`
	_, err := r.db.ExecContext(ctx, query, messageID)
	return err
}
