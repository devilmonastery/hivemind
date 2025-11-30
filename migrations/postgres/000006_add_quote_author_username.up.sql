-- Add source_msg_author_username column to quotes table
-- This stores a snapshot of the Discord username at quote creation time

ALTER TABLE quotes
ADD COLUMN source_msg_author_username TEXT;

-- Backfill existing quotes with usernames from discord_users table where available
UPDATE quotes q
SET source_msg_author_username = du.discord_username
FROM discord_users du
WHERE q.source_msg_author_discord_id = du.discord_id
  AND q.source_msg_author_username IS NULL;
