package postgres

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
)

// DiscordGuildRepository implements repositories.DiscordGuildRepository for PostgreSQL
type DiscordGuildRepository struct {
	db  *sqlx.DB
	log *slog.Logger
}

// NewDiscordGuildRepository creates a new Discord guild repository
func NewDiscordGuildRepository(db *sqlx.DB) repositories.DiscordGuildRepository {
	return &DiscordGuildRepository{
		db:  db,
		log: slog.Default().With(slog.String("repo", "discord_guild")),
	}
}

// Create creates a new Discord guild record
func (r *DiscordGuildRepository) Create(ctx context.Context, guild *entities.DiscordGuild) error {
	r.log.Debug("creating discord guild",
		slog.String("guild_id", guild.GuildID),
		slog.String("guild_name", guild.GuildName))

	query := `
		INSERT INTO discord_guilds (
			guild_id, guild_name, icon_url, owner_discord_id,
			enabled, settings, added_at, last_activity
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.ExecContext(ctx, query,
		guild.GuildID,
		guild.GuildName,
		guild.IconURL,
		guild.OwnerID,
		guild.Enabled,
		guild.Settings,
		guild.AddedAt,
		guild.LastActivity,
	)
	if err != nil {
		return err
	}

	return nil
}

// GetByID retrieves a Discord guild by its ID
func (r *DiscordGuildRepository) GetByID(ctx context.Context, guildID string) (*entities.DiscordGuild, error) {
	query := `
		SELECT guild_id, guild_name, icon_url, owner_discord_id,
		       enabled, settings, added_at, last_activity
		FROM discord_guilds
		WHERE guild_id = $1
	`

	var guild entities.DiscordGuild
	err := r.db.GetContext(ctx, &guild, query, guildID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repositories.ErrDiscordGuildNotFound
		}
		return nil, err
	}

	return &guild, nil
}

// Update updates an existing Discord guild record
func (r *DiscordGuildRepository) Update(ctx context.Context, guild *entities.DiscordGuild) error {
	r.log.Debug("updating discord guild",
		slog.String("guild_id", guild.GuildID),
		slog.String("guild_name", guild.GuildName))

	query := `
		UPDATE discord_guilds
		SET guild_name = $2,
		    icon_url = $3,
		    owner_discord_id = $4,
		    enabled = $5,
		    settings = $6,
		    last_activity = $7
		WHERE guild_id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		guild.GuildID,
		guild.GuildName,
		guild.IconURL,
		guild.OwnerID,
		guild.Enabled,
		guild.Settings,
		guild.LastActivity,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return repositories.ErrDiscordGuildNotFound
	}

	return nil
}

// UpdateLastActivity updates the last activity timestamp for a guild
func (r *DiscordGuildRepository) UpdateLastActivity(ctx context.Context, guildID string) error {
	query := `
		UPDATE discord_guilds
		SET last_activity = $2
		WHERE guild_id = $1
	`

	result, err := r.db.ExecContext(ctx, query, guildID, time.Now())
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return repositories.ErrDiscordGuildNotFound
	}

	return nil
}

// UpdateMemberSyncTime updates the last_member_sync timestamp for a guild
func (r *DiscordGuildRepository) UpdateMemberSyncTime(ctx context.Context, guildID string) error {
	query := `
		UPDATE discord_guilds
		SET last_member_sync = $2
		WHERE guild_id = $1
	`

	result, err := r.db.ExecContext(ctx, query, guildID, time.Now())
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return repositories.ErrDiscordGuildNotFound
	}

	return nil
}

// List retrieves all Discord guilds, optionally filtering by enabled status
func (r *DiscordGuildRepository) List(ctx context.Context, enabledOnly bool) ([]*entities.DiscordGuild, error) {
	query := `
		SELECT guild_id, guild_name, icon_url, owner_discord_id,
		       enabled, settings, added_at, last_activity
		FROM discord_guilds
	`

	if enabledOnly {
		query += " WHERE enabled = true"
	}

	query += " ORDER BY guild_name"

	var guilds []*entities.DiscordGuild
	err := r.db.SelectContext(ctx, &guilds, query)
	if err != nil {
		return nil, err
	}

	return guilds, nil
}

// Delete removes a Discord guild record
func (r *DiscordGuildRepository) Delete(ctx context.Context, guildID string) error {
	query := `DELETE FROM discord_guilds WHERE guild_id = $1`

	result, err := r.db.ExecContext(ctx, query, guildID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return repositories.ErrDiscordGuildNotFound
	}

	return nil
}
