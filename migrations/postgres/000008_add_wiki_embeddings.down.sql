DROP INDEX IF EXISTS idx_wiki_pages_embedding;
DROP TRIGGER IF EXISTS trg_wiki_pages_embedding ON wiki_pages;
DROP FUNCTION IF EXISTS wiki_pages_set_embedding();
ALTER TABLE wiki_pages DROP COLUMN IF EXISTS embedding;
-- Extensions (vector, pg_gembed) are left installed; they may be used elsewhere.
