package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Database/Repository Metrics
var (
	// DBOperations tracks total database operations
	DBOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hivemind_db_operations_total",
			Help: "Total database operations by repository, operation, and status",
		},
		[]string{"repo", "operation", "status"},
	)

	// DBDuration tracks database operation latency
	DBDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:                            "hivemind_db_operation_duration_ms",
			Help:                            "Database operation duration in milliseconds",
			NativeHistogramBucketFactor:     1.1,
			NativeHistogramMaxBucketNumber:  100,
			NativeHistogramMinResetDuration: 1 * time.Hour,
		},
		[]string{"repo", "operation"},
	)

	// DBRowsAffected tracks rows affected by write operations
	DBRowsAffected = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:                            "hivemind_db_rows_affected",
			Help:                            "Number of rows affected by database write operations",
			NativeHistogramBucketFactor:     1.1,
			NativeHistogramMaxBucketNumber:  100,
			NativeHistogramMinResetDuration: 1 * time.Hour,
		},
		[]string{"repo", "operation"},
	)

	// DBRowsReturned tracks rows returned by read operations
	DBRowsReturned = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:                            "hivemind_db_rows_returned",
			Help:                            "Number of rows returned by database read operations",
			NativeHistogramBucketFactor:     1.1,
			NativeHistogramMaxBucketNumber:  100,
			NativeHistogramMinResetDuration: 1 * time.Hour,
		},
		[]string{"repo", "operation"},
	)

	// DBErrors tracks database errors by type
	DBErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hivemind_db_errors_total",
			Help: "Total database errors by repository, operation, and error type",
		},
		[]string{"repo", "operation", "error_type"},
	)
)

// Service Layer Metrics
var (
	// ServiceOperations tracks service-level operations
	ServiceOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hivemind_service_operations_total",
			Help: "Total service operations by service, method, and status",
		},
		[]string{"service", "method", "status"},
	)

	// ServiceDuration tracks service operation latency
	ServiceDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:                            "hivemind_service_operation_duration_ms",
			Help:                            "Service operation duration in milliseconds",
			NativeHistogramBucketFactor:     1.1,
			NativeHistogramMaxBucketNumber:  100,
			NativeHistogramMinResetDuration: 1 * time.Hour,
		},
		[]string{"service", "method"},
	)

	// CacheHits tracks cache hits
	CacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hivemind_cache_hits_total",
			Help: "Total cache hits by service and cache name",
		},
		[]string{"service", "cache_name"},
	)

	// CacheMisses tracks cache misses
	CacheMisses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hivemind_cache_misses_total",
			Help: "Total cache misses by service and cache name",
		},
		[]string{"service", "cache_name"},
	)

	// CacheSize tracks current cache size
	CacheSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hivemind_cache_entries",
			Help: "Current number of entries in cache",
		},
		[]string{"service", "cache_name"},
	)

	// CacheEvictions tracks cache evictions
	CacheEvictions = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hivemind_cache_evictions_total",
			Help: "Total cache evictions by service and cache name",
		},
		[]string{"service", "cache_name"},
	)
)

// gRPC Handler Metrics
var (
	// GRPCRequests tracks gRPC requests
	GRPCRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hivemind_grpc_requests_total",
			Help: "Total gRPC requests by service, method, and status code",
		},
		[]string{"service", "method", "status_code"},
	)

	// GRPCDuration tracks gRPC request duration
	GRPCDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:                            "hivemind_grpc_request_duration_ms",
			Help:                            "gRPC request duration in milliseconds",
			NativeHistogramBucketFactor:     1.1,
			NativeHistogramMaxBucketNumber:  100,
			NativeHistogramMinResetDuration: 1 * time.Hour,
		},
		[]string{"service", "method"},
	)

	// GRPCActiveConnections tracks active gRPC connections
	GRPCActiveConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "hivemind_grpc_active_connections",
			Help: "Number of active gRPC connections",
		},
	)
)

// HTTP/Web Handler Metrics
var (
	// HTTPRequests tracks HTTP requests
	HTTPRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hivemind_http_requests_total",
			Help: "Total HTTP requests by method, path, and status",
		},
		[]string{"method", "path", "status"},
	)

	// HTTPDuration tracks HTTP request duration
	HTTPDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:                            "hivemind_http_request_duration_ms",
			Help:                            "HTTP request duration in milliseconds",
			NativeHistogramBucketFactor:     1.1,
			NativeHistogramMaxBucketNumber:  100,
			NativeHistogramMinResetDuration: 1 * time.Hour,
		},
		[]string{"method", "path"},
	)

	// HTTPActiveRequests tracks active HTTP requests
	HTTPActiveRequests = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "hivemind_http_active_requests",
			Help: "Number of active HTTP requests",
		},
	)
)

