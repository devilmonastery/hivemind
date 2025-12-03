package handlers

import (
	"context"
	"time"

	discordpb "github.com/devilmonastery/hivemind/api/generated/go/discordpb"
	"github.com/devilmonastery/hivemind/internal/domain/entities"
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

// UpsertGuildMember creates or updates a guild member record
func (h *DiscordHandler) UpsertGuildMember(ctx context.Context, req *discordpb.UpsertGuildMemberRequest) (*discordpb.UpsertGuildMemberResponse, error) {
	if req.GuildId == "" {
		return nil, status.Error(codes.InvalidArgument, "guild_id is required")
	}
	if req.DiscordId == "" {
		return nil, status.Error(codes.InvalidArgument, "discord_id is required")
	}

	// If user data is provided, update discord_users first
	if req.DiscordUsername != "" {
		discordUser := &entities.DiscordUser{
			DiscordID:       req.DiscordId,
			UserID:          nil,
			DiscordUsername: req.DiscordUsername,
			LinkedAt:        time.Now(),
		}
		if req.DiscordGlobalName != "" {
			discordUser.DiscordGlobalName = &req.DiscordGlobalName
		}
		if req.AvatarUrl != "" {
			discordUser.AvatarURL = &req.AvatarUrl
		}
		now := time.Now()
		discordUser.LastSeen = &now

		if err := h.discordService.UpsertDiscordUser(ctx, discordUser); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to upsert discord user: %v", err)
		}
	}

	member := &entities.GuildMember{
		GuildID:   req.GuildId,
		DiscordID: req.DiscordId,
		JoinedAt:  req.JoinedAt.AsTime(),
		Roles:     req.Roles,
		SyncedAt:  time.Now(),
	}

	if req.GuildNick != "" {
		member.GuildNick = &req.GuildNick
	}
	if req.GuildAvatarHash != "" {
		member.GuildAvatarHash = &req.GuildAvatarHash
	}

	if err := h.discordService.UpsertGuildMember(ctx, member); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to upsert guild member: %v", err)
	}

	return &discordpb.UpsertGuildMemberResponse{
		Success: true,
	}, nil
}

// UpsertGuildMembersBatch efficiently inserts/updates multiple members
func (h *DiscordHandler) UpsertGuildMembersBatch(ctx context.Context, req *discordpb.UpsertGuildMembersBatchRequest) (*discordpb.UpsertGuildMembersBatchResponse, error) {
	if len(req.Members) == 0 {
		return &discordpb.UpsertGuildMembersBatchResponse{Count: 0}, nil
	}

	// First, ensure all Discord users exist
	discordUsers := make([]*entities.DiscordUser, 0, len(req.Members))
	for _, m := range req.Members {
		// Only create discord_users entries if we have username info
		if m.DiscordUsername != "" {
			discordUser := &entities.DiscordUser{
				DiscordID:       m.DiscordId,
				UserID:          nil, // No linked Hivemind user yet during sync
				DiscordUsername: m.DiscordUsername,
				LinkedAt:        time.Now(),
			}
			if m.DiscordGlobalName != "" {
				discordUser.DiscordGlobalName = &m.DiscordGlobalName
			}
			if m.AvatarUrl != "" {
				discordUser.AvatarURL = &m.AvatarUrl
			}
			now := time.Now()
			discordUser.LastSeen = &now

			discordUsers = append(discordUsers, discordUser)
		}
	}

	// Upsert all Discord users first (satisfies foreign key constraint)
	if len(discordUsers) > 0 {
		if err := h.discordService.UpsertDiscordUsersBatch(ctx, discordUsers); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to batch upsert discord users: %v", err)
		}
	}

	// Now upsert guild members
	members := make([]*entities.GuildMember, len(req.Members))
	for i, m := range req.Members {
		member := &entities.GuildMember{
			GuildID:   m.GuildId,
			DiscordID: m.DiscordId,
			JoinedAt:  m.JoinedAt.AsTime(),
			Roles:     m.Roles,
			SyncedAt:  time.Now(),
		}
		if m.GuildNick != "" {
			member.GuildNick = &m.GuildNick
		}
		if m.GuildAvatarHash != "" {
			member.GuildAvatarHash = &m.GuildAvatarHash
		}
		if m.LastSeen != nil {
			t := m.LastSeen.AsTime()
			member.LastSeen = &t
		}
		members[i] = member
	}

	if err := h.discordService.UpsertGuildMembersBatch(ctx, members); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to batch upsert members: %v", err)
	}

	return &discordpb.UpsertGuildMembersBatchResponse{
		Count: int32(len(members)),
	}, nil
}

// RemoveGuildMember removes a member record
func (h *DiscordHandler) RemoveGuildMember(ctx context.Context, req *discordpb.RemoveGuildMemberRequest) (*discordpb.RemoveGuildMemberResponse, error) {
	if req.GuildId == "" {
		return nil, status.Error(codes.InvalidArgument, "guild_id is required")
	}
	if req.DiscordId == "" {
		return nil, status.Error(codes.InvalidArgument, "discord_id is required")
	}

	if err := h.discordService.RemoveGuildMember(ctx, req.GuildId, req.DiscordId); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove guild member: %v", err)
	}

	return &discordpb.RemoveGuildMemberResponse{
		Success: true,
	}, nil
}

// CheckGuildMembership checks if a user is a member of a guild
func (h *DiscordHandler) CheckGuildMembership(ctx context.Context, req *discordpb.CheckGuildMembershipRequest) (*discordpb.CheckGuildMembershipResponse, error) {
	if req.GuildId == "" {
		return nil, status.Error(codes.InvalidArgument, "guild_id is required")
	}
	if req.DiscordId == "" {
		return nil, status.Error(codes.InvalidArgument, "discord_id is required")
	}

	isMember, err := h.discordService.CheckGuildMembership(ctx, req.GuildId, req.DiscordId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check membership: %v", err)
	}

	return &discordpb.CheckGuildMembershipResponse{
		IsMember: isMember,
	}, nil
}

// ListUserGuilds returns all guilds a user is a member of
func (h *DiscordHandler) ListUserGuilds(ctx context.Context, req *discordpb.ListUserGuildsRequest) (*discordpb.ListUserGuildsResponse, error) {
	if req.DiscordId == "" {
		return nil, status.Error(codes.InvalidArgument, "discord_id is required")
	}

	guildIDs, err := h.discordService.ListUserGuilds(ctx, req.DiscordId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list user guilds: %v", err)
	}

	return &discordpb.ListUserGuildsResponse{
		GuildIds: guildIDs,
	}, nil
}
