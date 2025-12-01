package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
)

// DiscordUserRepository implements repositories.DiscordUserRepository for PostgreSQL
type DiscordUserRepository struct {
	db *sqlx.DB
}

// NewDiscordUserRepository creates a new Discord user repository
func NewDiscordUserRepository(db *sqlx.DB) repositories.DiscordUserRepository {
	return &DiscordUserRepository{db: db}
}

// Create creates a new Discord user record
func (r *DiscordUserRepository) Create(ctx context.Context, discordUser *entities.DiscordUser) error {
	query := `
		INSERT INTO discord_users (
			discord_id, user_id, discord_username, discord_global_name,
			avatar_url, linked_at, last_seen
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.ExecContext(ctx, query,
		discordUser.DiscordID,
		discordUser.UserID,
		discordUser.DiscordUsername,
		discordUser.DiscordGlobalName,
		discordUser.AvatarURL,
		discordUser.LinkedAt,
		discordUser.LastSeen,
	)
	if err != nil {
		return err
	}

	return nil
}

// Upsert creates or updates a Discord user record
func (r *DiscordUserRepository) Upsert(ctx context.Context, discordUser *entities.DiscordUser) error {
	query := `
		INSERT INTO discord_users (
			discord_id, user_id, discord_username, discord_global_name,
			avatar_url, linked_at, last_seen
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (discord_id) DO UPDATE SET
			user_id = COALESCE(EXCLUDED.user_id, discord_users.user_id),
			discord_username = EXCLUDED.discord_username,
			discord_global_name = EXCLUDED.discord_global_name,
			avatar_url = EXCLUDED.avatar_url,
			last_seen = EXCLUDED.last_seen
	`

	_, err := r.db.ExecContext(ctx, query,
		discordUser.DiscordID,
		discordUser.UserID,
		discordUser.DiscordUsername,
		discordUser.DiscordGlobalName,
		discordUser.AvatarURL,
		discordUser.LinkedAt,
		discordUser.LastSeen,
	)
	if err != nil {
		return err
	}

	return nil
}

// GetByDiscordID retrieves a Discord user by their Discord ID
func (r *DiscordUserRepository) GetByDiscordID(ctx context.Context, discordID string) (*entities.DiscordUser, error) {
	query := `
		SELECT discord_id, user_id, discord_username, discord_global_name,
		       avatar_url, linked_at, last_seen
		FROM discord_users
		WHERE discord_id = $1
	`

	var user entities.DiscordUser
	err := r.db.GetContext(ctx, &user, query, discordID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repositories.ErrDiscordUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

// GetByUserID retrieves a Discord user by their Hivemind user ID
func (r *DiscordUserRepository) GetByUserID(ctx context.Context, userID string) (*entities.DiscordUser, error) {
	query := `
		SELECT discord_id, user_id, discord_username, discord_global_name,
		       avatar_url, linked_at, last_seen
		FROM discord_users
		WHERE user_id = $1
	`

	var user entities.DiscordUser
	err := r.db.GetContext(ctx, &user, query, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repositories.ErrDiscordUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

// Update updates a Discord user record
func (r *DiscordUserRepository) Update(ctx context.Context, discordUser *entities.DiscordUser) error {
	query := `
		UPDATE discord_users
		SET user_id = $2,
		    discord_username = $3,
		    discord_global_name = $4,
		    avatar_url = $5,
		    last_seen = $6
		WHERE discord_id = $1
	`

	fmt.Printf("[DiscordUserRepository.Update] discord_id=%s user_id=%v\n",
		discordUser.DiscordID, discordUser.UserID)

	result, err := r.db.ExecContext(ctx, query,
		discordUser.DiscordID,
		discordUser.UserID,
		discordUser.DiscordUsername,
		discordUser.DiscordGlobalName,
		discordUser.AvatarURL,
		discordUser.LastSeen,
	)
	if err != nil {
		fmt.Printf("[DiscordUserRepository.Update] ERROR: %v\n", err)
		return err
	}

	rows, _ := result.RowsAffected()
	fmt.Printf("[DiscordUserRepository.Update] rows affected: %d\n", rows)

	return nil
}

// UpdateLastSeen updates the last_seen timestamp for a Discord user
func (r *DiscordUserRepository) UpdateLastSeen(ctx context.Context, discordID string) error {
	query := `
		UPDATE discord_users
		SET last_seen = $2
		WHERE discord_id = $1
	`

	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, discordID, now)
	return err
}

// Delete removes a Discord user record (unlinking)
func (r *DiscordUserRepository) Delete(ctx context.Context, discordID string) error {
	query := `DELETE FROM discord_users WHERE discord_id = $1`
	_, err := r.db.ExecContext(ctx, query, discordID)
	return err
}
