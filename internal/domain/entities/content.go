package entities

import "time"

// WikiPage represents a guild knowledge base article
type WikiPage struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	AuthorID  string     `json:"author_id"`
	GuildID   string     `json:"guild_id"`
	ChannelID string     `json:"channel_id,omitempty"`
	Tags      []string   `json:"tags,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// Note represents a private user note
type Note struct {
	ID              string     `json:"id"`
	Title           string     `json:"title,omitempty"`
	Body            string     `json:"body"`
	AuthorID        string     `json:"author_id"`
	GuildID         string     `json:"guild_id,omitempty"` // NULL for personal notes
	ChannelID       string     `json:"channel_id,omitempty"`
	SourceMsgID     string     `json:"source_msg_id,omitempty"`
	SourceChannelID string     `json:"source_channel_id,omitempty"`
	Tags            []string   `json:"tags,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`
}

// Quote represents a saved memorable message from Discord
type Quote struct {
	ID                       string     `json:"id"`
	Body                     string     `json:"body"`
	AuthorID                 string     `json:"author_id"` // Who saved the quote
	GuildID                  string     `json:"guild_id"`
	SourceMsgID              string     `json:"source_msg_id"`
	SourceChannelID          string     `json:"source_channel_id"`
	SourceMsgAuthorDiscordID string     `json:"source_msg_author_discord_id"`
	Tags                     []string   `json:"tags,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
	DeletedAt                *time.Time `json:"deleted_at,omitempty"`
}
