package bot

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

// GuildSyncCoordinator manages distributed guild member sync coordination via database leases
type GuildSyncCoordinator struct {
	db         *sql.DB
	instanceID string
	logger     *slog.Logger
}

// SyncResult captures the outcome of a guild sync operation
type SyncResult struct {
	GuildID       string
	MemberCount   int
	Duration      time.Duration
	Error         error
	SyncStartTime time.Time
}

// NewGuildSyncCoordinator creates a coordinator with a unique instance ID
func NewGuildSyncCoordinator(db *sql.DB, instanceID string, logger *slog.Logger) *GuildSyncCoordinator {
	return &GuildSyncCoordinator{
		db:         db,
		instanceID: instanceID,
		logger:     logger,
	}
}

// TryAcquireLease attempts to acquire the sync lease for a guild.
// Returns true if lease acquired, false if another instance holds it.
func (c *GuildSyncCoordinator) TryAcquireLease(ctx context.Context, guildID string, leaseDuration time.Duration) (bool, error) {
	query := `
		UPDATE discord_guilds
		SET 
			sync_lease_holder = $1,
			sync_lease_expires_at = NOW() + $2::INTERVAL
		WHERE guild_id = $3
		  AND (
			sync_lease_holder IS NULL 
			OR sync_lease_expires_at < NOW() 
			OR sync_lease_holder = $1
		  )
		RETURNING guild_id
	`

	var returnedGuildID string
	err := c.db.QueryRowContext(ctx, query,
		c.instanceID,
		fmt.Sprintf("%d seconds", int(leaseDuration.Seconds())),
		guildID,
	).Scan(&returnedGuildID)

	if err == sql.ErrNoRows {
		// Someone else holds the lease
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to acquire lease: %w", err)
	}

	c.logger.Debug("acquired guild sync lease",
		slog.String("guild_id", guildID),
		slog.String("instance", c.instanceID),
		slog.Duration("duration", leaseDuration))

	return true, nil
}

// ReleaseLease releases the lease and records sync completion
func (c *GuildSyncCoordinator) ReleaseLease(ctx context.Context, result SyncResult) error {
	if result.Error != nil {
		// Failed sync - release lease but don't update last_member_sync
		query := `
			UPDATE discord_guilds
			SET 
				sync_lease_holder = NULL,
				sync_lease_expires_at = NULL
			WHERE guild_id = $1
			  AND sync_lease_holder = $2
		`
		_, err := c.db.ExecContext(ctx, query, result.GuildID, c.instanceID)
		if err != nil {
			return fmt.Errorf("failed to release lease after error: %w", err)
		}

		c.logger.Warn("released guild sync lease after error",
			slog.String("guild_id", result.GuildID),
			slog.String("error", result.Error.Error()))
		return nil
	}

	// Successful sync - release lease and update last_member_sync
	query := `
		UPDATE discord_guilds
		SET 
			sync_lease_holder = NULL,
			sync_lease_expires_at = NULL,
			last_member_sync = $1
		WHERE guild_id = $2
		  AND sync_lease_holder = $3
	`

	_, err := c.db.ExecContext(ctx, query,
		result.SyncStartTime,
		result.GuildID,
		c.instanceID,
	)
	if err != nil {
		return fmt.Errorf("failed to release lease: %w", err)
	}

	c.logger.Info("released guild sync lease",
		slog.String("guild_id", result.GuildID),
		slog.Int("members", result.MemberCount),
		slog.Duration("duration", result.Duration))

	return nil
}

// NeedsSyncSince checks if a guild needs syncing based on the last sync time
func (c *GuildSyncCoordinator) NeedsSyncSince(ctx context.Context, guildID string, interval time.Duration) (bool, error) {
	query := `
		SELECT 
			last_member_sync IS NULL 
			OR last_member_sync + $1::INTERVAL < NOW()
		FROM discord_guilds
		WHERE guild_id = $2
	`

	var needsSync bool
	err := c.db.QueryRowContext(ctx, query,
		fmt.Sprintf("%d seconds", int(interval.Seconds())),
		guildID,
	).Scan(&needsSync)

	if err == sql.ErrNoRows {
		// Guild doesn't exist in DB yet - needs sync
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check sync status: %w", err)
	}

	return needsSync, nil
}

// GetLeaseHolder returns the current lease holder for a guild, if any
func (c *GuildSyncCoordinator) GetLeaseHolder(ctx context.Context, guildID string) (*string, error) {
	query := `
		SELECT sync_lease_holder, sync_lease_expires_at
		FROM discord_guilds
		WHERE guild_id = $1
		  AND sync_lease_holder IS NOT NULL
		  AND sync_lease_expires_at > NOW()
	`

	var holder string
	var expiresAt time.Time
	err := c.db.QueryRowContext(ctx, query, guildID).Scan(&holder, &expiresAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get lease holder: %w", err)
	}

	return &holder, nil
}
