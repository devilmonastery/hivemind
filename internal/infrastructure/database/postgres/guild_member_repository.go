package postgres

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
	"github.com/devilmonastery/hivemind/internal/pkg/metrics"
)

// GuildMemberRepository implements repositories.GuildMemberRepository for PostgreSQL
type GuildMemberRepository struct {
	db  *sqlx.DB
	log *slog.Logger
}

// NewGuildMemberRepository creates a new guild member repository
func NewGuildMemberRepository(db *sqlx.DB) repositories.GuildMemberRepository {
	return &GuildMemberRepository{
		db:  db,
		log: slog.Default().With(slog.String("repo", "guild_member")),
	}
}

// Upsert creates or updates a guild member record
func (r *GuildMemberRepository) Upsert(ctx context.Context, member *entities.GuildMember) error {
	start := time.Now()
	var err error
	defer func() {
		metrics.RecordDBOperation("guild_member", "upsert", time.Since(start), 1, err)
	}()

	r.log.Debug("upserting guild member",
		slog.String("guild_id", member.GuildID),
		slog.String("discord_id", member.DiscordID))

	query := `
		INSERT INTO guild_members (
			guild_id, discord_id, guild_nick, guild_avatar_hash,
			roles, joined_at, synced_at, last_seen
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (guild_id, discord_id) DO UPDATE SET
			guild_nick = EXCLUDED.guild_nick,
			guild_avatar_hash = EXCLUDED.guild_avatar_hash,
			roles = EXCLUDED.roles,
			synced_at = EXCLUDED.synced_at,
			last_seen = COALESCE(EXCLUDED.last_seen, guild_members.last_seen)
	`

	_, err = r.db.ExecContext(ctx, query,
		member.GuildID,
		member.DiscordID,
		member.GuildNick,
		member.GuildAvatarHash,
		pq.Array(member.Roles),
		member.JoinedAt,
		member.SyncedAt,
		member.LastSeen,
	)
	return err
}

