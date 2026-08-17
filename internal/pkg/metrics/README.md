# Metrics Package

This package provides Prometheus metrics collection for the Hivemind application.

## Overview

The metrics package defines and registers all Prometheus metrics used throughout the application. Metrics are automatically collected and exposed via HTTP endpoints on each service.

## Metrics Endpoints

Each service exposes metrics on its configured metrics port:

- **Server**: `http://localhost:4163/metrics` (gRPC port + 10)
- **Bot**: `http://localhost:9091/metrics`
- **Web**: `http://localhost:9092/metrics`

Additional endpoints available on all services:
- `/health` - Health check endpoint
- `/readiness` - Readiness check endpoint

## Metric Categories

### 1. Database/Repository Metrics

Track database operations across all repositories:

- `hivemind_db_operations_total{repo, operation, status}` - Total operations
- `hivemind_db_operation_duration_ms{repo, operation}` - Query latency
- `hivemind_db_rows_affected{repo, operation}` - Rows affected by writes
- `hivemind_db_rows_returned{repo, operation}` - Rows returned by reads
- `hivemind_db_errors_total{repo, operation, error_type}` - Database errors

### 2. Service Layer Metrics

Track business logic operations:

- `hivemind_service_operations_total{service, method, status}` - Service operations
- `hivemind_service_operation_duration_ms{service, method}` - Service latency
- `hivemind_cache_hits_total{service, cache_name}` - Cache hits
- `hivemind_cache_misses_total{service, cache_name}` - Cache misses
- `hivemind_cache_entries{service, cache_name}` - Current cache size
- `hivemind_cache_evictions_total{service, cache_name}` - Cache evictions

### 3. gRPC Handler Metrics

Track gRPC request handling:

- `hivemind_grpc_requests_total{service, method, status_code}` - gRPC requests
- `hivemind_grpc_request_duration_ms{service, method}` - gRPC latency
- `hivemind_grpc_active_connections` - Active gRPC connections

### 4. HTTP/Web Metrics

Track HTTP request handling:

- `hivemind_http_requests_total{method, path, status}` - HTTP requests
- `hivemind_http_request_duration_ms{method, path}` - HTTP latency
- `hivemind_http_active_requests` - Active HTTP requests

### 5. Discord API Metrics

Track Discord API interactions:

- `hivemind_discord_api_calls_total{method, bucket, status_code}` - API calls
- `hivemind_discord_api_duration_ms{method, bucket}` - API latency
- `hivemind_discord_api_errors_total{bucket, error_type}` - API errors
- `hivemind_discord_ratelimit_remaining{bucket}` - Rate limit remaining
- `hivemind_discord_ratelimit_limit{bucket}` - Rate limit maximum
- `hivemind_discord_ratelimit_reset_timestamp{bucket}` - Rate limit reset time
- `hivemind_discord_ratelimit_hits_total{bucket}` - Rate limit hits (429s)
- `hivemind_discord_events_total{event_type}` - Gateway events
- `hivemind_discord_event_processing_ms{event_type}` - Event processing time
- `hivemind_discord_gateway_heartbeat_ms{shard}` - Gateway heartbeat latency
- `hivemind_discord_gateway_connected{shard}` - Gateway connection status
- `hivemind_discord_gateway_reconnects_total{reason}` - Gateway reconnections

### 6. Bot Command Metrics

Track Discord bot command execution:

- `hivemind_discord_commands_total{command, subcommand, status}` - Commands executed
- `hivemind_discord_command_duration_ms{command, subcommand}` - Command latency
- `hivemind_discord_interactions_total{type, custom_id, status}` - Interactions handled

### 7. Business Metrics

Domain-specific aggregate metrics:

- `hivemind_wiki_pages_total{guild_id}` - Total wiki pages per guild
- `hivemind_notes_total{guild_id}` - Total notes per guild
- `hivemind_quotes_total{guild_id}` - Total quotes per guild
- `hivemind_active_sessions` - Active user sessions
- `hivemind_registered_guilds` - Registered Discord guilds
- `hivemind_active_tokens` - Active API tokens

## Usage

### Importing Metrics

Metrics are automatically registered when the package is imported:

```go
import _ "github.com/devilmonastery/hivemind/internal/pkg/metrics"
```

