package urlutil

import (
	"net/url"
	"strings"
	"testing"
)

func TestDiscordMessageURL(t *testing.T) {
	tests := []struct {
		name      string
		guildID   string
		channelID string
		messageID string
		want      string
	}{
		{
			name:      "basic URL",
			guildID:   "123456789",
			channelID: "987654321",
			messageID: "111222333",
			want:      "https://discord.com/channels/123456789/987654321/111222333",
		},
		{
			name:      "with long snowflake IDs",
			guildID:   "1234567890123456789",
			channelID: "9876543210987654321",
			messageID: "1112223334445556667",
			want:      "https://discord.com/channels/1234567890123456789/9876543210987654321/1112223334445556667",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DiscordMessageURL(tt.guildID, tt.channelID, tt.messageID)
			if got != tt.want {
				t.Errorf("DiscordMessageURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiscordCDNIconURL(t *testing.T) {
	tests := []struct {
		name     string
		guildID  string
		iconHash string
		want     string
	}{
		{
			name:     "basic icon URL",
			guildID:  "123456789",
			iconHash: "a1b2c3d4e5f6",
			want:     "https://cdn.discordapp.com/icons/123456789/a1b2c3d4e5f6.png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DiscordCDNIconURL(tt.guildID, tt.iconHash)
			if got != tt.want {
				t.Errorf("DiscordCDNIconURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiscordOAuthURL(t *testing.T) {
	tests := []struct {
		name            string
		clientID        string
		permissions     string
		integrationType int
		wantContains    []string
	}{
		{
			name:            "guild installation",
			clientID:        "123456789",
			permissions:     "277025507392",
			integrationType: 0,
			wantContains: []string{
				"https://discord.com/oauth2/authorize",
				"client_id=123456789",
				"permissions=277025507392",
				"integration_type=0",
				"scope=bot+applications.commands",
			},
		},
		{
			name:            "user installation",
			clientID:        "987654321",
			permissions:     "277025507392",
			integrationType: 1,
			wantContains: []string{
				"https://discord.com/oauth2/authorize",
				"client_id=987654321",
				"permissions=277025507392",
				"integration_type=1",
				"scope=bot+applications.commands",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DiscordOAuthURL(tt.clientID, tt.permissions, tt.integrationType)

			// Verify URL is valid
			if _, err := url.Parse(got); err != nil {
				t.Errorf("DiscordOAuthURL() returned invalid URL: %v", err)
			}

			// Check for required components
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("DiscordOAuthURL() = %v, want to contain %v", got, want)
				}
			}
		})
	}
}

func TestBuildNoteViewURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		noteID  string
		want    string
		wantErr bool
	}{
		{
			name:    "basic note URL",
			baseURL: "http://localhost:8080",
			noteID:  "note123",
			want:    "http://localhost:8080/note?id=note123",
			wantErr: false,
		},
		{
			name:    "with special characters in ID",
			baseURL: "https://example.com",
			noteID:  "note with spaces",
			want:    "https://example.com/note?id=note+with+spaces",
			wantErr: false,
		},
		{
			name:    "invalid base URL",
			baseURL: "://invalid",
			noteID:  "note123",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildNoteViewURL(tt.baseURL, tt.noteID)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildNoteViewURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("BuildNoteViewURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildWikiViewURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		guildID string
		title   string
		want    string
		wantErr bool
	}{
		{
			name:    "basic wiki URL",
			baseURL: "http://localhost:8080",
			guildID: "guild123",
			title:   "My Page",
			want:    "http://localhost:8080/wiki?guild_id=guild123&title=My+Page",
			wantErr: false,
		},
		{
			name:    "with special characters in title",
			baseURL: "https://example.com",
			guildID: "guild456",
			title:   "Page & Title / With Special?",
			want:    "https://example.com/wiki?guild_id=guild456&title=Page+%26+Title+%2F+With+Special%3F",
			wantErr: false,
		},
		{
			name:    "with unicode in title",
			baseURL: "https://example.com",
			guildID: "guild789",
			title:   "日本語タイトル",
			wantErr: false,
			// Note: We don't check exact encoding, just that it doesn't error
		},
		{
			name:    "invalid base URL",
			baseURL: "://invalid",
			guildID: "guild123",
			title:   "Title",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildWikiViewURL(tt.baseURL, tt.guildID, tt.title)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildWikiViewURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.want != "" && got != tt.want {
				t.Errorf("BuildWikiViewURL() = %v, want %v", got, tt.want)
			}

			// For all successful cases, verify the URL is valid and contains expected parts
			if err == nil {
				parsedURL, err := url.Parse(got)
				if err != nil {
					t.Errorf("BuildWikiViewURL() returned invalid URL: %v", err)
				}
				if parsedURL.Query().Get("guild_id") != tt.guildID {
					t.Errorf("BuildWikiViewURL() guild_id = %v, want %v", parsedURL.Query().Get("guild_id"), tt.guildID)
				}
				if parsedURL.Query().Get("title") != tt.title {
					t.Errorf("BuildWikiViewURL() title = %v, want %v", parsedURL.Query().Get("title"), tt.title)
				}
			}
		})
	}
}

func TestOIDCDiscoveryURL(t *testing.T) {
	tests := []struct {
		name   string
		issuer string
		want   string
	}{
		{
			name:   "basic issuer",
			issuer: "https://accounts.google.com",
			want:   "https://accounts.google.com/.well-known/openid-configuration",
		},
		{
			name:   "issuer with trailing slash",
			issuer: "https://accounts.google.com/",
			want:   "https://accounts.google.com/.well-known/openid-configuration",
		},
		{
			name:   "issuer with multiple trailing slashes",
			issuer: "https://example.com///",
			want:   "https://example.com/.well-known/openid-configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OIDCDiscoveryURL(tt.issuer)
			if got != tt.want {
				t.Errorf("OIDCDiscoveryURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
