-- Make email nullable for Discord-first users
-- Users can exist with just Discord ID, email added later via OAuth
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;

-- Email remains unique when present (prevents duplicate real emails)
-- The existing UNIQUE constraint on email still applies
