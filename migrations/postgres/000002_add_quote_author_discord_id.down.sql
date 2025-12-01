-- Remove author_discord_id column from quotes table
DROP INDEX IF EXISTS idx_quotes_author_discord_id;
ALTER TABLE quotes DROP COLUMN IF EXISTS author_discord_id;
