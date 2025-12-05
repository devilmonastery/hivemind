package metrics

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/devilmonastery/hivemind/internal/pkg/metrics"
)

// discordMetricsTransport wraps an http.RoundTripper to collect metrics on Discord API calls
type discordMetricsTransport struct {
	base http.RoundTripper
}

// NewDiscordMetricsTransport creates a new transport wrapper that collects metrics
// for all Discord API calls. It should be installed on the DiscordGo session's HTTP client.
func NewDiscordMetricsTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &discordMetricsTransport{base: base}
}

// RoundTrip implements http.RoundTripper, wrapping the base transport with metrics collection
func (t *discordMetricsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Only instrument Discord API calls
	if !isDiscordAPIRequest(req) {
		return t.base.RoundTrip(req)
	}

	start := time.Now()
	resp, err := t.base.RoundTrip(req)
	duration := time.Since(start)

	// Normalize the route for human-readable metrics
	route := normalizeDiscordRoute(req.URL.Path)

	// Extract bucket ID and status code
	bucket := "unknown"
	statusCode := 0

	if resp != nil {
		statusCode = resp.StatusCode

		// Discord provides bucket ID in response header (most accurate for rate limiting)
		bucket = resp.Header.Get("X-RateLimit-Bucket")
		if bucket == "" {
			// Fall back to normalized path if Discord doesn't provide bucket
			bucket = route
		}

		// Track rate limit information from response headers
		trackRateLimitHeaders(resp, route, bucket)

		// Track rate limit hits (429 responses)
		if statusCode == 429 {
			metrics.DiscordRateLimitHits.WithLabelValues(route, bucket).Inc()
		}
	}

	// Record API call metrics with both route (readable) and bucket (accurate)
	metrics.DiscordAPICalls.WithLabelValues(req.Method, route, bucket, strconv.Itoa(statusCode)).Inc()
	metrics.DiscordAPIDuration.WithLabelValues(req.Method, route, bucket).Observe(float64(duration.Milliseconds()))

	// Track errors
	if err != nil || statusCode >= 400 {
		errorType := classifyDiscordError(statusCode, err)
		metrics.DiscordAPIErrors.WithLabelValues(route, bucket, errorType).Inc()
	}

	return resp, err
}

// isDiscordAPIRequest checks if the request is going to Discord's API
func isDiscordAPIRequest(req *http.Request) bool {
	host := req.URL.Host
	return strings.Contains(host, "discord.com") || strings.Contains(host, "discordapp.com")
}

// trackRateLimitHeaders extracts and records rate limit information from Discord response headers
func trackRateLimitHeaders(resp *http.Response, route, bucket string) {
	// X-RateLimit-Remaining: Number of requests remaining before rate limit
	if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
		if r, err := strconv.Atoi(remaining); err == nil {
			metrics.DiscordRateLimitRemaining.WithLabelValues(route, bucket).Set(float64(r))
		}
	}

	// X-RateLimit-Limit: Maximum number of requests allowed
	if limit := resp.Header.Get("X-RateLimit-Limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			metrics.DiscordRateLimitLimit.WithLabelValues(route, bucket).Set(float64(l))
		}
	}

	// X-RateLimit-Reset: Unix timestamp when the rate limit resets
	if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
		if r, err := strconv.ParseFloat(reset, 64); err == nil {
			metrics.DiscordRateLimitReset.WithLabelValues(route, bucket).Set(r)
		}
	}
}

// normalizeDiscordRoute normalizes Discord API routes by replacing IDs with placeholders
// This prevents high cardinality in metrics while still providing useful aggregation
func normalizeDiscordRoute(path string) string {
	patterns := []struct {
		regex   *regexp.Regexp
		replace string
	}{
		{regexp.MustCompile(`/channels/\d+`), "/channels/:id"},
		{regexp.MustCompile(`/guilds/\d+`), "/guilds/:id"},
		{regexp.MustCompile(`/users/\d+`), "/users/:id"},
		{regexp.MustCompile(`/webhooks/\d+`), "/webhooks/:id"},
		{regexp.MustCompile(`/interactions/\d+`), "/interactions/:id"},
		{regexp.MustCompile(`/messages/\d+`), "/messages/:id"},
		{regexp.MustCompile(`/members/\d+`), "/members/:id"},
		{regexp.MustCompile(`/emojis/\d+`), "/emojis/:id"},
		{regexp.MustCompile(`/roles/\d+`), "/roles/:id"},
		{regexp.MustCompile(`/invites/[a-zA-Z0-9]+`), "/invites/:code"},
	}

	normalized := path
	for _, p := range patterns {
		normalized = p.regex.ReplaceAllString(normalized, p.replace)
	}

	return normalized
}

// classifyDiscordError categorizes Discord API errors for metrics
func classifyDiscordError(statusCode int, err error) string {
	if err != nil {
		errStr := err.Error()
		switch {
		case strings.Contains(errStr, "timeout"):
			return "timeout"
		case strings.Contains(errStr, "connection"):
			return "connection"
		case strings.Contains(errStr, "TLS"):
			return "tls"
		default:
			return "network"
		}
	}

	// HTTP status code errors
	switch {
	case statusCode == 400:
		return "bad_request"
	case statusCode == 401:
		return "unauthorized"
	case statusCode == 403:
		return "forbidden"
	case statusCode == 404:
		return "not_found"
	case statusCode == 429:
		return "rate_limited"
	case statusCode >= 500:
		return "server_error"
	case statusCode >= 400:
		return "client_error"
	default:
		return "unknown"
	}
}
