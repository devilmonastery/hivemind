package urlutil

import (
	"fmt"
	"strconv"
)

// ConstructAvatarURL builds a Discord CDN avatar URL with proper fallback logic
// Priority: guild avatar > user avatar > default avatar
//
// Parameters:
//   - discordID: The Discord user ID (snowflake)
//   - guildID: The Discord guild ID (empty string if not guild-specific)
//   - guildAvatarHash: Hash of guild-specific avatar (empty if not set)
//   - userAvatarHash: Hash of global user avatar (empty if not set)
//   - size: Desired image size (64, 128, 256, etc.)
//
// Returns: Full CDN URL to the avatar image
func ConstructAvatarURL(discordID, guildID, guildAvatarHash, userAvatarHash string, size int) string {
	// Priority 1: Guild-specific avatar
	if guildAvatarHash != "" && guildID != "" {
		return fmt.Sprintf("https://cdn.discordapp.com/guilds/%s/users/%s/avatars/%s.png?size=%d",
			guildID, discordID, guildAvatarHash, size)
	}

	// Priority 2: Global user avatar
	if userAvatarHash != "" {
		return fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png?size=%d",
			discordID, userAvatarHash, size)
	}

	// Priority 3: Default Discord avatar
	// Discord calculates default avatar index from user ID
	id, _ := strconv.ParseInt(discordID, 10, 64)
	avatarIndex := (id >> 22) % 6
	return fmt.Sprintf("https://cdn.discordapp.com/embed/avatars/%d.png", avatarIndex)
}
