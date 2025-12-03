-- Create a view that provides consistent username display resolution
-- Priority: guild_nick > discord_global_name > discord_username

CREATE VIEW user_display_names AS
SELECT 
    du.discord_id,
    gm.guild_id,
    COALESCE(gm.guild_nick, du.discord_global_name, du.discord_username) AS display_name,
    gm.guild_nick,
    du.discord_global_name,
    du.discord_username
FROM discord_users du
LEFT JOIN guild_members gm ON du.discord_id = gm.discord_id;

-- Create index on the underlying tables if not already present
-- (guild_members already has index on discord_id, discord_users is indexed by PK)
