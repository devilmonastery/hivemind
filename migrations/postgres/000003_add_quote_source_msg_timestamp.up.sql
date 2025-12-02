-- Add source_msg_timestamp column to quotes table
-- This stores the timestamp of the original Discord message that was quoted

ALTER TABLE quotes
ADD COLUMN source_msg_timestamp TIMESTAMP;

-- Backfill timestamps by extracting from Discord snowflake IDs
-- Discord snowflakes encode timestamp in first 42 bits (ms since Discord epoch: Jan 1, 2015)
-- Formula: ((snowflake >> 22) + 1420070400000) / 1000 = Unix timestamp in seconds
UPDATE quotes 
SET source_msg_timestamp = TO_TIMESTAMP(
    ((source_msg_id::bigint >> 22) + 1420070400000) / 1000.0
)
WHERE source_msg_id IS NOT NULL 
  AND source_msg_id != '' 
  AND source_msg_id ~ '^[0-9]+$'; -- Only process valid numeric snowflakes

-- Create index for sorting and filtering quotes by message timestamp
CREATE INDEX idx_quotes_source_msg_timestamp ON quotes(source_msg_timestamp);
