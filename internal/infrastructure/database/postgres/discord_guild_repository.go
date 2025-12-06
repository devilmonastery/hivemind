package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
	"github.com/devilmonastery/hivemind/internal/pkg/metrics"
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

// GetByID retrieves a Discord guild by guild ID
func (r *DiscordGuildRepository) GetByID(ctx context.Context, guildID string) (*entities.DiscordGuild, error) {
	start := time.Now()
	var err error
	defer func() {
		metrics.RecordDBOperation("discord_guild", "get_by_id", time.Since(start), -1, err)
	}()

	query := `
		SELECT guild_id, guild_name, icon_url, owner_discord_id,
		       enabled, settings, added_at, last_activity
		FROM discord_guilds
		WHERE guild_id = $1
	`

	var guild entities.DiscordGuild
	err = r.db.GetContext(ctx, &guild, query, guildID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repositories.ErrDiscordGuildNotFound
		}
		return nil, err
	}

	return &guild, nil
}

// Update updates a Discord guild's information
func (r *DiscordGuildRepository) Update(ctx context.Context, guild *entities.DiscordGuild) error {
	start := time.Now()
	var err error
	var rowsAffected int64
	defer func() {
		metrics.RecordDBOperation("discord_guild", "update", time.Since(start), rowsAffected, err)
	}()

	query := `
		UPDATE discord_guilds
		SET guild_name = $2, icon_url = $3, owner_discord_id = $4,
		    enabled = $5, settings = $6, last_activity = $7
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

	rowsAffected, err = result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return repositories.ErrDiscordGuildNotFound
	}

	return nil
}

// UpdateLastActivity updates the last activity timestamp for a guild
func (r *DiscordGuildRepository) UpdateLastActivity(ctx context.Context, guildID string) error {
	start := time.Now()
	var err error
	var rowsAffected int64
	defer func() {
		metrics.RecordDBOperation("discord_guild", "update_last_activity", time.Since(start), rowsAffected, err)
	}()

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

// UpdateMemberSyncTime updates the last member sync timestamp for a guild
func (r *DiscordGuildRepository) UpdateMemberSyncTime(ctx context.Context, guildID string) error {
	start := time.Now()
	var err error
	var rowsAffected int64
	defer func() {
		metrics.RecordDBOperation("discord_guild", "update_member_sync_time", time.Since(start), rowsAffected, err)
	}()

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
	start := time.Now()
	var err error
	var rowCount int64
	defer func() {
		metrics.RecordDBOperation("discord_guild", "list", time.Since(start), rowCount, err)
	}()
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
	err = r.db.SelectContext(ctx, &guilds, query)
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

// UpdateSettings updates the settings JSONB for a guild
func (r *DiscordGuildRepository) UpdateSettings(ctx context.Context, guildID string, settings map[string]interface{}) error {
	start := time.Now()
	var err error
	var rowsAffected int64
	defer func() {
		metrics.RecordDBOperation("discord_guild", "update_settings", time.Since(start), rowsAffected, err)
	}()

	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return err
	}

	query := `
		UPDATE discord_guilds
		SET settings = $1, last_activity = CURRENT_TIMESTAMP
		WHERE guild_id = $2
	`

	result, err := r.db.ExecContext(ctx, query, settingsJSON, guildID)
	if err != nil {
		return err
	}

	rowsAffected, err = result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return repositories.ErrDiscordGuildNotFound
	}

	r.log.Debug("updated guild settings",
		slog.String("guild_id", guildID))

	return nil
}

// GetSettings retrieves the settings JSONB for a guild
func (r *DiscordGuildRepository) GetSettings(ctx context.Context, guildID string) (map[string]interface{}, error) {
	start := time.Now()
	var err error
	defer func() {
		metrics.RecordDBOperation("discord_guild", "get_settings", time.Since(start), -1, err)
	}()

	query := `
		SELECT settings
		FROM discord_guilds
		WHERE guild_id = $1
	`

	var settingsJSON []byte
	err = r.db.QueryRowContext(ctx, query, guildID).Scan(&settingsJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repositories.ErrDiscordGuildNotFound
		}
		return nil, err
	}

	var settings map[string]interface{}
	if len(settingsJSON) > 0 {
		if err := json.Unmarshal(settingsJSON, &settings); err != nil {
			return nil, err
		}
	}

	return settings, nil
}
