-- Remove lease coordination columns from discord_guilds table
DROP INDEX IF EXISTS idx_discord_guilds_last_member_sync;
DROP INDEX IF EXISTS idx_discord_guilds_sync_lease_expires;

ALTER TABLE discord_guilds 
    DROP COLUMN IF EXISTS sync_lease_holder,
    DROP COLUMN IF EXISTS sync_lease_expires_at,
    DROP COLUMN IF EXISTS last_member_sync;
