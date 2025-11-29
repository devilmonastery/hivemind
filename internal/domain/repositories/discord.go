package repositories

import (
	"context"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
)

// DiscordUserRepository handles Discord user data persistence
type DiscordUserRepository interface {
	// Create creates a new Discord user record
	Create(ctx context.Context, discordUser *entities.DiscordUser) error

	// Upsert creates or updates a Discord user record (based on discord_id)
	Upsert(ctx context.Context, discordUser *entities.DiscordUser) error

	// GetByDiscordID retrieves a Discord user by their Discord ID
	GetByDiscordID(ctx context.Context, discordID string) (*entities.DiscordUser, error)

	// GetByUserID retrieves a Discord user by their Hivemind user ID
	GetByUserID(ctx context.Context, userID string) (*entities.DiscordUser, error)

	// Update updates a Discord user record
	Update(ctx context.Context, discordUser *entities.DiscordUser) error

	// UpdateLastSeen updates the last_seen timestamp for a Discord user
	UpdateLastSeen(ctx context.Context, discordID string) error

	// Delete removes a Discord user record (unlinking)
	Delete(ctx context.Context, discordID string) error
}

// DiscordGuildRepository handles Discord guild data persistence
type DiscordGuildRepository interface {
	// Create creates a new Discord guild record
	Create(ctx context.Context, guild *entities.DiscordGuild) error

	// GetByID retrieves a guild by ID
	GetByID(ctx context.Context, guildID string) (*entities.DiscordGuild, error)

	// Update updates a guild record
	Update(ctx context.Context, guild *entities.DiscordGuild) error

	// UpdateLastActivity updates the last_activity timestamp
	UpdateLastActivity(ctx context.Context, guildID string) error

	// List retrieves all guilds (optionally filtered by enabled status)
	List(ctx context.Context, enabledOnly bool) ([]*entities.DiscordGuild, error)

	// Delete removes a guild record
	Delete(ctx context.Context, guildID string) error
}
