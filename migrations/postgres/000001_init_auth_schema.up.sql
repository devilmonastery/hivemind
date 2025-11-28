-- Initial schema for Hivemind authentication system

-- ============================================================================
-- Users table - core user identity
-- ============================================================================
CREATE TABLE users (
    id TEXT PRIMARY KEY,                    -- Snowflake ID
    email TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    avatar_url TEXT,                        -- Profile picture URL
    timezone TEXT,                          -- IANA Time Zone format (e.g., America/Chicago)
    user_type TEXT DEFAULT 'oidc',         -- 'oidc', 'local', 'system'
    role TEXT DEFAULT 'user',              -- 'user', 'admin'
    password_hash TEXT,                     -- only for local users (bcrypt hash)
    disabled BOOLEAN DEFAULT FALSE,        -- for soft-disabling users
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_seen TIMESTAMP
);

COMMENT ON COLUMN users.timezone IS 'User''s preferred timezone in IANA Time Zone format (e.g., America/Chicago, Europe/Berlin)';

-- Indexes for users table
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_last_seen ON users(last_seen);
CREATE INDEX idx_users_user_type ON users(user_type);
CREATE INDEX idx_users_role ON users(role);

-- ============================================================================
-- User identities - multi-provider authentication support
-- ============================================================================
CREATE TABLE user_identities (
    identity_id TEXT PRIMARY KEY,                   -- Snowflake ID
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,                         -- 'google', 'github', 'okta', 'discord', etc.
    external_id TEXT NOT NULL,                      -- Provider's 'sub' claim (stable identifier)
    email TEXT NOT NULL,                            -- Email from this provider
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,  -- Whether provider verified the email
    display_name TEXT,                              -- Name from provider
    profile_picture_url TEXT,                       -- Profile picture from provider
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    last_login_at TIMESTAMP,                        -- Last time this identity was used to login
    
    -- Ensure one identity per provider account
    UNIQUE(provider, external_id)
);

-- Indexes for user_identities
CREATE INDEX idx_user_identities_user_id ON user_identities(user_id);
CREATE INDEX idx_user_identities_provider_external_id ON user_identities(provider, external_id);
CREATE INDEX idx_user_identities_email ON user_identities(email);
CREATE INDEX idx_user_identities_last_login ON user_identities(last_login_at);

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
-- Device flow sessions - for CLI authentication via device code flow
-- ============================================================================
CREATE TABLE device_flow_sessions (
    device_code TEXT PRIMARY KEY,          -- server-generated device code
    user_code TEXT UNIQUE NOT NULL,       -- human-readable code user enters
    client_type TEXT NOT NULL DEFAULT 'cli',
    device_name TEXT,                      -- user can name their device during flow
    scopes TEXT NOT NULL DEFAULT '["articles:read","articles:write"]', -- JSON array
    user_id TEXT REFERENCES users(id) ON DELETE CASCADE, -- null until user completes auth
    api_token_id TEXT REFERENCES api_tokens(id), -- null until token issued
    expires_at TIMESTAMP NOT NULL,        -- device codes expire quickly (10-15 minutes)
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for device_flow_sessions table
CREATE INDEX idx_device_flow_user_code ON device_flow_sessions(user_code);
CREATE INDEX idx_device_flow_expires_at ON device_flow_sessions(expires_at);
CREATE INDEX idx_device_flow_user_id ON device_flow_sessions(user_id);

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
