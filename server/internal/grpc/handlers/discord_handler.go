package handlers

import (
	"context"
	"time"

	discordpb "github.com/devilmonastery/hivemind/api/generated/go/discordpb"
	"github.com/devilmonastery/hivemind/internal/domain/services"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// DiscordHandler implements the Discord gRPC service
type DiscordHandler struct {
	discordpb.UnimplementedDiscordServiceServer
	discordService *services.DiscordService
}

// NewDiscordHandler creates a new Discord handler
func NewDiscordHandler(discordService *services.DiscordService) *DiscordHandler {
	return &DiscordHandler{
		discordService: discordService,
	}
}

// UpsertGuild creates or updates a Discord guild
func (h *DiscordHandler) UpsertGuild(ctx context.Context, req *discordpb.UpsertGuildRequest) (*discordpb.UpsertGuildResponse, error) {
	if req.GuildId == "" {
		return nil, status.Error(codes.InvalidArgument, "guild_id is required")
	}
	if req.GuildName == "" {
		return nil, status.Error(codes.InvalidArgument, "guild_name is required")
	}

	guild, err := h.discordService.UpsertGuild(ctx, req.GuildId, req.GuildName, req.IconUrl, req.OwnerDiscordId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to upsert guild: %v", err)
	}

	return &discordpb.UpsertGuildResponse{
		Guild: &discordpb.Guild{
			GuildId:        guild.GuildID,
			GuildName:      guild.GuildName,
			IconUrl:        stringPtrValue(guild.IconURL),
			OwnerDiscordId: stringPtrValue(guild.OwnerID),
			Enabled:        guild.Enabled,
			AddedAt:        timestamppb.New(guild.AddedAt),
			LastActivity:   timestampPtrToProto(guild.LastActivity),
		},
	}, nil
}

// DisableGuild marks a guild as disabled
func (h *DiscordHandler) DisableGuild(ctx context.Context, req *discordpb.DisableGuildRequest) (*discordpb.DisableGuildResponse, error) {
	if req.GuildId == "" {
		return nil, status.Error(codes.InvalidArgument, "guild_id is required")
	}

	if err := h.discordService.DisableGuild(ctx, req.GuildId); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to disable guild: %v", err)
	}

	return &discordpb.DisableGuildResponse{
		Success: true,
	}, nil
}

// GetGuild retrieves guild information
func (h *DiscordHandler) GetGuild(ctx context.Context, req *discordpb.GetGuildRequest) (*discordpb.GetGuildResponse, error) {
	if req.GuildId == "" {
		return nil, status.Error(codes.InvalidArgument, "guild_id is required")
	}

	guild, err := h.discordService.GetGuild(ctx, req.GuildId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "guild not found: %v", err)
	}

	return &discordpb.GetGuildResponse{
		Guild: &discordpb.Guild{
			GuildId:        guild.GuildID,
			GuildName:      guild.GuildName,
			IconUrl:        stringPtrValue(guild.IconURL),
			OwnerDiscordId: stringPtrValue(guild.OwnerID),
			Enabled:        guild.Enabled,
			AddedAt:        timestamppb.New(guild.AddedAt),
			LastActivity:   timestampPtrToProto(guild.LastActivity),
		},
	}, nil
}

func stringPtrValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func timestampPtrToProto(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}
