-- Convert user_display_names from VIEW to TABLE for better performance
-- This allows incremental updates during guild sync instead of recomputing joins

-- Drop the existing view
DROP VIEW IF EXISTS user_display_names;

-- Create the table with the same structure
CREATE TABLE user_display_names (
    discord_id TEXT NOT NULL,
    guild_id TEXT,
    display_name TEXT NOT NULL,
    guild_nick TEXT,
    discord_global_name TEXT,
    discord_username TEXT NOT NULL,
    PRIMARY KEY (discord_id, guild_id)
);

-- Create indexes for efficient lookups
CREATE INDEX idx_user_display_names_discord_id ON user_display_names(discord_id);
CREATE INDEX idx_user_display_names_guild_id ON user_display_names(guild_id);

-- Add foreign key constraint to guild_members for automatic cleanup
-- When a member is removed from guild_members, their display name entry is also removed
ALTER TABLE user_display_names 
    ADD CONSTRAINT fk_user_display_names_guild_member 
    FOREIGN KEY (discord_id, guild_id) 
    REFERENCES guild_members(discord_id, guild_id) 
    ON DELETE CASCADE;

-- Populate the table with existing data from discord_users and guild_members
INSERT INTO user_display_names (
    discord_id, 
    guild_id, 
    display_name,
    guild_nick,
    discord_global_name,
    discord_username
)
SELECT 
    du.discord_id,
    gm.guild_id,
    COALESCE(gm.guild_nick, du.discord_global_name, du.discord_username) AS display_name,
    gm.guild_nick,
    du.discord_global_name,
    du.discord_username
FROM discord_users du
LEFT JOIN guild_members gm ON du.discord_id = gm.discord_id
WHERE gm.guild_id IS NOT NULL; -- Only include guild members, not standalone discord_users

-- Add comment explaining maintenance
COMMENT ON TABLE user_display_names IS 
'Denormalized display names for guild members. Maintained automatically by bot guild sync process. Provides guild_nick > discord_global_name > discord_username priority resolution.';
