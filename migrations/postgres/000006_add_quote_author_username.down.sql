-- Rollback: Remove source_msg_author_username column from quotes table

ALTER TABLE quotes
DROP COLUMN IF EXISTS source_msg_author_username;
