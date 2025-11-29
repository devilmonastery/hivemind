-- Add note_message_references table for attaching Discord messages to notes

CREATE TABLE note_message_references (
    id TEXT PRIMARY KEY,
    note_id TEXT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
    
    -- Discord identifiers
    message_id TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    guild_id TEXT,  -- Nullable for DM contexts
    
    -- Captured message content (snapshot at time of reference)
    content TEXT NOT NULL,
    author_id TEXT NOT NULL,
    author_username TEXT NOT NULL,
    author_display_name TEXT,
    message_timestamp TIMESTAMPTZ NOT NULL,
    
    -- Attachment metadata (JSONB for rich attachment info with MIME types)
    attachment_metadata JSONB,
    
    -- Metadata
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE(note_id, message_id)
);

COMMENT ON TABLE note_message_references IS 'Discord messages referenced in private notes - creates many-to-many relationship between messages and notes';
COMMENT ON COLUMN note_message_references.content IS 'Snapshot of message content at time of reference - preserved even if Discord message is deleted';
COMMENT ON COLUMN note_message_references.guild_id IS 'Nullable to support DM contexts';
COMMENT ON COLUMN note_message_references.attachment_metadata IS 'JSONB array of attachment objects with URL, content_type, filename, dimensions, size';

-- Indexes for efficient queries
CREATE INDEX idx_note_msg_refs_note_id ON note_message_references(note_id, added_at DESC);
CREATE INDEX idx_note_msg_refs_message_id ON note_message_references(message_id);
CREATE INDEX idx_note_msg_refs_author_id ON note_message_references(author_id);
