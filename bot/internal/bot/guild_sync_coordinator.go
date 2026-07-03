package bot

import (
	"context"
	"time"

	discordpb "github.com/devilmonastery/hivemind/api/generated/go/discordpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GuildSyncCoordinator manages distributed guild member sync coordination via gRPC
type GuildSyncCoordinator struct {
	instanceID    string
	discordClient discordpb.DiscordServiceClient
}

// NewGuildSyncCoordinator creates a new gRPC-based guild sync coordinator
func NewGuildSyncCoordinator(instanceID string, discordClient discordpb.DiscordServiceClient) *GuildSyncCoordinator {
	return &GuildSyncCoordinator{
		instanceID:    instanceID,
		discordClient: discordClient,
	}
}

// TryAcquireLease attempts to acquire a sync lease for a guild
// Returns: acquired (bool), currentHolder (string), error
func (c *GuildSyncCoordinator) TryAcquireLease(ctx context.Context, guildID string, leaseDuration time.Duration) (bool, string, error) {
	resp, err := c.discordClient.TryAcquireGuildSyncLease(ctx, &discordpb.TryAcquireGuildSyncLeaseRequest{
		GuildId:              guildID,
		InstanceId:           c.instanceID,
		LeaseDurationSeconds: int32(leaseDuration.Seconds()),
	})
	if err != nil {
		return false, "", err
	}

	return resp.Acquired, resp.CurrentHolder, nil
}

// ReleaseLease releases a sync lease for a guild
func (c *GuildSyncCoordinator) ReleaseLease(ctx context.Context, guildID string, success bool, memberCount int, syncStartedAt time.Time) error {
	_, err := c.discordClient.ReleaseGuildSyncLease(ctx, &discordpb.ReleaseGuildSyncLeaseRequest{
		GuildId:       guildID,
		InstanceId:    c.instanceID,
		Success:       success,
		MemberCount:   int32(memberCount),
		SyncStartedAt: timestamppb.New(syncStartedAt),
	})
	return err
}

// NeedsSyncSince checks if a guild needs syncing based on interval
// Returns: needsSync (bool), lastSync (*time.Time), error
func (c *GuildSyncCoordinator) NeedsSyncSince(ctx context.Context, guildID string, interval time.Duration) (bool, *time.Time, error) {
	resp, err := c.discordClient.CheckGuildNeedsSync(ctx, &discordpb.CheckGuildNeedsSyncRequest{
		GuildId:         guildID,
		IntervalSeconds: int32(interval.Seconds()),
	})
	if err != nil {
		return false, nil, err
	}

	var lastSync *time.Time
	if resp.LastSync != nil {
		t := resp.LastSync.AsTime()
		lastSync = &t
	}

	return resp.NeedsSync, lastSync, nil
}

// GetLeaseHolder returns the current lease holder for a guild (for logging/debugging)
// This is derived from the TryAcquireLease response when acquisition fails
func (c *GuildSyncCoordinator) GetLeaseHolder(ctx context.Context, guildID string) (string, error) {
	// Try to acquire with 0 duration - will fail if someone else has it
	_, holder, err := c.TryAcquireLease(ctx, guildID, 0)
	return holder, err
}
