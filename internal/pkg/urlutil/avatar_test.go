package urlutil

import (
	"testing"
)

func TestConstructAvatarURL(t *testing.T) {
	tests := []struct {
		name            string
		discordID       string
		guildID         string
		guildAvatarHash string
		userAvatarHash  string
		size            int
		expected        string
	}{
		{
			name:            "guild avatar takes priority",
			discordID:       "123456789012345678",
			guildID:         "987654321098765432",
			guildAvatarHash: "guild_hash_123",
			userAvatarHash:  "user_hash_456",
			size:            128,
			expected:        "https://cdn.discordapp.com/guilds/987654321098765432/users/123456789012345678/avatars/guild_hash_123.png?size=128",
		},
		{
			name:            "user avatar fallback when no guild avatar",
			discordID:       "123456789012345678",
			guildID:         "987654321098765432",
			guildAvatarHash: "",
			userAvatarHash:  "user_hash_456",
			size:            64,
			expected:        "https://cdn.discordapp.com/avatars/123456789012345678/user_hash_456.png?size=64",
		},
		{
			name:            "user avatar when no guild ID",
			discordID:       "123456789012345678",
			guildID:         "",
			guildAvatarHash: "",
			userAvatarHash:  "user_hash_456",
			size:            256,
			expected:        "https://cdn.discordapp.com/avatars/123456789012345678/user_hash_456.png?size=256",
		},
		{
			name:            "default avatar when no hashes",
			discordID:       "123456789012345678",
			guildID:         "987654321098765432",
			guildAvatarHash: "",
			userAvatarHash:  "",
			size:            128,
			expected:        "https://cdn.discordapp.com/embed/avatars/2.png", // (123456789012345678 >> 22) % 6 = 2
		},
		{
			name:            "default avatar calculation - ID results in index 0",
			discordID:       "4194304", // (4194304 >> 22) % 6 = 0
			guildID:         "",
			guildAvatarHash: "",
			userAvatarHash:  "",
			size:            128,
			expected:        "https://cdn.discordapp.com/embed/avatars/0.png",
		},
		{
			name:            "default avatar calculation - ID results in index 5",
			discordID:       "25165824", // (25165824 >> 22) % 6 = 5
			guildID:         "",
			guildAvatarHash: "",
			userAvatarHash:  "",
			size:            128,
			expected:        "https://cdn.discordapp.com/embed/avatars/5.png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConstructAvatarURL(tt.discordID, tt.guildID, tt.guildAvatarHash, tt.userAvatarHash, tt.size)
			if result != tt.expected {
				t.Errorf("ConstructAvatarURL() = %v, want %v", result, tt.expected)
			}
		})
	}
}
