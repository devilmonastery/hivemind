The preferred name resolution order should be:

1. guild_nick
2. discord_global_name
3. discord_username

This can be implemented with a database view. For example:

```
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
```
