-- Rollback hybrid search vectors migration
-- Restores original single-dictionary search indexes

-- ============================================================================
-- Wiki Pages: restore separate title and body indexes
-- ============================================================================

-- Drop new search vector
DROP INDEX IF EXISTS idx_wiki_pages_search_vector;
ALTER TABLE wiki_pages DROP COLUMN IF EXISTS search_vector;

-- Restore old indexes
CREATE INDEX idx_wiki_pages_title_search ON wiki_pages USING GIN(to_tsvector('english', title));
CREATE INDEX idx_wiki_pages_body_search ON wiki_pages USING GIN(to_tsvector('english', body));

-- ============================================================================
-- Notes: restore body-only index
-- ============================================================================

-- Drop new search vector
DROP INDEX IF EXISTS idx_notes_search_vector;
ALTER TABLE notes DROP COLUMN IF EXISTS search_vector;

-- Restore old index
CREATE INDEX idx_notes_body_search ON notes USING GIN(to_tsvector('english', body));

-- ============================================================================
-- Quotes: restore body-only index
-- ============================================================================

-- Drop new search vector
DROP INDEX IF EXISTS idx_quotes_search_vector;
ALTER TABLE quotes DROP COLUMN IF EXISTS search_vector;

-- Restore old index
CREATE INDEX idx_quotes_body_search ON quotes USING GIN(to_tsvector('english', body));
