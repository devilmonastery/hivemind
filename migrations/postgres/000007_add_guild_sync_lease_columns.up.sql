-- Add lease coordination columns to discord_guilds table
ALTER TABLE discord_guilds 
    ADD COLUMN sync_lease_holder TEXT,
    ADD COLUMN sync_lease_expires_at TIMESTAMPTZ,
    ADD COLUMN last_member_sync TIMESTAMPTZ;

-- Index for finding expired leases
CREATE INDEX idx_discord_guilds_sync_lease_expires 
    ON discord_guilds(sync_lease_expires_at) 
    WHERE sync_lease_holder IS NOT NULL;

-- Index for finding guilds that need syncing
CREATE INDEX idx_discord_guilds_last_member_sync
    ON discord_guilds(last_member_sync);
