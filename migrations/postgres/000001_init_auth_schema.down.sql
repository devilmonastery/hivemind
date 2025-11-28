-- Rollback initial Hivemind authentication schema

-- Drop tables in reverse dependency order

-- Drop indexes and tables for audit_logs
DROP INDEX IF EXISTS idx_audit_logs_timestamp;
DROP INDEX IF EXISTS idx_audit_logs_user_id;
DROP INDEX IF EXISTS idx_audit_logs_action;
DROP INDEX IF EXISTS idx_audit_logs_resource_type;
DROP INDEX IF EXISTS idx_audit_logs_success;
DROP TABLE IF EXISTS audit_logs;

-- Drop indexes and tables for oidc_sessions
DROP INDEX IF EXISTS idx_oidc_sessions_user_id;
DROP INDEX IF EXISTS idx_oidc_sessions_provider;
DROP INDEX IF EXISTS idx_oidc_sessions_expires_at;
DROP INDEX IF EXISTS idx_oidc_sessions_completed_at;
DROP TABLE IF EXISTS oidc_sessions;

-- Drop indexes and tables for device_flow_sessions
DROP INDEX IF EXISTS idx_device_flow_user_code;
DROP INDEX IF EXISTS idx_device_flow_expires_at;
DROP INDEX IF EXISTS idx_device_flow_user_id;
DROP TABLE IF EXISTS device_flow_sessions;

-- Drop indexes and tables for api_tokens
DROP INDEX IF EXISTS idx_api_tokens_user_id;
DROP INDEX IF EXISTS idx_api_tokens_expires_at;
DROP INDEX IF EXISTS idx_api_tokens_revoked_at;
DROP INDEX IF EXISTS idx_api_tokens_token_hash;
DROP TABLE IF EXISTS api_tokens;

-- Drop indexes and tables for user_identities
DROP INDEX IF EXISTS idx_user_identities_user_id;
DROP INDEX IF EXISTS idx_user_identities_provider_external_id;
DROP INDEX IF EXISTS idx_user_identities_email;
DROP INDEX IF EXISTS idx_user_identities_last_login;
DROP TABLE IF EXISTS user_identities;

-- Drop indexes and tables for users
DROP INDEX IF EXISTS idx_users_email;
DROP INDEX IF EXISTS idx_users_last_seen;
DROP INDEX IF EXISTS idx_users_user_type;
DROP INDEX IF EXISTS idx_users_role;
DROP TABLE IF EXISTS users;
