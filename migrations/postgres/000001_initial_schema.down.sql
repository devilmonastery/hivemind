-- Drop all tables in reverse dependency order
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS note_message_references;
DROP TABLE IF EXISTS wiki_message_references;
DROP TABLE IF EXISTS wiki_titles;
DROP TABLE IF EXISTS quotes;
DROP TABLE IF EXISTS notes;
DROP TABLE IF EXISTS wiki_pages;
DROP TABLE IF EXISTS oidc_sessions;
DROP TABLE IF EXISTS api_tokens;
DROP TABLE IF EXISTS guild_members;
DROP TABLE IF EXISTS discord_guilds;
DROP TABLE IF EXISTS discord_users;
DROP TABLE IF EXISTS users;

-- Drop extension
DROP EXTENSION IF EXISTS vector;
