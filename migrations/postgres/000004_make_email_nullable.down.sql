-- Revert email to NOT NULL
-- Note: This will fail if any users have NULL email
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
