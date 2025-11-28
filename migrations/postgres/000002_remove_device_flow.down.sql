-- Rollback migration - recreate device flow table
CREATE TABLE device_flow_sessions (
    device_code VARCHAR(128) PRIMARY KEY,
    user_code VARCHAR(16) NOT NULL UNIQUE,
    client_type VARCHAR(32) NOT NULL DEFAULT 'cli',
    device_name VARCHAR(255),
    scopes JSONB NOT NULL DEFAULT '["hivemind:read","hivemind:write"]',
    user_id VARCHAR(64),
    api_token_id VARCHAR(64),
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Recreate indexes
CREATE INDEX idx_device_flow_user_code ON device_flow_sessions(user_code);
CREATE INDEX idx_device_flow_expires_at ON device_flow_sessions(expires_at);
CREATE INDEX idx_device_flow_user_id ON device_flow_sessions(user_id);
