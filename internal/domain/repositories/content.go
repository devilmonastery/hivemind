package repositories

import (
	"context"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
)

// WikiPageRepository defines operations for wiki page persistence
type WikiPageRepository interface {
	// Create creates a new wiki page
	Create(ctx context.Context, page *entities.WikiPage) error

	// GetByID retrieves a wiki page by ID
	GetByID(ctx context.Context, id string) (*entities.WikiPage, error)

	// GetByGuildAndSlug retrieves a wiki page by guild ID and slug (normalized for lookup)
	GetByGuildAndSlug(ctx context.Context, guildID, slug string) (*entities.WikiPage, error)

	// Update updates an existing wiki page
	Update(ctx context.Context, page *entities.WikiPage) error

	// Delete soft-deletes a wiki page
	Delete(ctx context.Context, id string) error

	// List lists wiki pages in a guild with pagination
	List(ctx context.Context, guildID string, limit, offset int, orderBy string, ascending bool) ([]*entities.WikiPage, int, error)

	// Search performs full-text search on wiki pages
	Search(ctx context.Context, guildID, query string, tags []string, limit, offset int) ([]*entities.WikiPage, int, error)

	// GetTitlesForGuild retrieves only the ID, title, and slug of all wiki pages in a guild
	GetTitlesForGuild(ctx context.Context, guildID string) ([]struct {
		ID    string
		Title string
		Slug  string
	}, error)
}

// WikiTitleRepository defines operations for wiki title (canonical + alias) persistence
type WikiTitleRepository interface {
	// Create creates a new title (canonical or alias)
	Create(ctx context.Context, title *entities.WikiTitle) error

	// GetByGuildAndSlug retrieves the page ID for a slug (normalized lookup)
	GetByGuildAndSlug(ctx context.Context, guildID, slug string) (*entities.WikiTitle, error)

	// GetCanonicalTitle retrieves the canonical title for a page
	GetCanonicalTitle(ctx context.Context, pageID string) (*entities.WikiTitle, error)

	// ListByPageID retrieves all titles (canonical + aliases) for a page
	ListByPageID(ctx context.Context, pageID string) ([]*entities.WikiTitle, error)

	// UpdatePageID updates the page ID for all non-canonical titles pointing to oldPageID
	UpdatePageID(ctx context.Context, oldPageID, newPageID string) (int, error)
}

// NoteRepository defines operations for note persistence
type NoteRepository interface {
	// Create creates a new note
	Create(ctx context.Context, note *entities.Note) error

	// GetByID retrieves a note by ID
	GetByID(ctx context.Context, id string) (*entities.Note, error)

	// Update updates an existing note
	Update(ctx context.Context, note *entities.Note) error

	// Delete soft-deletes a note
	Delete(ctx context.Context, id string) error

	// List lists notes for a user with optional filtering
	List(ctx context.Context, authorID, guildID string, tags []string, limit, offset int, orderBy string, ascending bool) ([]*entities.Note, int, error)

	// Search performs full-text search on notes
	Search(ctx context.Context, authorID string, query, guildID string, tags []string, limit, offset int) ([]*entities.Note, int, error)

	// GetTitlesForUser retrieves only the ID and title of all notes for a user in a guild
	GetTitlesForUser(ctx context.Context, authorID, guildID string) ([]struct {
		ID    string
		Title string
	}, error)
}

// QuoteRepository defines operations for quote persistence
type QuoteRepository interface {
	// Create creates a new quote
	Create(ctx context.Context, quote *entities.Quote) error

	// GetByID retrieves a quote by ID
	GetByID(ctx context.Context, id string) (*entities.Quote, error)

	// Delete soft-deletes a quote
	Delete(ctx context.Context, id string) error

	// List lists quotes in a guild with pagination
	List(ctx context.Context, guildID string, authorDiscordID string, tags []string, limit, offset int, orderBy string, ascending bool) ([]*entities.Quote, int, error)

	// Search performs full-text search on quotes
	Search(ctx context.Context, guildID, query string, tags []string, limit, offset int) ([]*entities.Quote, int, error)

	// GetRandom retrieves a random quote from a guild
	GetRandom(ctx context.Context, guildID string, tags []string) (*entities.Quote, error)
}

// WikiMessageReferenceRepository defines operations for wiki message reference persistence
type WikiMessageReferenceRepository interface {
	// Create creates a new wiki message reference (no-op if already exists)
	Create(ctx context.Context, ref *entities.WikiMessageReference) error

	// GetByPageID retrieves all message references for a wiki page (ordered by added_at DESC)
	GetByPageID(ctx context.Context, pageID string) ([]*entities.WikiMessageReference, error)

	// GetByMessageID retrieves all wiki pages that reference a specific message
	GetByMessageID(ctx context.Context, messageID string) ([]*entities.WikiMessageReference, error)

	// Delete deletes a specific message reference by ID
	Delete(ctx context.Context, id string) error

	// DeleteByMessageID deletes all references to a specific message (cleanup if message deleted)
	DeleteByMessageID(ctx context.Context, messageID string) error

	// TransferReferences transfers all references from sourcePageID to targetPageID
	// Uses ON CONFLICT DO NOTHING to handle duplicates
	TransferReferences(ctx context.Context, sourcePageID, targetPageID string) (int, error)
}

// NoteMessageReferenceRepository defines operations for note message reference persistence
type NoteMessageReferenceRepository interface {
	// Create creates a new note message reference (no-op if already exists)
	Create(ctx context.Context, ref *entities.NoteMessageReference) error

	// GetByNoteID retrieves all message references for a note (ordered by added_at DESC)
	GetByNoteID(ctx context.Context, noteID string) ([]*entities.NoteMessageReference, error)

	// GetByMessageID retrieves all notes that reference a specific message
	GetByMessageID(ctx context.Context, messageID string) ([]*entities.NoteMessageReference, error)

	// Delete deletes a specific message reference by ID
	Delete(ctx context.Context, id string) error

	// DeleteByMessageID deletes all references to a specific message (cleanup if message deleted)
	DeleteByMessageID(ctx context.Context, messageID string) error
}
