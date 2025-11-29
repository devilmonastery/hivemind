package grpc

import (
	"context"

	"google.golang.org/grpc/metadata"
)

const (
	// MetadataKeyDiscordUserID is the metadata key for Discord user ID
	MetadataKeyDiscordUserID = "x-discord-user-id"

	// MetadataKeyDiscordGuildID is the metadata key for Discord guild ID
	MetadataKeyDiscordGuildID = "x-discord-guild-id"

	// MetadataKeyDiscordUsername is the metadata key for Discord username (for logging)
	MetadataKeyDiscordUsername = "x-discord-username"
)

// WithDiscordContext adds Discord user context to a gRPC context
// This allows the backend to know which Discord user initiated the request
// Note: This assumes the authorization header is already set by the gRPC client interceptor
func WithDiscordContext(ctx context.Context, discordUserID, guildID, username string) context.Context {
	// Get existing metadata or create new
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}

	// Add Discord context metadata
	md = metadata.Join(md, metadata.New(map[string]string{
		MetadataKeyDiscordUserID:   discordUserID,
		MetadataKeyDiscordGuildID:  guildID,
		MetadataKeyDiscordUsername: username,
	}))

	return metadata.NewOutgoingContext(ctx, md)
}

// WithDiscordUser is a simplified version that only adds user ID
func WithDiscordUser(ctx context.Context, discordUserID string) context.Context {
	return WithDiscordContext(ctx, discordUserID, "", "")
}
