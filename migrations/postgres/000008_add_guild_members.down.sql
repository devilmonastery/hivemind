-- Rollback guild members tracking

-- Remove last_member_sync column from discord_guilds
ALTER TABLE discord_guilds DROP COLUMN IF EXISTS last_member_sync;

-- Drop guild_members table (cascades handled by ON DELETE CASCADE in foreign keys)
DROP TABLE IF EXISTS guild_members;
