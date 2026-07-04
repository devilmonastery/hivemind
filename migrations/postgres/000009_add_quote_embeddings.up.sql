-- Semantic search for quotes via in-database embeddings.
-- Requires the custom postgres image (pgvector + pg_gembed) with the MiniLM model
-- bundled; this migration will fail on a vanilla postgres image.

CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_gembed;

ALTER TABLE quotes ADD COLUMN embedding vector(384);

-- Embed the quote body on write using the in-DB MiniLM model (384-dim). Truncation
-- is generous; the model caps its own input length internally.
CREATE OR REPLACE FUNCTION quotes_set_embedding() RETURNS trigger AS $$
BEGIN
    NEW.embedding := embed_text(
        'embed_anything',
        'sentence-transformers/all-MiniLM-L6-v2',
        left(NEW.body, 8000)
    );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_quotes_embedding
    BEFORE INSERT OR UPDATE OF body ON quotes
    FOR EACH ROW EXECUTE FUNCTION quotes_set_embedding();

-- Backfill existing rows (fills embedding directly; does not fire the trigger
-- since body is unchanged).
UPDATE quotes
SET embedding = embed_text(
        'embed_anything',
        'sentence-transformers/all-MiniLM-L6-v2',
        left(body, 8000)
    )
WHERE embedding IS NULL AND deleted_at IS NULL;

-- Approximate nearest-neighbour index for cosine distance (<=>).
CREATE INDEX idx_quotes_embedding ON quotes
    USING hnsw (embedding vector_cosine_ops);