// Discord API Metrics
var (
	// DiscordAPICalls tracks Discord API calls
	DiscordAPICalls = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hivemind_discord_api_calls_total",
			Help: "Total Discord API calls by method, route (normalized path), bucket (Discord's rate limit bucket ID), and status code",
		},
		[]string{"method", "route", "bucket", "status_code"},
	)

	// DiscordAPIDuration tracks Discord API latency
	DiscordAPIDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:                            "hivemind_discord_api_duration_ms",
			Help:                            "Discord API call duration in milliseconds",
			NativeHistogramBucketFactor:     1.1,
			NativeHistogramMaxBucketNumber:  100,
			NativeHistogramMinResetDuration: 1 * time.Hour,
		},
		[]string{"method", "route", "bucket"},
	)

	// DiscordAPIErrors tracks Discord API errors
	DiscordAPIErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hivemind_discord_api_errors_total",
			Help: "Total Discord API errors by route, bucket, and error type",
		},
		[]string{"route", "bucket", "error_type"},
	)

	// DiscordRateLimitRemaining tracks rate limit remaining requests
	DiscordRateLimitRemaining = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hivemind_discord_ratelimit_remaining",
			Help: "Discord rate limit remaining requests (by route and bucket)",
		},
		[]string{"route", "bucket"},
	)

	// DiscordRateLimitLimit tracks rate limit maximum
	DiscordRateLimitLimit = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hivemind_discord_ratelimit_limit",
			Help: "Discord rate limit maximum requests (by route and bucket)",
		},
		[]string{"route", "bucket"},
	)

	// DiscordRateLimitReset tracks rate limit reset time
	DiscordRateLimitReset = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hivemind_discord_ratelimit_reset_timestamp",
			Help: "Discord rate limit reset time (Unix timestamp, by route and bucket)",
		},
		[]string{"route", "bucket"},
	)

	// DiscordRateLimitHits tracks rate limit hits (429 responses)
	DiscordRateLimitHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hivemind_discord_ratelimit_hits_total",
			Help: "Total Discord rate limit hits (429 responses, by route and bucket)",
		},
		[]string{"route", "bucket"},
	)

	// DiscordEvents tracks Discord gateway event counts by event type and status
	DiscordEvents = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "discord_events_total",
			Help: "Total number of Discord gateway events received",
		},
		[]string{"event_type", "status"},
	)

	// DiscordEventProcessing tracks event processing duration
	DiscordEventProcessing = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:                            "hivemind_discord_event_processing_ms",
			Help:                            "Discord event processing duration in milliseconds",
			NativeHistogramBucketFactor:     1.1,
			NativeHistogramMaxBucketNumber:  100,
			NativeHistogramMinResetDuration: 1 * time.Hour,
		},
		[]string{"event_type"},
	)

	// DiscordGatewayHeartbeat tracks gateway heartbeat latency
	DiscordGatewayHeartbeat = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hivemind_discord_gateway_heartbeat_ms",
			Help: "Discord gateway heartbeat latency in milliseconds",
		},
		[]string{"shard"},
	)

	// DiscordGatewayConnected tracks gateway connection status
	DiscordGatewayConnected = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hivemind_discord_gateway_connected",
			Help: "Discord gateway connection status (0=disconnected, 1=connected)",
		},
		[]string{"shard"},
	)

	// DiscordGatewayReconnects tracks gateway reconnections
	DiscordGatewayReconnects = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hivemind_discord_gateway_reconnects_total",
			Help: "Total Discord gateway reconnections",
		},
		[]string{"reason"},
	)
)

// Discord Bot Command Metrics
var (
	// DiscordCommands tracks bot commands executed
	DiscordCommands = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hivemind_discord_commands_total",
			Help: "Total Discord bot commands executed",
		},
		[]string{"command", "subcommand", "status"},
	)

	// DiscordCommandDuration tracks command execution duration
	DiscordCommandDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:                            "hivemind_discord_command_duration_ms",
			Help:                            "Discord bot command execution duration in milliseconds",
			NativeHistogramBucketFactor:     1.1,
			NativeHistogramMaxBucketNumber:  100,
			NativeHistogramMinResetDuration: 1 * time.Hour,
		},
		[]string{"command", "subcommand"},
	)

	// DiscordInteractions tracks interaction handling
	DiscordInteractions = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hivemind_discord_interactions_total",
			Help: "Total Discord interactions handled",
		},
		[]string{"type", "custom_id", "status"},
	)
)

// Business Metrics
var (
	// WikiPagesTotal tracks total wiki pages by guild
	WikiPagesTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hivemind_wiki_pages_total",
			Help: "Total wiki pages by guild",
		},
		[]string{"guild_id"},
	)

	// NotesTotal tracks total notes by guild
	NotesTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hivemind_notes_total",
			Help: "Total notes by guild",
		},
		[]string{"guild_id"},
	)

	// QuotesTotal tracks total quotes by guild
	QuotesTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hivemind_quotes_total",
			Help: "Total quotes by guild",
		},
		[]string{"guild_id"},
	)

	// ActiveSessions tracks active user sessions
	ActiveSessions = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "hivemind_active_sessions",
			Help: "Number of active user sessions",
		},
	)

	// RegisteredGuilds tracks registered guilds
	RegisteredGuilds = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "hivemind_registered_guilds",
			Help: "Number of registered Discord guilds",
		},
	)

	// ActiveTokens tracks active API tokens
	ActiveTokens = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "hivemind_active_tokens",
			Help: "Number of active API tokens",
		},
	)
)
