-- Hivemind Initial Schema
-- Consolidated migration with all features

-- ============================================================================
-- Users table - core user identity
-- ============================================================================
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE,
    name TEXT NOT NULL,
    avatar_url TEXT,
    timezone TEXT,
    user_type TEXT DEFAULT 'oidc',
    role TEXT DEFAULT 'user',
    password_hash TEXT,
    disabled BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_seen TIMESTAMP
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_last_seen ON users(last_seen);
CREATE INDEX idx_users_user_type ON users(user_type);
CREATE INDEX idx_users_role ON users(role);

-- ============================================================================
-- Discord Users - Discord identity mapping
-- ============================================================================
CREATE TABLE discord_users (
    discord_id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    discord_username TEXT NOT NULL,
    discord_global_name TEXT,
    avatar_url TEXT,
    linked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_seen TIMESTAMP
);

CREATE INDEX idx_discord_users_user_id ON discord_users(user_id);
CREATE INDEX idx_discord_users_last_seen ON discord_users(last_seen);

-- ============================================================================
-- Discord Guilds - Guild-level configuration
-- ============================================================================
CREATE TABLE discord_guilds (
    guild_id TEXT PRIMARY KEY,
    guild_name TEXT NOT NULL,
    icon_url TEXT,
    owner_discord_id TEXT,
    enabled BOOLEAN DEFAULT TRUE,
    settings JSONB DEFAULT '{}'::jsonb,
    added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_activity TIMESTAMP
);

CREATE INDEX idx_discord_guilds_enabled ON discord_guilds(enabled);
CREATE INDEX idx_discord_guilds_last_activity ON discord_guilds(last_activity);

-- ============================================================================
-- Guild Members - ACL for guild membership
-- ============================================================================
CREATE TABLE guild_members (
    guild_id TEXT NOT NULL REFERENCES discord_guilds(guild_id) ON DELETE CASCADE,
    discord_id TEXT NOT NULL REFERENCES discord_users(discord_id) ON DELETE CASCADE,
    guild_nick TEXT,
    guild_avatar_hash TEXT,
    roles TEXT[] DEFAULT '{}',
    joined_at TIMESTAMP NOT NULL,
    synced_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_seen TIMESTAMP,
    
    PRIMARY KEY (guild_id, discord_id)
);

CREATE INDEX idx_guild_members_discord_id ON guild_members(discord_id);
CREATE INDEX idx_guild_members_synced_at ON guild_members(synced_at);
CREATE INDEX idx_guild_members_last_seen ON guild_members(last_seen);

-- ============================================================================
-- API tokens - for CLI and programmatic access
-- ============================================================================
CREATE TABLE api_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT UNIQUE NOT NULL,
    device_name TEXT NOT NULL,
    scopes TEXT NOT NULL DEFAULT '["articles:read","articles:write"]',
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_used TIMESTAMP,
    revoked_at TIMESTAMP
);

CREATE INDEX idx_api_tokens_user_id ON api_tokens(user_id);
CREATE INDEX idx_api_tokens_expires_at ON api_tokens(expires_at);
CREATE INDEX idx_api_tokens_revoked_at ON api_tokens(revoked_at) WHERE revoked_at IS NULL;
CREATE INDEX idx_api_tokens_token_hash ON api_tokens(token_hash);

-- ============================================================================
-- OIDC sessions - server-side storage of OAuth/OIDC state
-- ============================================================================
CREATE TABLE oidc_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    state TEXT,
    nonce TEXT,
    code_verifier TEXT,
    redirect_uri TEXT,
    scopes JSONB DEFAULT '[]'::jsonb,
    id_token TEXT,
    access_token TEXT,
    refresh_token TEXT,
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    last_refreshed TIMESTAMP
);

CREATE INDEX idx_oidc_sessions_user_id ON oidc_sessions(user_id);
CREATE INDEX idx_oidc_sessions_provider ON oidc_sessions(provider);
CREATE INDEX idx_oidc_sessions_expires_at ON oidc_sessions(expires_at);
CREATE INDEX idx_oidc_sessions_completed_at ON oidc_sessions(completed_at);

