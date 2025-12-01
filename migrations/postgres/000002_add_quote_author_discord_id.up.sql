-- Add author_discord_id column to quotes table
ALTER TABLE quotes
ADD COLUMN author_discord_id TEXT;

-- Populate the new column with Discord IDs from the discord_users table
UPDATE quotes q
SET author_discord_id = du.discord_id
FROM discord_users du
WHERE q.author_id = du.user_id;

-- Create index for faster lookups
CREATE INDEX idx_quotes_author_discord_id ON quotes(author_discord_id);