// UpsertBatch efficiently inserts/updates multiple members in a transaction
func (r *GuildMemberRepository) UpsertBatch(ctx context.Context, members []*entities.GuildMember) error {
	start := time.Now()
	var err error
	var rowsAffected int64
	defer func() {
		metrics.RecordDBOperation("guild_member", "upsert_batch", time.Since(start), rowsAffected, err)
	}()

	if len(members) == 0 {
		return nil
	}

	r.log.Debug("upserting batch of guild members",
		slog.Int("count", len(members)),
		slog.String("guild_id", members[0].GuildID))

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		INSERT INTO guild_members (
			guild_id, discord_id, guild_nick, guild_avatar_hash,
			roles, joined_at, synced_at, last_seen
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (guild_id, discord_id) DO UPDATE SET
			guild_nick = EXCLUDED.guild_nick,
			guild_avatar_hash = EXCLUDED.guild_avatar_hash,
			roles = EXCLUDED.roles,
			synced_at = EXCLUDED.synced_at,
			last_seen = COALESCE(EXCLUDED.last_seen, guild_members.last_seen)
	`

	stmt, err := tx.PreparexContext(ctx, query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, member := range members {
		_, execErr := stmt.ExecContext(ctx,
			member.GuildID,
			member.DiscordID,
			member.GuildNick,
			member.GuildAvatarHash,
			pq.Array(member.Roles),
			member.JoinedAt,
			member.SyncedAt,
			member.LastSeen,
		)
		if execErr != nil {
			err = execErr
			return err
		}
	}

	err = tx.Commit()
	if err == nil {
		rowsAffected = int64(len(members))
	}
	return err
}

// IsMember checks if a Discord user is a member of a guild
func (r *GuildMemberRepository) IsMember(ctx context.Context, guildID, discordID string) (bool, error) {
	r.log.Debug("checking guild membership",
		slog.String("guild_id", guildID),
		slog.String("discord_id", discordID))

	query := `
		SELECT EXISTS(
			SELECT 1 FROM guild_members
			WHERE guild_id = $1 AND discord_id = $2
		)
	`

	var exists bool
	err := r.db.GetContext(ctx, &exists, query, guildID, discordID)
	return exists, err
}

// GetMember retrieves a guild member record
func (r *GuildMemberRepository) GetMember(ctx context.Context, guildID, discordID string) (*entities.GuildMember, error) {
	start := time.Now()
	var err error
	var rowCount int64
	defer func() {
		metrics.RecordDBOperation("guild_member", "get_member", time.Since(start), rowCount, err)
	}()

	r.log.Debug("getting guild member",
		slog.String("guild_id", guildID),
		slog.String("discord_id", discordID))

	query := `
		SELECT guild_id, discord_id, guild_nick, guild_avatar_hash,
		       roles, joined_at, synced_at, last_seen
		FROM guild_members
		WHERE guild_id = $1 AND discord_id = $2
	`

	var member entities.GuildMember
	err = r.db.GetContext(ctx, &member, query, guildID, discordID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repositories.ErrGuildMemberNotFound
		}
		return nil, err
	}

	rowCount = 1
	return &member, nil
}

// ListGuildMembers retrieves all members for a guild
func (r *GuildMemberRepository) ListGuildMembers(ctx context.Context, guildID string) ([]*entities.GuildMember, error) {
	start := time.Now()
	var err error
	var rowCount int64
	defer func() {
		metrics.RecordDBOperation("guild_member", "list_guild_members", time.Since(start), rowCount, err)
	}()

	query := `
		SELECT guild_id, discord_id, guild_nick, guild_avatar_hash,
		       roles, joined_at, synced_at, last_seen
		FROM guild_members
		WHERE guild_id = $1
		ORDER BY joined_at ASC
	`

	var members []*entities.GuildMember
	err = r.db.SelectContext(ctx, &members, query, guildID)
	if err != nil {
		return nil, err
	}

	rowCount = int64(len(members))
	return members, nil
}

// ListUserGuilds retrieves all guild IDs a user is a member of
func (r *GuildMemberRepository) ListUserGuilds(ctx context.Context, discordID string) ([]string, error) {
	query := `
		SELECT guild_id
		FROM guild_members
		WHERE discord_id = $1
		ORDER BY joined_at DESC
	`

	var guildIDs []string
	err := r.db.SelectContext(ctx, &guildIDs, query, discordID)
	if err != nil {
		return nil, err
	}

	return guildIDs, nil
}

// UpdateLastSeen updates the last_seen timestamp for a guild member
func (r *GuildMemberRepository) UpdateLastSeen(ctx context.Context, guildID, discordID string) error {
	query := `
		UPDATE guild_members
		SET last_seen = $3
		WHERE guild_id = $1 AND discord_id = $2
	`

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query, guildID, discordID, now)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return repositories.ErrGuildMemberNotFound
	}

	return nil
}

// DeleteMember removes a member record (when they leave)
func (r *GuildMemberRepository) DeleteMember(ctx context.Context, guildID, discordID string) error {
	start := time.Now()
	var err error
	var rowsAffected int64
	defer func() {
		metrics.RecordDBOperation("guild_member", "delete_member", time.Since(start), rowsAffected, err)
	}()

	r.log.Debug("deleting guild member",
		slog.String("guild_id", guildID),
		slog.String("discord_id", discordID))

	query := `
		DELETE FROM guild_members
		WHERE guild_id = $1 AND discord_id = $2
	`

	result, err := r.db.ExecContext(ctx, query, guildID, discordID)
	if err != nil {
		return err
	}

	rowsAffected, err = result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		err = repositories.ErrGuildMemberNotFound
		return err
	}

	return nil
}

// DeleteAllGuildMembers removes all members for a guild (when bot leaves guild)
func (r *GuildMemberRepository) DeleteAllGuildMembers(ctx context.Context, guildID string) error {
	r.log.Debug("deleting all guild members", slog.String("guild_id", guildID))

	query := `
		DELETE FROM guild_members
		WHERE guild_id = $1
	`

	_, err := r.db.ExecContext(ctx, query, guildID)
	return err
}

// CountGuildMembers returns the number of members in a guild
func (r *GuildMemberRepository) CountGuildMembers(ctx context.Context, guildID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM guild_members
		WHERE guild_id = $1
	`

	var count int
	err := r.db.GetContext(ctx, &count, query, guildID)
	return count, err
}

// RefreshDisplayNames updates the user_display_names table for a guild
// This keeps the denormalized display names in sync after member updates
func (r *GuildMemberRepository) RefreshDisplayNames(ctx context.Context, guildID string) error {
	start := time.Now()
	var err error
	var rowsAffected int64
	defer func() {
		metrics.RecordDBOperation("guild_member", "refresh_display_names", time.Since(start), rowsAffected, err)
	}()

	r.log.Debug("refreshing display names for guild", slog.String("guild_id", guildID))

	// Upsert all display names for this guild using SQL-based COALESCE logic
	// This ensures single source of truth while allowing incremental updates
	query := `
		INSERT INTO user_display_names (
			discord_id, guild_id, display_name,
			guild_nick, discord_global_name, discord_username,
			guild_avatar_hash, user_avatar_hash
		)
		SELECT 
			gm.discord_id,
			gm.guild_id,
			COALESCE(gm.guild_nick, du.discord_global_name, du.discord_username) AS display_name,
			gm.guild_nick,
			du.discord_global_name,
			du.discord_username,
			gm.guild_avatar_hash,
			du.avatar_hash
		FROM guild_members gm
		JOIN discord_users du ON gm.discord_id = du.discord_id
		WHERE gm.guild_id = $1
		ON CONFLICT (discord_id, guild_id) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			guild_nick = EXCLUDED.guild_nick,
			discord_global_name = EXCLUDED.discord_global_name,
			discord_username = EXCLUDED.discord_username,
			guild_avatar_hash = EXCLUDED.guild_avatar_hash,
			user_avatar_hash = EXCLUDED.user_avatar_hash
	`

	result, err := r.db.ExecContext(ctx, query, guildID)
	if err != nil {
		return err
	}

	rowsAffected, err = result.RowsAffected()
	if err != nil {
		return err
	}

	r.log.Debug("refreshed display names",
		slog.String("guild_id", guildID),
		slog.Int64("rows_affected", rowsAffected))

	return nil
}
