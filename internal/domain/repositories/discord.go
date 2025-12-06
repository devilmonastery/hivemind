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

	// UpdateMemberSyncTime updates the last_member_sync timestamp
	UpdateMemberSyncTime(ctx context.Context, guildID string) error

	// Delete removes a guild record
	Delete(ctx context.Context, guildID string) error

	// UpdateSettings updates the settings JSONB for a guild
	UpdateSettings(ctx context.Context, guildID string, settings map[string]interface{}) error

	// GetSettings retrieves the settings JSONB for a guild
	GetSettings(ctx context.Context, guildID string) (map[string]interface{}, error)
}

// GuildMemberRepository handles Discord guild membership data persistence
type GuildMemberRepository interface {
	// Upsert creates or updates a guild member record
	Upsert(ctx context.Context, member *entities.GuildMember) error

	// UpsertBatch efficiently inserts/updates multiple members in a transaction
	UpsertBatch(ctx context.Context, members []*entities.GuildMember) error

	// IsMember checks if a Discord user is a member of a guild
	IsMember(ctx context.Context, guildID, discordID string) (bool, error)

	// GetMember retrieves a guild member record
	GetMember(ctx context.Context, guildID, discordID string) (*entities.GuildMember, error)

	// ListGuildMembers retrieves all members for a guild
	ListGuildMembers(ctx context.Context, guildID string) ([]*entities.GuildMember, error)

	// ListUserGuilds retrieves all guild IDs a user is a member of
	ListUserGuilds(ctx context.Context, discordID string) ([]string, error)

	// UpdateLastSeen updates the last_seen timestamp for a guild member
	UpdateLastSeen(ctx context.Context, guildID, discordID string) error

	// DeleteMember removes a member record (when they leave)
	DeleteMember(ctx context.Context, guildID, discordID string) error

	// DeleteAllGuildMembers removes all members for a guild (when bot leaves guild)
	DeleteAllGuildMembers(ctx context.Context, guildID string) error

	// CountGuildMembers returns the number of members in a guild
	CountGuildMembers(ctx context.Context, guildID string) (int, error)

	// RefreshDisplayNames updates the user_display_names table for a guild
	// This should be called after batch upserting guild members to keep display names in sync
	RefreshDisplayNames(ctx context.Context, guildID string) error
}
