-- Remove source_msg_timestamp column and index

DROP INDEX IF EXISTS idx_quotes_source_msg_timestamp;

ALTER TABLE quotes
DROP COLUMN IF EXISTS source_msg_timestamp;
