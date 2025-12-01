-- Add guild_members table for tracking Discord guild memberships
-- This enables ACL checks to ensure users can only view content from guilds they're members of

CREATE TABLE guild_members (
    guild_id TEXT NOT NULL REFERENCES discord_guilds(guild_id) ON DELETE CASCADE,
    discord_id TEXT NOT NULL REFERENCES discord_users(discord_id) ON DELETE CASCADE,
    
    -- Guild-specific user data
    guild_nick TEXT,                      -- Server nickname (Member.Nick)
    guild_avatar_hash TEXT,               -- Server-specific avatar hash (Member.Avatar)
    roles TEXT[] DEFAULT '{}',            -- Array of role IDs member has
    joined_at TIMESTAMP NOT NULL,         -- When member joined guild
    
    -- Sync metadata
    synced_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,  -- Last time this record was synced
    last_seen TIMESTAMP,                  -- Last bot interaction from this member
    
    PRIMARY KEY (guild_id, discord_id)
);

-- Index for looking up all guilds a user is a member of
CREATE INDEX idx_guild_members_discord_id ON guild_members(discord_id);

-- Index for finding stale records that need re-syncing
CREATE INDEX idx_guild_members_synced_at ON guild_members(synced_at);

-- GIN index for efficient role-based queries (future use)
CREATE INDEX idx_guild_members_roles ON guild_members USING GIN(roles);

-- Add last_member_sync timestamp to discord_guilds for tracking sync status
ALTER TABLE discord_guilds ADD COLUMN last_member_sync TIMESTAMP;

-- Comments for documentation
COMMENT ON TABLE guild_members IS 'Discord guild memberships - enables ACL checks for guild-scoped content';
COMMENT ON COLUMN guild_members.guild_nick IS 'Server-specific nickname if set, otherwise NULL (use discord_users.discord_username)';
COMMENT ON COLUMN guild_members.guild_avatar_hash IS 'Server-specific avatar hash if set, otherwise NULL (use discord_users avatar)';
COMMENT ON COLUMN guild_members.roles IS 'Array of Discord role IDs for future role-based permissions';
COMMENT ON COLUMN guild_members.synced_at IS 'Last time this membership was confirmed via Discord API';
COMMENT ON COLUMN guild_members.last_seen IS 'Last time this member interacted with bot via slash commands';
