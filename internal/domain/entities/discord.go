package entities

import "time"

// DiscordUser represents a Discord identity linked to a Hivemind user
type DiscordUser struct {
	DiscordID         string     `json:"discord_id" db:"discord_id"`
	UserID            string     `json:"user_id" db:"user_id"`
	DiscordUsername   string     `json:"discord_username" db:"discord_username"`
	DiscordGlobalName *string    `json:"discord_global_name,omitempty" db:"discord_global_name"`
	AvatarURL         *string    `json:"avatar_url,omitempty" db:"avatar_url"`
	LinkedAt          time.Time  `json:"linked_at" db:"linked_at"`
	LastSeen          *time.Time `json:"last_seen,omitempty" db:"last_seen"`
}

// DiscordGuild represents a Discord server configuration
type DiscordGuild struct {
	GuildID      string     `json:"guild_id" db:"guild_id"`
	GuildName    string     `json:"guild_name" db:"guild_name"`
	IconURL      *string    `json:"icon_url,omitempty" db:"icon_url"`
	OwnerID      *string    `json:"owner_discord_id,omitempty" db:"owner_discord_id"`
	Enabled      bool       `json:"enabled" db:"enabled"`
	Settings     string     `json:"settings" db:"settings"` // JSONB stored as string
	AddedAt      time.Time  `json:"added_at" db:"added_at"`
	LastActivity *time.Time `json:"last_activity,omitempty" db:"last_activity"`
}
