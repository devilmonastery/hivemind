package metrics

import (
	"net/http"
	"testing"
)

func TestNormalizeDiscordRoute(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "normalize channel ID",
			path:     "/channels/123456789/messages",
			expected: "/channels/:id/messages",
		},
		{
			name:     "normalize guild ID",
			path:     "/guilds/987654321/members",
			expected: "/guilds/:id/members",
		},
		{
			name:     "normalize user ID",
			path:     "/users/111222333/profile",
			expected: "/users/:id/profile",
		},
		{
			name:     "normalize webhook ID",
			path:     "/webhooks/444555666/token",
			expected: "/webhooks/:id/token",
		},
		{
			name:     "normalize message ID",
			path:     "/channels/123/messages/456",
			expected: "/channels/:id/messages/:id",
		},
		{
			name:     "normalize invite code",
			path:     "/invites/abc123XYZ",
			expected: "/invites/:code",
		},
		{
			name:     "no normalization needed",
			path:     "/gateway",
			expected: "/gateway",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeDiscordRoute(tt.path)
			if result != tt.expected {
				t.Errorf("normalizeDiscordRoute(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestClassifyDiscordError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		err        error
		expected   string
	}{
		{
			name:       "bad request",
			statusCode: 400,
			err:        nil,
			expected:   "bad_request",
		},
		{
			name:       "unauthorized",
			statusCode: 401,
			err:        nil,
			expected:   "unauthorized",
		},
		{
			name:       "forbidden",
			statusCode: 403,
			err:        nil,
			expected:   "forbidden",
		},
		{
			name:       "not found",
			statusCode: 404,
			err:        nil,
			expected:   "not_found",
		},
		{
			name:       "rate limited",
			statusCode: 429,
			err:        nil,
			expected:   "rate_limited",
		},
		{
			name:       "server error",
			statusCode: 500,
			err:        nil,
			expected:   "server_error",
		},
		{
			name:       "client error",
			statusCode: 418,
			err:        nil,
			expected:   "client_error",
		},
		{
			name:       "unknown",
			statusCode: 200,
			err:        nil,
			expected:   "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyDiscordError(tt.statusCode, tt.err)
			if result != tt.expected {
				t.Errorf("classifyDiscordError(%d, %v) = %q, want %q", tt.statusCode, tt.err, result, tt.expected)
			}
		})
	}
}

func TestIsDiscordAPIRequest(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "discord.com",
			url:      "https://discord.com/api/v10/users/@me",
			expected: true,
		},
		{
			name:     "discordapp.com",
			url:      "https://discordapp.com/api/v10/users/@me",
			expected: true,
		},
		{
			name:     "other domain",
			url:      "https://example.com/api/test",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", tt.url, nil)
			result := isDiscordAPIRequest(req)
			if result != tt.expected {
				t.Errorf("isDiscordAPIRequest(%q) = %v, want %v", tt.url, result, tt.expected)
			}
		})
	}
}
