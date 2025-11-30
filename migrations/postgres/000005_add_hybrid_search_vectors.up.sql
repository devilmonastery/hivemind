-- Add hybrid search vectors with weighted ranking
-- Combines 'english' (stemmed) and 'simple' (literal) dictionaries
-- Weights: A (highest) > B > C > D (lowest)

-- ============================================================================
-- Quotes: body-only search vector
-- ============================================================================

-- Drop old index
DROP INDEX IF EXISTS idx_quotes_body_search;

-- Add new search_vector column with hybrid weighted approach
ALTER TABLE quotes
ADD COLUMN search_vector tsvector
GENERATED ALWAYS AS (
    setweight(to_tsvector('english', body), 'A') ||
    setweight(to_tsvector('simple', body), 'B')
) STORED;

-- Create GIN index on new search vector
CREATE INDEX idx_quotes_search_vector ON quotes USING GIN(search_vector);

-- ============================================================================
-- Notes: title+body combined search vector
-- ============================================================================

-- Drop old index
DROP INDEX IF EXISTS idx_notes_body_search;

-- Add new search_vector column with hybrid weighted approach
-- Title gets higher weights (A/B), body gets lower weights (C/D)
ALTER TABLE notes
ADD COLUMN search_vector tsvector
GENERATED ALWAYS AS (
    setweight(to_tsvector('english', COALESCE(title, '')), 'A') ||
    setweight(to_tsvector('simple', COALESCE(title, '')), 'B') ||
    setweight(to_tsvector('english', body), 'C') ||
    setweight(to_tsvector('simple', body), 'D')
) STORED;

-- Create GIN index on new search vector
CREATE INDEX idx_notes_search_vector ON notes USING GIN(search_vector);

-- ============================================================================
-- Wiki Pages: title+body combined search vector
-- ============================================================================

-- Drop old indexes
DROP INDEX IF EXISTS idx_wiki_pages_title_search;
DROP INDEX IF EXISTS idx_wiki_pages_body_search;

-- Add new search_vector column with hybrid weighted approach
-- Title gets higher weights (A/B), body gets lower weights (C/D)
ALTER TABLE wiki_pages
ADD COLUMN search_vector tsvector
GENERATED ALWAYS AS (
    setweight(to_tsvector('english', title), 'A') ||
    setweight(to_tsvector('simple', title), 'B') ||
    setweight(to_tsvector('english', body), 'C') ||
    setweight(to_tsvector('simple', body), 'D')
) STORED;

-- Create GIN index on new search vector
CREATE INDEX idx_wiki_pages_search_vector ON wiki_pages USING GIN(search_vector);
