-- Drop note_message_references table

DROP INDEX IF EXISTS idx_note_msg_refs_author_id;
DROP INDEX IF EXISTS idx_note_msg_refs_message_id;
DROP INDEX IF EXISTS idx_note_msg_refs_note_id;

DROP TABLE IF EXISTS note_message_references;
