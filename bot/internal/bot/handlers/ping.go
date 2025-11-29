package handlers

import (
	"context"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	"google.golang.org/protobuf/types/known/emptypb"

	adminpb "github.com/devilmonastery/hivemind/api/generated/go/adminpb"
	"github.com/devilmonastery/hivemind/bot/internal/grpc"
	"github.com/devilmonastery/hivemind/internal/client"
)

// handlePing responds to the /ping command and tests backend connectivity
func handlePing(s *discordgo.Session, i *discordgo.InteractionCreate, log *slog.Logger, grpcClient *client.Client) {
	// Get username from interaction
	var username string
	var discordUserID string
	if i.Member != nil && i.Member.User != nil {
		username = i.Member.User.Username
		discordUserID = i.Member.User.ID
	} else if i.User != nil {
		username = i.User.Username
		discordUserID = i.User.ID
	} else {
		username = "unknown"
		discordUserID = "unknown"
	}

	// Test backend connectivity with Discord user context
	ctx := context.Background()
	ctx = grpc.WithDiscordContext(ctx, discordUserID, i.GuildID, username)

	// Make a gRPC call to trigger user provisioning
	adminClient := adminpb.NewAdminServiceClient(grpcClient.Conn())
	sysInfo, err := adminClient.GetSystemInfo(ctx, &emptypb.Empty{})
	if err != nil {
		log.Error("failed to get system info from backend",
			slog.String("error", err.Error()),
			slog.String("discord_user_id", discordUserID))

		respErr := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "🏓 Pong! Hello user " + username + " (⚠️  backend error: " + err.Error() + ")",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if respErr != nil {
			log.Error("failed to respond", slog.String("error", respErr.Error()))
		}
		return
	}

	log.Info("backend call successful",
		slog.String("discord_user_id", discordUserID),
		slog.String("username", username),
		slog.String("guild_id", i.GuildID),
		slog.Int("total_users", int(sysInfo.TotalUsers)))

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "🏓 Pong! Hello user " + username + " (backend connected)",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Error("failed to respond to ping",
			slog.String("error", err.Error()),
			slog.String("user_id", discordUserID),
		)
	}
}
