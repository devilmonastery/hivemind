-- Drop the wiki_titles table (CASCADE removes foreign keys)
DROP TABLE IF EXISTS wiki_titles CASCADE;

-- Note: We don't restore the old wiki_pages.title column or alias_for column
-- If rollback is needed, title data will be lost (should restore from backup)
