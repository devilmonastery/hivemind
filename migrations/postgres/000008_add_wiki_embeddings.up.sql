-- Semantic search for wiki pages via in-database embeddings.
-- Requires the custom postgres image (pgvector + pg_gembed) with the MiniLM model
-- bundled; this migration will fail on a vanilla postgres image.

CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_gembed;

ALTER TABLE wiki_pages ADD COLUMN embedding vector(384);

-- Embed title+body on write using the in-DB MiniLM model (384-dim). Truncation is
-- generous; the model caps its own input length internally.
CREATE OR REPLACE FUNCTION wiki_pages_set_embedding() RETURNS trigger AS $$
BEGIN
    NEW.embedding := embed_text(
        'embed_anything',
        'sentence-transformers/all-MiniLM-L6-v2',
        left(COALESCE(NEW.title, '') || ' ' || NEW.body, 8000)
    );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_wiki_pages_embedding
    BEFORE INSERT OR UPDATE OF title, body ON wiki_pages
    FOR EACH ROW EXECUTE FUNCTION wiki_pages_set_embedding();

-- Backfill existing rows (fills embedding directly; does not fire the trigger
-- since title/body are unchanged).
UPDATE wiki_pages
SET embedding = embed_text(
        'embed_anything',
        'sentence-transformers/all-MiniLM-L6-v2',
        left(COALESCE(title, '') || ' ' || body, 8000)
    )
WHERE embedding IS NULL AND deleted_at IS NULL;

-- Approximate nearest-neighbour index for cosine distance (<=>).
CREATE INDEX idx_wiki_pages_embedding ON wiki_pages
    USING hnsw (embedding vector_cosine_ops);
