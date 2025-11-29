-- Add attachment metadata to store content types and filenames

ALTER TABLE wiki_message_references 
ADD COLUMN attachment_metadata JSONB;

COMMENT ON COLUMN wiki_message_references.attachment_metadata IS 'Array of attachment metadata objects with url, content_type, filename, size, etc';

-- Example structure: [{"url": "...", "content_type": "image/png", "filename": "...", "size": 12345}]
