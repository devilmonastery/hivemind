-- Remove attachment metadata column

ALTER TABLE wiki_message_references 
DROP COLUMN IF EXISTS attachment_metadata;
