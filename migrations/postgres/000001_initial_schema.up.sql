-- Hivemind Initial Schema
-- Consolidated migration for Discord-based authentication and content management

-- ============================================================================
-- Users table - core user identity
-- ============================================================================
CREATE TABLE users (
    id TEXT PRIMARY KEY,                    -- Snowflake ID
    email TEXT UNIQUE,                      -- Nullable for Discord-first users
    name TEXT NOT NULL,
    avatar_url TEXT,                        -- Profile picture URL
    timezone TEXT,                          -- IANA Time Zone format (e.g., America/Chicago)
    user_type TEXT DEFAULT 'oidc',         -- 'oidc', 'local', 'system'
    role TEXT DEFAULT 'user',              -- 'user', 'admin', 'bot'
    password_hash TEXT,                     -- only for local users (bcrypt hash)
    disabled BOOLEAN DEFAULT FALSE,        -- for soft-disabling users
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_seen TIMESTAMP
);

COMMENT ON COLUMN users.timezone IS 'User''s preferred timezone in IANA Time Zone format (e.g., America/Chicago, Europe/Berlin)';
COMMENT ON COLUMN users.email IS 'Nullable for Discord-first users; email can be added later via OAuth';

-- Indexes for users table
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_last_seen ON users(last_seen);
CREATE INDEX idx_users_user_type ON users(user_type);
CREATE INDEX idx_users_role ON users(role);

-- ============================================================================
-- Discord Users - Discord identity mapping (replaces multi-provider identities)
-- ============================================================================
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

-- ============================================================================
-- Discord Guilds - Guild-level configuration
-- ============================================================================
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

-- ============================================================================
-- API tokens - for CLI and programmatic access
-- ============================================================================
CREATE TABLE api_tokens (
    id TEXT PRIMARY KEY,                    -- Snowflake ID
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT UNIQUE NOT NULL,       -- SHA256 hash of the actual token
    device_name TEXT NOT NULL,             -- user-provided name like "laptop", "server-01"
    scopes TEXT NOT NULL DEFAULT '["articles:read","articles:write"]', -- JSON array
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_used TIMESTAMP,
    revoked_at TIMESTAMP                   -- soft delete for audit trail
);

-- Indexes for api_tokens table
CREATE INDEX idx_api_tokens_user_id ON api_tokens(user_id);
CREATE INDEX idx_api_tokens_expires_at ON api_tokens(expires_at);
CREATE INDEX idx_api_tokens_revoked_at ON api_tokens(revoked_at) WHERE revoked_at IS NULL;
CREATE INDEX idx_api_tokens_token_hash ON api_tokens(token_hash);

-- ============================================================================
-- OIDC sessions - server-side storage of OAuth/OIDC state
-- ============================================================================
CREATE TABLE oidc_sessions (
    id TEXT PRIMARY KEY,                    -- Snowflake ID
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,                -- which OIDC provider this session is for
    
    -- OAuth flow state
    state TEXT,                             -- CSRF protection state parameter
    nonce TEXT,                             -- OIDC nonce for replay protection
    code_verifier TEXT,                     -- PKCE code verifier
    redirect_uri TEXT,                      -- OAuth redirect URI
    scopes JSONB DEFAULT '[]'::jsonb,       -- Requested OAuth scopes
    
    -- Tokens (should be encrypted at rest in production)
    id_token TEXT,                          -- OIDC ID token (JWT)
    access_token TEXT,                      -- OAuth access token
    refresh_token TEXT,                     -- OAuth refresh token
    
    -- Timestamps
    expires_at TIMESTAMP,                   -- when refresh token expires
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,                 -- when OAuth flow completed successfully
    last_refreshed TIMESTAMP                -- last time tokens were refreshed
);

-- Indexes for oidc_sessions table
CREATE INDEX idx_oidc_sessions_user_id ON oidc_sessions(user_id);
CREATE INDEX idx_oidc_sessions_provider ON oidc_sessions(provider);
CREATE INDEX idx_oidc_sessions_expires_at ON oidc_sessions(expires_at);
CREATE INDEX idx_oidc_sessions_completed_at ON oidc_sessions(completed_at);

-- ============================================================================
-- Wiki Pages - Guild knowledge base articles
-- ============================================================================
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

-- ============================================================================
-- Notes - Private user notes
-- ============================================================================
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

-- ============================================================================
-- Quotes - Memorable guild quotes
-- ============================================================================
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

-- ============================================================================
-- Audit logs - tracking authentication and authorization events
-- ============================================================================
CREATE TABLE audit_logs (
    id TEXT PRIMARY KEY,                    -- Snowflake ID
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    user_id TEXT REFERENCES users(id) ON DELETE SET NULL, -- nullable in case user is deleted
    action TEXT NOT NULL,                  -- 'login', 'create_token', 'delete_article', etc.
    resource_type TEXT,                    -- 'user', 'token', 'article', 'session', 'guild'
    resource_id TEXT,                      -- ID of the affected resource
    metadata TEXT,                         -- JSON object with additional context
    ip_address TEXT,
    user_agent TEXT,
    success BOOLEAN DEFAULT TRUE
);

-- Indexes for audit_logs table
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
