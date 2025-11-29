-- Discord Users Table: Maps Discord identities to Hivemind users
CREATE TABLE discord_users (
    discord_id TEXT PRIMARY KEY,              -- Discord snowflake ID
    user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    discord_username TEXT NOT NULL,           -- username#discriminator or new @username
    discord_global_name TEXT,                 -- Display name
    avatar_url TEXT,                          -- Discord avatar URL
    linked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_seen TIMESTAMP,
    
    UNIQUE(user_id)                           -- One Discord account per Hivemind user
);

CREATE INDEX idx_discord_users_user_id ON discord_users(user_id);
CREATE INDEX idx_discord_users_last_seen ON discord_users(last_seen);

-- Discord Guilds Table: Guild-level configuration
CREATE TABLE discord_guilds (
    guild_id TEXT PRIMARY KEY,                -- Discord guild (server) ID
    guild_name TEXT NOT NULL,
    icon_url TEXT,
    owner_discord_id TEXT,                    -- Guild owner's Discord ID
    enabled BOOLEAN DEFAULT TRUE,             -- Can be disabled without deleting data
    settings JSONB DEFAULT '{}'::jsonb,       -- Bot-specific guild settings
    added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_activity TIMESTAMP
);

CREATE INDEX idx_discord_guilds_enabled ON discord_guilds(enabled);
CREATE INDEX idx_discord_guilds_last_activity ON discord_guilds(last_activity);

-- Wiki Pages Table: Guild knowledge base articles
CREATE TABLE wiki_pages (
    id TEXT PRIMARY KEY,                      -- Snowflake ID
    title TEXT NOT NULL,                      -- Wiki page title
    body TEXT NOT NULL,                       -- Markdown content
    
    -- Ownership
    author_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    guild_id TEXT NOT NULL REFERENCES discord_guilds(guild_id) ON DELETE CASCADE,
    channel_id TEXT,                          -- Channel where created (optional)
    channel_name TEXT,                        -- Channel name at creation time (snapshot)
    
    -- Metadata
    tags TEXT[] DEFAULT '{}',                 -- Array of tags (e.g., ['cli', 'production'])
    linked_page_ids TEXT[] DEFAULT '{}',      -- Wiki-style links to other pages
    
    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP                      -- Soft delete
);

CREATE INDEX idx_wiki_pages_author_id ON wiki_pages(author_id);
CREATE INDEX idx_wiki_pages_guild_id ON wiki_pages(guild_id);
CREATE INDEX idx_wiki_pages_channel_id ON wiki_pages(channel_id);
CREATE INDEX idx_wiki_pages_deleted_at ON wiki_pages(deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_wiki_pages_tags ON wiki_pages USING GIN(tags);
CREATE INDEX idx_wiki_pages_title_search ON wiki_pages USING gin(to_tsvector('english', title));
CREATE INDEX idx_wiki_pages_body_search ON wiki_pages USING gin(to_tsvector('english', body));
CREATE UNIQUE INDEX idx_wiki_pages_guild_title ON wiki_pages(guild_id, LOWER(title)) WHERE deleted_at IS NULL;

-- Notes Table: Private user notes
CREATE TABLE notes (
    id TEXT PRIMARY KEY,                      -- Snowflake ID
    title TEXT,                               -- Optional note title
    body TEXT NOT NULL,                       -- Note content (markdown)
    
    -- Ownership
    author_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    guild_id TEXT REFERENCES discord_guilds(guild_id) ON DELETE CASCADE, -- NULL for personal notes
    channel_id TEXT,                          -- Channel context (if guild-scoped)
    channel_name TEXT,                        -- Channel name at creation time (snapshot)
    
    -- Reference (optional)
    source_msg_id TEXT,                       -- Message that inspired this note (optional)
    source_channel_id TEXT,                   -- Channel of source message
    mentioned_user_ids TEXT[],                -- Discord user IDs mentioned in note
    
    -- Metadata
    tags TEXT[] DEFAULT '{}',                 -- Array of tags
    
    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP                      -- Soft delete
);

CREATE INDEX idx_notes_author_id ON notes(author_id);
CREATE INDEX idx_notes_guild_id ON notes(guild_id);
CREATE INDEX idx_notes_channel_id ON notes(channel_id);
CREATE INDEX idx_notes_deleted_at ON notes(deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_notes_tags ON notes USING GIN(tags);
CREATE INDEX idx_notes_body_search ON notes USING gin(to_tsvector('english', body));

-- Quotes Table: Memorable guild quotes
CREATE TABLE quotes (
    id TEXT PRIMARY KEY,                      -- Snowflake ID
    body TEXT NOT NULL,                       -- Quote text
    
    -- Ownership
    author_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, -- Who saved the quote
    guild_id TEXT NOT NULL REFERENCES discord_guilds(guild_id) ON DELETE CASCADE,
    
    -- Source message information
    source_msg_id TEXT NOT NULL,              -- Original Discord message ID
    source_channel_id TEXT NOT NULL,          -- Original channel ID
    source_channel_name TEXT,                 -- Channel name at quote time (snapshot)
    source_msg_author_discord_id TEXT,        -- Discord ID of who originally said it
    mentioned_user_ids TEXT[],                -- Discord user IDs mentioned in quote
    
    -- Metadata
    tags TEXT[] DEFAULT '{}',                 -- Array of tags
    
    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP                      -- Soft delete
);

CREATE INDEX idx_quotes_author_id ON quotes(author_id);
CREATE INDEX idx_quotes_guild_id ON quotes(guild_id);
CREATE INDEX idx_quotes_source_msg_id ON quotes(source_msg_id);
CREATE INDEX idx_quotes_source_msg_author ON quotes(source_msg_author_discord_id);
CREATE INDEX idx_quotes_deleted_at ON quotes(deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_quotes_tags ON quotes USING GIN(tags);
CREATE INDEX idx_quotes_body_search ON quotes USING gin(to_tsvector('english', body));
