-- Add avatar hash support to user_display_names and discord_users tables

-- Part 1: Update discord_users to store avatar hash instead of URL
ALTER TABLE discord_users DROP COLUMN IF EXISTS avatar_url;
ALTER TABLE discord_users ADD COLUMN avatar_hash TEXT;

-- Part 2: Add avatar hash columns to user_display_names for denormalization
ALTER TABLE user_display_names 
    ADD COLUMN guild_avatar_hash TEXT,
    ADD COLUMN user_avatar_hash TEXT;

-- Part 3: Populate existing rows from current data
-- This will be NULL for rows where the source data doesn't exist yet
-- Next bot sync will populate everything correctly
UPDATE user_display_names udn
SET 
    guild_avatar_hash = gm.guild_avatar_hash,
    user_avatar_hash = du.avatar_hash
FROM guild_members gm
JOIN discord_users du ON gm.discord_id = du.discord_id
WHERE udn.discord_id = gm.discord_id 
  AND udn.guild_id = gm.guild_id;

-- Add comment explaining the avatar hash columns
COMMENT ON COLUMN user_display_names.guild_avatar_hash IS 
'Guild-specific avatar hash from guild_members. Takes priority over user_avatar_hash.';

COMMENT ON COLUMN user_display_names.user_avatar_hash IS 
'Global user avatar hash from discord_users. Fallback when guild_avatar_hash is not set.';

COMMENT ON COLUMN discord_users.avatar_hash IS 
'Discord avatar hash for constructing CDN URLs. URL construction happens in presentation layer.';
