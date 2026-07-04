DROP INDEX IF EXISTS idx_quotes_embedding;
DROP TRIGGER IF EXISTS trg_quotes_embedding ON quotes;
DROP FUNCTION IF EXISTS quotes_set_embedding();
ALTER TABLE quotes DROP COLUMN IF EXISTS embedding;
-- Extensions (vector, pg_gembed) are left installed; they may be used elsewhere.
