-- Remove user_identities table (replaced by discord_users for Discord-only app)
-- Keep oidc_sessions for OAuth refresh token storage (needed by web/CLI)

-- Drop user_identities (multi-provider identity mapping - not needed for Discord-only)
DROP TABLE IF EXISTS user_identities CASCADE;

-- Note: We keep oidc_sessions for storing Discord OAuth refresh tokens
-- Note: discord_users is the single identity mapping table for Discord
