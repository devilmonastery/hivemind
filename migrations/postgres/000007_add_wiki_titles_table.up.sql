-- Create wiki_titles table for storing all wiki page titles (canonical + aliases)
CREATE TABLE wiki_titles (
    id TEXT PRIMARY KEY,
    guild_id TEXT NOT NULL,
    display_title TEXT NOT NULL,            -- Original formatting for display
    page_slug TEXT NOT NULL,                -- URL-friendly slug (e.g., "wiki-page-name")
    page_id TEXT NOT NULL,                  -- References wiki_pages(id)
    is_canonical BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by_user_id TEXT,
    created_by_merge BOOLEAN DEFAULT FALSE,
    
    -- Each slug must be unique per guild
    CONSTRAINT uq_wiki_titles_guild_slug UNIQUE (guild_id, page_slug),
    
    -- Title must point to existing wiki page (CASCADE delete when page deleted)
    CONSTRAINT fk_wiki_titles_page 
        FOREIGN KEY (page_id) 
        REFERENCES wiki_pages(id) 
        ON DELETE CASCADE
);

-- Create indexes for efficient lookups
CREATE INDEX idx_wiki_titles_page_id ON wiki_titles(page_id);
CREATE INDEX idx_wiki_titles_guild ON wiki_titles(guild_id);
CREATE INDEX idx_wiki_titles_lookup ON wiki_titles(guild_id, page_slug);

-- Each page has exactly ONE canonical title (partial unique index with WHERE clause)
CREATE UNIQUE INDEX idx_wiki_titles_canonical ON wiki_titles(guild_id, page_id) WHERE is_canonical = TRUE;

-- Add comments
COMMENT ON TABLE wiki_titles IS 
  'Stores all titles (canonical and aliases) for wiki pages. Each page has one canonical title and optional alias titles. Preserves original formatting in display_title.';
COMMENT ON COLUMN wiki_titles.display_title IS 
  'Original title formatting preserved for display (e.g., "BitR1ot" shows as "BitR1ot" while page_slug is "bitr1ot")';
COMMENT ON COLUMN wiki_titles.page_slug IS 
  'URL-friendly slug generated from display_title using github.com/gosimple/slug. Used for lookups and URLs (e.g., "wiki-page-name")';
COMMENT ON COLUMN wiki_titles.page_id IS 
  'The ID of the wiki page this title refers to';
COMMENT ON COLUMN wiki_titles.is_canonical IS 
  'True for the canonical (primary) title, false for aliases. Each page has exactly one canonical title.';
COMMENT ON COLUMN wiki_titles.created_by_merge IS 
  'True if title was created as alias by merge operation, false if manually created';

-- Migrate existing wiki_pages.title to wiki_titles table
-- Creates canonical title for each existing page
INSERT INTO wiki_titles (id, guild_id, display_title, page_slug, page_id, is_canonical, created_at)
SELECT 
    'title_' || id,
    guild_id,
    title,               -- Use existing as display (already lowercase, but preserved)
    -- Generate slug: remove special chars, convert spaces to hyphens, lowercase
    LOWER(REGEXP_REPLACE(REGEXP_REPLACE(title, '[^a-zA-Z0-9\s-]', '', 'g'), '\s+', '-', 'g')),
    id,
    TRUE,                -- All existing titles are canonical
    created_at
FROM wiki_pages
WHERE deleted_at IS NULL;

-- Migrate any existing pages with alias_for (if column exists from old plan)
-- This is a safety measure in case the old approach was partially implemented
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'wiki_pages' AND column_name = 'alias_for'
    ) THEN
        -- Migrate old alias_for column to new wiki_titles table as non-canonical titles
        INSERT INTO wiki_titles (id, guild_id, display_title, page_slug, page_id, is_canonical, created_by_merge)
        SELECT 
            'alias_' || wp_source.id,
            wp_source.guild_id,
            wp_source.title,
            -- Generate slug for aliases
            LOWER(REGEXP_REPLACE(REGEXP_REPLACE(wp_source.title, '[^a-zA-Z0-9\s-]', '', 'g'), '\s+', '-', 'g')),
            wp_target.id,
            FALSE,  -- These are aliases
            TRUE
        FROM wiki_pages wp_source
        JOIN wiki_pages wp_target 
            ON wp_target.guild_id = wp_source.guild_id 
            AND LOWER(wp_target.title) = LOWER(wp_source.alias_for)
            AND wp_target.deleted_at IS NULL
        WHERE wp_source.alias_for IS NOT NULL;
        
        -- Drop the old column
        ALTER TABLE wiki_pages DROP COLUMN alias_for;
    END IF;
END $$;
