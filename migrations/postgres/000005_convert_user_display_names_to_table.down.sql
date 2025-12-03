-- Revert user_display_names table back to a VIEW

-- Drop the table
DROP TABLE IF EXISTS user_display_names;

-- Recreate the original view
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
