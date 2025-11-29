package handlers

import (
	"context"

	"github.com/bwmarrin/discordgo"
	botgrpc "github.com/devilmonastery/hivemind/bot/internal/grpc"
)

// discordContextFor creates a gRPC context with Discord user identity from an interaction.
// It extracts the Discord user ID, guild ID, and preferred username (nick if set, otherwise username)
// and embeds them as metadata for the backend to identify the user making the request.
func discordContextFor(i *discordgo.InteractionCreate) context.Context {
	username := i.Member.User.Username
	if i.Member.Nick != "" {
		username = i.Member.Nick
	}
	return botgrpc.WithDiscordContext(
		context.Background(),
		i.Member.User.ID,
		i.GuildID,
		username,
	)
}
