package entities

import "time"

// WikiPage represents a guild knowledge base article
type WikiPage struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Slug      string     `json:"slug"`
	Body      string     `json:"body"`
	AuthorID  string     `json:"author_id"`
	GuildID   string     `json:"guild_id"`
	GuildName string     `json:"guild_name,omitempty"`
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
	GuildName       string     `json:"guild_name,omitempty"`
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
	AuthorID                 string     `json:"author_id"`         // Who saved the quote (internal user ID)
	AuthorDiscordID          string     `json:"author_discord_id"` // Discord ID of who saved the quote
	AuthorUsername           string     `json:"author_username,omitempty"`
	AuthorGuildNick          string     `json:"author_guild_nick,omitempty"` // Current guild nickname
	GuildID                  string     `json:"guild_id"`
	GuildName                string     `json:"guild_name,omitempty"`
	SourceMsgID              string     `json:"source_msg_id"`
	SourceChannelID          string     `json:"source_channel_id"`
	SourceChannelName        string     `json:"source_channel_name,omitempty"`
	SourceMsgAuthorDiscordID string     `json:"source_msg_author_discord_id"`
	SourceMsgAuthorUsername  string     `json:"source_msg_author_username,omitempty"`
	SourceMsgAuthorGuildNick string     `json:"source_msg_author_guild_nick,omitempty"` // Current guild nickname
	SourceMsgTimestamp       time.Time  `json:"source_msg_timestamp"`                   // When the original message was sent
	Tags                     []string   `json:"tags,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
	DeletedAt                *time.Time `json:"deleted_at,omitempty"`
}

// WikiMessageReference represents a Discord message tagged with a wiki page topic
type WikiMessageReference struct {
	ID                string               `json:"id"`
	WikiPageID        string               `json:"wiki_page_id"`
	MessageID         string               `json:"message_id"`
	ChannelID         string               `json:"channel_id"`
	GuildID           string               `json:"guild_id"`
	Content           string               `json:"content"`
	AuthorID          string               `json:"author_id"`
	AuthorUsername    string               `json:"author_username"`
	AuthorDisplayName string               `json:"author_display_name,omitempty"`
	AuthorAvatarURL   string               `json:"author_avatar_url,omitempty"`
	MessageTimestamp  time.Time            `json:"message_timestamp"`
	AttachmentURLs    []string             `json:"attachment_urls,omitempty"` // Deprecated, use Attachments
	Attachments       []AttachmentMetadata `json:"attachments,omitempty"`     // Attachment metadata with content types
	AddedAt           time.Time            `json:"added_at"`
	AddedByUserID     string               `json:"added_by_user_id,omitempty"`
}

// NoteMessageReference represents a Discord message referenced in a private note
type NoteMessageReference struct {
	ID                string               `json:"id"`
	NoteID            string               `json:"note_id"`
	MessageID         string               `json:"message_id"`
	ChannelID         string               `json:"channel_id"`
	GuildID           string               `json:"guild_id,omitempty"` // Nullable for DM contexts
	Content           string               `json:"content"`
	AuthorID          string               `json:"author_id"`
	AuthorUsername    string               `json:"author_username"`
	AuthorDisplayName string               `json:"author_display_name,omitempty"`
	AuthorAvatarURL   string               `json:"author_avatar_url,omitempty"`
	MessageTimestamp  time.Time            `json:"message_timestamp"`
	Attachments       []AttachmentMetadata `json:"attachments,omitempty"` // Attachment metadata with content types
	AddedAt           time.Time            `json:"added_at"`
}

// WikiTitle represents a title (canonical or alias) for a wiki page
type WikiTitle struct {
	ID              string    `json:"id"`
	GuildID         string    `json:"guild_id"`
	DisplayTitle    string    `json:"display_title"` // Original formatting for display
	PageSlug        string    `json:"page_slug"`     // URL-friendly slug for lookups
	PageID          string    `json:"page_id"`
	IsCanonical     bool      `json:"is_canonical"`
	CreatedAt       time.Time `json:"created_at"`
	CreatedByUserID string    `json:"created_by_user_id,omitempty"`
	CreatedByMerge  bool      `json:"created_by_merge"`
}

// AttachmentMetadata stores Discord attachment information
type AttachmentMetadata struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type,omitempty"` // MIME type (e.g., "image/png", "video/mp4")
	Filename    string `json:"filename,omitempty"`
	Width       int    `json:"width,omitempty"`  // For images/videos
	Height      int    `json:"height,omitempty"` // For images/videos
	Size        int64  `json:"size,omitempty"`   // File size in bytes
}
