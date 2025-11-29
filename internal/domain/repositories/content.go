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

	// GetByGuildAndTitle retrieves a wiki page by guild ID and title (case-insensitive)
	GetByGuildAndTitle(ctx context.Context, guildID, title string) (*entities.WikiPage, error)

	// Update updates an existing wiki page
	Update(ctx context.Context, page *entities.WikiPage) error

	// Delete soft-deletes a wiki page
	Delete(ctx context.Context, id string) error

	// List lists wiki pages in a guild with pagination
	List(ctx context.Context, guildID string, limit, offset int, orderBy string, ascending bool) ([]*entities.WikiPage, int, error)

	// Search performs full-text search on wiki pages
	Search(ctx context.Context, guildID, query string, tags []string, limit, offset int) ([]*entities.WikiPage, int, error)
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
}
