package urlutil

import (
	"fmt"
	"net/url"
)

// DiscordMessageURL builds a Discord message URL for the given guild, channel, and message IDs.
// Returns a URL like: https://discord.com/channels/{guildID}/{channelID}/{messageID}
func DiscordMessageURL(guildID, channelID, messageID string) string {
	return fmt.Sprintf("https://discord.com/channels/%s/%s/%s", guildID, channelID, messageID)
}

// DiscordCDNIconURL builds a Discord CDN URL for a guild icon.
// Returns a URL like: https://cdn.discordapp.com/icons/{guildID}/{iconHash}.png
func DiscordCDNIconURL(guildID, iconHash string) string {
	return fmt.Sprintf("https://cdn.discordapp.com/icons/%s/%s.png", guildID, iconHash)
}

// DiscordOAuthURL builds a Discord OAuth authorization URL with the specified parameters.
// integrationType should be 0 for guild installation or 1 for user installation.
func DiscordOAuthURL(clientID, permissions string, integrationType int) string {
	u := &url.URL{
		Scheme: "https",
		Host:   "discord.com",
		Path:   "/oauth2/authorize",
	}
	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("permissions", permissions)
	q.Set("integration_type", fmt.Sprintf("%d", integrationType))
	q.Set("scope", "bot applications.commands")
	u.RawQuery = q.Encode()
	return u.String()
}
