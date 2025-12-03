The preferred name resolution order should be:

1. guild_nick
2. discord_global_name
3. discord_username

This is implemented as a denormalized table `user_display_names` that is automatically maintained by the bot's guild member sync process.

## Implementation

The `user_display_names` table stores pre-computed display names for all guild members:

```sql
CREATE TABLE user_display_names (
    discord_id TEXT NOT NULL,
    guild_id TEXT,
    display_name TEXT NOT NULL,
    guild_nick TEXT,
    discord_global_name TEXT,
    discord_username TEXT NOT NULL,
    PRIMARY KEY (discord_id, guild_id)
);
```

## Maintenance

The table is automatically updated by the bot in two ways:

1. **Real-time updates**: When Discord sends member update events (nickname changes, username changes, etc.), the bot immediately updates both `guild_members` and refreshes the corresponding `user_display_names` entry
2. **Periodic sync**: Every 24 hours, the bot performs a full sync of all guild members to catch any missed updates

Both update mechanisms are incremental - only the affected guild's display names are updated (not all rows).

The display name computation uses SQL-based logic to ensure consistency:

```sql
COALESCE(gm.guild_nick, du.discord_global_name, du.discord_username) AS display_name
```

## Benefits

- **Real-time freshness**: Display names update immediately when users change nicknames or usernames
- **Performance**: Eliminates real-time joins and COALESCE computation during queries
- **Incremental**: Only updates affected guilds/members, not the entire table
- **Automatic cleanup**: When members leave a guild, their display name entries are automatically deleted via foreign key CASCADE
- **Simple queries**: Single JOIN to user_display_names instead of multiple table lookups