The blank import (`_`) ensures the package's `init()` functions run, which register all metrics with Prometheus.

### Recording Metrics

Use the exported metric variables to record measurements:

```go
import "github.com/devilmonastery/hivemind/internal/pkg/metrics"

// Record a database operation
timer := prometheus.NewTimer(metrics.DBDuration.WithLabelValues("wiki_page", "create"))
defer timer.ObserveDuration()

err := repo.Create(ctx, page)

status := "success"
if err != nil {
    status = "error"
}
metrics.DBOperations.WithLabelValues("wiki_page", "create", status).Inc()
```

## Prometheus Configuration

Add these scrape targets to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'hivemind-server'
    static_configs:
      - targets: ['localhost:4163']
    scrape_interval: 15s
    
  - job_name: 'hivemind-bot'
    static_configs:
      - targets: ['localhost:9091']
    scrape_interval: 15s
    
  - job_name: 'hivemind-web'
    static_configs:
      - targets: ['localhost:9092']
    scrape_interval: 15s
```

## Example Queries

### Database Performance

```promql
# Average query latency
rate(hivemind_db_operation_duration_ms_sum[5m]) / rate(hivemind_db_operation_duration_ms_count[5m])

# p95 latency for searches
histogram_quantile(0.95, rate(hivemind_db_operation_duration_ms_bucket{operation="search"}[5m]))

# Error rate
rate(hivemind_db_operations_total{status="error"}[5m]) / rate(hivemind_db_operations_total[5m])
```

### Service Performance

```promql
# Cache hit rate
rate(hivemind_cache_hits_total[5m]) / (rate(hivemind_cache_hits_total[5m]) + rate(hivemind_cache_misses_total[5m]))

# Service latency
rate(hivemind_service_operation_duration_ms_sum[5m]) / rate(hivemind_service_operation_duration_ms_count[5m])
```

### Discord API

```promql
# API call rate
rate(hivemind_discord_api_calls_total[1m])

# Rate limit status
hivemind_discord_ratelimit_remaining

# Rate limit hits
rate(hivemind_discord_ratelimit_hits_total[1m])
```

## Development

When adding new metrics:

1. Define the metric in `registry.go` using `promauto.New*Vec()`
2. Use appropriate metric type:
   - **Counter**: Monotonically increasing values (requests, errors)
   - **Gauge**: Values that can go up or down (connections, cache size)
   - **Histogram**: Distributions of values (latency, sizes)
3. Add helpful labels for filtering and aggregation
4. Document the metric in this README
5. Update METRICS_PLAN.md with the new metric

## Testing

Metrics can be tested using the `prometheus/client_golang/prometheus/testutil` package:

```go
import "github.com/prometheus/client_golang/prometheus/testutil"

func TestMetrics(t *testing.T) {
    // Reset metrics
    metrics.DBOperations.Reset()
    
    // Perform operation
    metrics.DBOperations.WithLabelValues("test_repo", "create", "success").Inc()
    
    // Verify metric
    value := testutil.ToFloat64(metrics.DBOperations.WithLabelValues("test_repo", "create", "success"))
    assert.Equal(t, 1.0, value)
}
```

## Performance

Metrics collection has minimal overhead:

- **Memory**: ~20-50MB per service
- **CPU**: ~0.5-1% during normal operation
- **Latency**: ~0.1-0.5ms per request
- **Network**: Minimal (only when Prometheus scrapes)

## Best Practices

1. **Label Cardinality**: Keep label cardinality low. Don't use user IDs, message IDs, or other high-cardinality values as labels.
2. **Histogram Buckets**: Use appropriate buckets for your latency patterns. Default buckets cover 1ms to 5s.
3. **Naming**: Follow Prometheus naming conventions: `<namespace>_<subsystem>_<name>_<unit>`
4. **Documentation**: Always include a `Help` string describing what the metric measures
5. **Testing**: Test metric collection in unit tests to ensure accurate tracking

## Next Steps

- Phase 2: Implement repository layer decorators
- Phase 3: Implement service layer decorators
- Phase 4: Implement handler layer interceptors/middleware
- Phase 5: Implement Discord API instrumentation
- Phase 6: Create Grafana dashboards and alerts