-- ============================================================================
-- Wiki Pages - Guild knowledge base articles
-- ============================================================================
CREATE TABLE wiki_pages (
    id TEXT PRIMARY KEY,
    title TEXT,
    body TEXT NOT NULL,
    author_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    guild_id TEXT NOT NULL REFERENCES discord_guilds(guild_id) ON DELETE CASCADE,
    channel_id TEXT,
    channel_name TEXT,
    tags TEXT[] DEFAULT '{}',
    linked_page_ids TEXT[] DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_wiki_pages_author_id ON wiki_pages(author_id);
CREATE INDEX idx_wiki_pages_guild_id ON wiki_pages(guild_id);
CREATE INDEX idx_wiki_pages_channel_id ON wiki_pages(channel_id);
CREATE INDEX idx_wiki_pages_deleted_at ON wiki_pages(deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_wiki_pages_tags ON wiki_pages USING GIN(tags);

-- Add hybrid search vector column (english stemmed + simple literal)
ALTER TABLE wiki_pages
ADD COLUMN search_vector tsvector
GENERATED ALWAYS AS (
    setweight(to_tsvector('english', COALESCE(title, '')), 'A') ||
    setweight(to_tsvector('simple', COALESCE(title, '')), 'B') ||
    setweight(to_tsvector('english', body), 'C') ||
    setweight(to_tsvector('simple', body), 'D')
) STORED;

CREATE INDEX idx_wiki_pages_search_vector ON wiki_pages USING GIN(search_vector);

-- Wiki message references
CREATE TABLE wiki_message_references (
    id TEXT PRIMARY KEY,
    wiki_page_id TEXT NOT NULL REFERENCES wiki_pages(id) ON DELETE CASCADE,
    message_id TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    guild_id TEXT NOT NULL REFERENCES discord_guilds(guild_id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    author_id TEXT NOT NULL,
    author_username TEXT NOT NULL,
    author_display_name TEXT,
    author_avatar_url TEXT,
    message_timestamp TIMESTAMP NOT NULL,
    attachment_urls TEXT[] DEFAULT '{}',
    attachment_metadata JSONB,
    added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    added_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    
    UNIQUE (wiki_page_id, message_id)
);

CREATE INDEX idx_wiki_message_references_wiki_page_id ON wiki_message_references(wiki_page_id);
CREATE INDEX idx_wiki_message_references_guild_id ON wiki_message_references(guild_id);
CREATE INDEX idx_wiki_message_references_message_id ON wiki_message_references(message_id);
CREATE INDEX idx_wiki_message_references_author_id ON wiki_message_references(author_id);

-- Wiki titles canonical table
CREATE TABLE wiki_titles (
    id TEXT PRIMARY KEY,
    guild_id TEXT NOT NULL REFERENCES discord_guilds(guild_id) ON DELETE CASCADE,
    display_title TEXT NOT NULL,
    page_slug TEXT NOT NULL,
    page_id TEXT NOT NULL REFERENCES wiki_pages(id) ON DELETE CASCADE,
    is_canonical BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_by_merge BOOLEAN NOT NULL DEFAULT false,
    
    UNIQUE (guild_id, page_slug)
);

CREATE INDEX idx_wiki_titles_page_id ON wiki_titles(page_id);
CREATE INDEX idx_wiki_titles_is_canonical ON wiki_titles(is_canonical);
CREATE INDEX idx_wiki_titles_created_at ON wiki_titles(created_at);

-- ============================================================================
-- Notes - Private user notes
-- ============================================================================
CREATE TABLE notes (
    id TEXT PRIMARY KEY,
    title TEXT,
    body TEXT NOT NULL,
    author_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    guild_id TEXT REFERENCES discord_guilds(guild_id) ON DELETE CASCADE,
    channel_id TEXT,
    channel_name TEXT,
    source_msg_id TEXT,
    source_channel_id TEXT,
    mentioned_user_ids TEXT[],
    tags TEXT[] DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_notes_author_id ON notes(author_id);
CREATE INDEX idx_notes_guild_id ON notes(guild_id);
CREATE INDEX idx_notes_channel_id ON notes(channel_id);
CREATE INDEX idx_notes_deleted_at ON notes(deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_notes_tags ON notes USING GIN(tags);

-- Add hybrid search vector column (english stemmed + simple literal)
ALTER TABLE notes
ADD COLUMN search_vector tsvector
GENERATED ALWAYS AS (
    setweight(to_tsvector('english', COALESCE(title, '')), 'A') ||
    setweight(to_tsvector('simple', COALESCE(title, '')), 'B') ||
    setweight(to_tsvector('english', body), 'C') ||
    setweight(to_tsvector('simple', body), 'D')
) STORED;

CREATE INDEX idx_notes_search_vector ON notes USING GIN(search_vector);

-- Note message references
CREATE TABLE note_message_references (
    id TEXT PRIMARY KEY,
    note_id TEXT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
    message_id TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    guild_id TEXT REFERENCES discord_guilds(guild_id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    author_id TEXT NOT NULL,
    author_username TEXT NOT NULL,
    author_display_name TEXT,
    author_avatar_url TEXT,
    message_timestamp TIMESTAMP NOT NULL,
    attachment_metadata JSONB,
    added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE (note_id, message_id)
);

CREATE INDEX idx_note_message_references_note_id ON note_message_references(note_id);
CREATE INDEX idx_note_message_references_guild_id ON note_message_references(guild_id);
CREATE INDEX idx_note_message_references_message_id ON note_message_references(message_id);
CREATE INDEX idx_note_message_references_author_id ON note_message_references(author_id);

-- ============================================================================
-- Quotes - Memorable guild quotes
-- ============================================================================
CREATE TABLE quotes (
    id TEXT PRIMARY KEY,
    body TEXT NOT NULL,
    author_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    guild_id TEXT NOT NULL REFERENCES discord_guilds(guild_id) ON DELETE CASCADE,
    source_msg_id TEXT NOT NULL,
    source_channel_id TEXT NOT NULL,
    source_channel_name TEXT,
    source_msg_author_discord_id TEXT,
    source_msg_author_username TEXT,
    mentioned_user_ids TEXT[],
    attachment_url TEXT,
    attachment_filename TEXT,
    attachment_content_type TEXT,
    attachment_size_bytes INTEGER,
    tags TEXT[] DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_quotes_author_id ON quotes(author_id);
CREATE INDEX idx_quotes_guild_id ON quotes(guild_id);
CREATE INDEX idx_quotes_source_msg_id ON quotes(source_msg_id);
CREATE INDEX idx_quotes_source_msg_author ON quotes(source_msg_author_discord_id);
CREATE INDEX idx_quotes_deleted_at ON quotes(deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_quotes_tags ON quotes USING GIN(tags);

-- Add hybrid search vector column (english stemmed + simple literal)
ALTER TABLE quotes
ADD COLUMN search_vector tsvector
GENERATED ALWAYS AS (
    setweight(to_tsvector('english', body), 'A') ||
    setweight(to_tsvector('simple', body), 'B')
) STORED;

CREATE INDEX idx_quotes_search_vector ON quotes USING GIN(search_vector);

-- ============================================================================
-- Audit logs - tracking authentication and authorization events
-- ============================================================================
CREATE TABLE audit_logs (
    id TEXT PRIMARY KEY,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    resource_type TEXT,
    resource_id TEXT,
    metadata TEXT,
    ip_address TEXT,
    user_agent TEXT,
    success BOOLEAN DEFAULT TRUE
);

CREATE INDEX idx_audit_logs_timestamp ON audit_logs(timestamp);
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_resource_type ON audit_logs(resource_type);
CREATE INDEX idx_audit_logs_success ON audit_logs(success);

-- ============================================================================
-- Initial Data - Development bot user
-- ============================================================================
INSERT INTO users (id, email, name, role, user_type, disabled)
VALUES ('bot-dev', '', 'Development Bot', 'bot', 'oidc', false)
ON CONFLICT (id) DO NOTHING;
