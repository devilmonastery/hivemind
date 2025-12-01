package entities

import (
	"fmt"
	"time"
)

// DiscordUser represents a Discord identity linked to a Hivemind user
type DiscordUser struct {
	DiscordID         string     `json:"discord_id" db:"discord_id"`
	UserID            *string    `json:"user_id,omitempty" db:"user_id"`
	DiscordUsername   string     `json:"discord_username" db:"discord_username"`
	DiscordGlobalName *string    `json:"discord_global_name,omitempty" db:"discord_global_name"`
	AvatarURL         *string    `json:"avatar_url,omitempty" db:"avatar_url"`
	LinkedAt          time.Time  `json:"linked_at" db:"linked_at"`
	LastSeen          *time.Time `json:"last_seen,omitempty" db:"last_seen"`
}

// DiscordGuild represents a Discord server configuration
type DiscordGuild struct {
	GuildID        string     `json:"guild_id" db:"guild_id"`
	GuildName      string     `json:"guild_name" db:"guild_name"`
	IconURL        *string    `json:"icon_url,omitempty" db:"icon_url"`
	OwnerID        *string    `json:"owner_discord_id,omitempty" db:"owner_discord_id"`
	Enabled        bool       `json:"enabled" db:"enabled"`
	Settings       string     `json:"settings" db:"settings"` // JSONB stored as string
	AddedAt        time.Time  `json:"added_at" db:"added_at"`
	LastActivity   *time.Time `json:"last_activity,omitempty" db:"last_activity"`
	LastMemberSync *time.Time `json:"last_member_sync,omitempty" db:"last_member_sync"`
}

// GuildMember represents a Discord user's membership in a guild
type GuildMember struct {
	GuildID         string     `json:"guild_id" db:"guild_id"`
	DiscordID       string     `json:"discord_id" db:"discord_id"`
	GuildNick       *string    `json:"guild_nick,omitempty" db:"guild_nick"`
	GuildAvatarHash *string    `json:"guild_avatar_hash,omitempty" db:"guild_avatar_hash"`
	Roles           []string   `json:"roles" db:"roles"`
	JoinedAt        time.Time  `json:"joined_at" db:"joined_at"`
	SyncedAt        time.Time  `json:"synced_at" db:"synced_at"`
	LastSeen        *time.Time `json:"last_seen,omitempty" db:"last_seen"`
}

// GuildAvatarURL constructs the CDN URL for the guild-specific avatar
// Returns empty string if no guild avatar is set (caller should fall back to global avatar)
func (m *GuildMember) GuildAvatarURL(size int) string {
	if m.GuildAvatarHash == nil || *m.GuildAvatarHash == "" {
		return ""
	}
	return fmt.Sprintf("https://cdn.discordapp.com/guilds/%s/users/%s/avatars/%s.png?size=%d",
		m.GuildID, m.DiscordID, *m.GuildAvatarHash, size)
}

// DisplayName returns the guild nickname if set, otherwise empty string
func (m *GuildMember) DisplayName() string {
	if m.GuildNick != nil && *m.GuildNick != "" {
		return *m.GuildNick
	}
	return ""
}
