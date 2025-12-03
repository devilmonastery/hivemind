-- Revert avatar hash support changes

-- Remove avatar hash columns from user_display_names
ALTER TABLE user_display_names 
    DROP COLUMN IF EXISTS guild_avatar_hash,
    DROP COLUMN IF EXISTS user_avatar_hash;

-- Restore discord_users to use avatar_url instead of avatar_hash
ALTER TABLE discord_users DROP COLUMN IF EXISTS avatar_hash;
ALTER TABLE discord_users ADD COLUMN avatar_url TEXT;
