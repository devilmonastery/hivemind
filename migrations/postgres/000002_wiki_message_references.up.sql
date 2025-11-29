-- Add wiki_message_references table for tagging Discord messages with wiki topics

CREATE TABLE wiki_message_references (
    id TEXT PRIMARY KEY,
    wiki_page_id TEXT NOT NULL REFERENCES wiki_pages(id) ON DELETE CASCADE,
    
    -- Discord identifiers
    message_id TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    guild_id TEXT NOT NULL,
    
    -- Captured message content (snapshot at time of reference)
    content TEXT NOT NULL,
    author_id TEXT NOT NULL,
    author_username TEXT NOT NULL,
    author_display_name TEXT,
    message_timestamp TIMESTAMPTZ NOT NULL,
    
    -- Attachment URLs (images, files, etc)
    attachment_urls TEXT[],
    
    -- Metadata
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    added_by_user_id TEXT REFERENCES users(id),
    
    UNIQUE(wiki_page_id, message_id)
);

COMMENT ON TABLE wiki_message_references IS 'Discord messages tagged with wiki page topics - creates many-to-many relationship between messages and wiki pages';
COMMENT ON COLUMN wiki_message_references.content IS 'Snapshot of message content at time of reference - preserved even if Discord message is deleted';
COMMENT ON COLUMN wiki_message_references.attachment_urls IS 'Array of Discord CDN URLs for message attachments (images, files, etc)';

CREATE INDEX idx_wiki_msg_refs_page ON wiki_message_references(wiki_page_id, message_timestamp DESC);
CREATE INDEX idx_wiki_msg_refs_message ON wiki_message_references(message_id);
