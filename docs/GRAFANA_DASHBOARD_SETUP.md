# Grafana Dashboard Setup for Hivemind

## Overview

Hivemind exposes Prometheus metrics on three services. This guide covers creating Grafana dashboards to visualize them.

## Metrics Endpoints

**Service Ports:**
- **Server**: Port 9090 (gRPC port + 10, configurable via `metrics_port`)
- **Bot**: Port 9091 (configurable via `backend.metrics_port` in bot config)
- **Web**: Port 9092 (HTTP port + 10, configurable via `server.metrics_port`)

**Prometheus Scrape Config:**
```yaml
scrape_configs:
  - job_name: 'hivemind-server'
    static_configs:
      - targets: ['hivemind-server:9090']
    
  - job_name: 'hivemind-bot'
    static_configs:
      - targets: ['hivemind-bot:9091']
    
  - job_name: 'hivemind-web'
    static_configs:
      - targets: ['hivemind-web:9092']
```

## Available Metrics

### Database/Repository Metrics
- `hivemind_db_operations_total{repo, operation, status}` - Operation counts
- `hivemind_db_operation_duration_ms{repo, operation}` - Query latency
- `hivemind_db_rows_affected{repo, operation}` - Rows modified
- `hivemind_db_rows_returned{repo, operation}` - Rows read
- `hivemind_db_errors_total{repo, operation, error_type}` - Errors

### Service Layer Metrics
- `hivemind_service_operations_total{service, method, status}` - Service calls
- `hivemind_service_operation_duration_ms{service, method}` - Service latency
- `hivemind_cache_hits_total{service, cache_name}` - Cache hits
- `hivemind_cache_misses_total{service, cache_name}` - Cache misses
- `hivemind_cache_entries{service, cache_name}` - Cache size

### gRPC Handler Metrics
- `hivemind_grpc_requests_total{service, method, status_code}` - Request counts
- `hivemind_grpc_request_duration_ms{service, method}` - Request latency
- `hivemind_grpc_active_connections` - Active connections

### HTTP/Web Metrics
- `hivemind_http_requests_total{method, path, status}` - HTTP requests
- `hivemind_http_request_duration_ms{method, path}` - HTTP latency
- `hivemind_http_active_requests` - Active requests

### Discord API Metrics
- `hivemind_discord_api_calls_total{method, bucket, status_code}` - API calls
- `hivemind_discord_api_duration_ms{method, bucket}` - API latency
- `hivemind_discord_api_errors_total{bucket, error_type}` - API errors
- `hivemind_discord_ratelimit_remaining{bucket}` - Rate limit headroom
- `hivemind_discord_ratelimit_limit{bucket}` - Rate limit max
- `hivemind_discord_ratelimit_reset_timestamp{bucket}` - Rate limit reset time
- `hivemind_discord_ratelimit_hits_total{bucket}` - 429 responses

### Bot Command Metrics
- `hivemind_discord_commands_total{command, subcommand, status}` - Command execution
- `hivemind_discord_command_duration_ms{command, subcommand}` - Command latency
- `hivemind_discord_events_total{event_type}` - Gateway events
- `hivemind_discord_gateway_connected{shard}` - Connection status
- `hivemind_discord_gateway_heartbeat_ms{shard}` - Heartbeat latency

### Business Metrics
- `hivemind_wiki_pages_total{guild_id}` - Wiki pages per guild
- `hivemind_notes_total{guild_id}` - Notes per guild
- `hivemind_quotes_total{guild_id}` - Quotes per guild
- `hivemind_active_sessions` - Active user sessions
- `hivemind_registered_guilds` - Registered Discord guilds
- `hivemind_active_tokens` - Active API tokens

## Dashboard Recommendations

### 1. Overview Dashboard
**Panels:**
- Request rate (gRPC + HTTP combined)
- Error rate (all services)
- P95 latency (handlers and database)
- Active connections/requests
- Discord rate limit status

**Key Queries:**
```promql
# Total request rate
sum(rate(hivemind_grpc_requests_total[5m])) + sum(rate(hivemind_http_requests_total[5m]))

# Error rate percentage
(sum(rate(hivemind_grpc_requests_total{status_code!="OK"}[5m])) / sum(rate(hivemind_grpc_requests_total[5m]))) * 100

# P95 latency
histogram_quantile(0.95, sum(rate(hivemind_grpc_request_duration_ms_bucket[5m])) by (le))
```

### 2. Database Performance Dashboard
**Panels:**
- Query latency by repository (P50, P95, P99)
- Operations per second by type (create, read, update, delete)
- Database error rate by repository
- Slowest queries (top 10)
- Rows affected/returned

**Key Queries:**
```promql
# P95 latency by repository
histogram_quantile(0.95, sum(rate(hivemind_db_operation_duration_ms_bucket[5m])) by (repo, le))

# Operations per second
sum(rate(hivemind_db_operations_total[1m])) by (operation)

# Error rate
rate(hivemind_db_operations_total{status="error"}[5m]) / rate(hivemind_db_operations_total[5m])
```

### 3. Discord Bot Dashboard
**Panels:**
- Discord API call rate
- Discord API latency
- Rate limit gauges (remaining vs limit)
- Time until rate limit reset
- Command execution rate and success rate
- Gateway connection status
- Bot command latency by command

**Key Queries:**
```promql
# API call rate
rate(hivemind_discord_api_calls_total[1m])

# Rate limit percentage used
(1 - (hivemind_discord_ratelimit_remaining / hivemind_discord_ratelimit_limit)) * 100

# Time until reset (seconds)
hivemind_discord_ratelimit_reset_timestamp - time()

# Command success rate
sum(rate(hivemind_discord_commands_total{status="success"}[5m])) / sum(rate(hivemind_discord_commands_total[5m]))
```

### 4. Service Layer Dashboard
**Panels:**
- Cache hit rate by service
- Cache size over time
- Service operation latency
- Service success rate
- Top slowest operations

**Key Queries:**
```promql
# Cache hit rate
rate(hivemind_cache_hits_total[5m]) / (rate(hivemind_cache_hits_total[5m]) + rate(hivemind_cache_misses_total[5m]))

# Service latency
rate(hivemind_service_operation_duration_ms_sum[5m]) / rate(hivemind_service_operation_duration_ms_count[5m])
```

## Alert Rules

**Critical Alerts:**
```yaml
groups:
  - name: hivemind_critical
    rules:
      # High error rate
      - alert: HighErrorRate
        expr: rate(hivemind_grpc_requests_total{status_code!="OK"}[5m]) / rate(hivemind_grpc_requests_total[5m]) > 0.05
        for: 2m
        
      # Discord rate limit hit
      - alert: DiscordRateLimitHit
        expr: rate(hivemind_discord_ratelimit_hits_total[5m]) > 0
        for: 1m
        
      # Gateway disconnected
      - alert: DiscordGatewayDown
        expr: hivemind_discord_gateway_connected == 0
        for: 1m
```

## ConfigMap Structure

When deploying via Argo CD, create a ConfigMap for dashboard JSON:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: hivemind-grafana-dashboards
  namespace: monitoring
  labels:
    grafana_dashboard: "1"
data:
  hivemind-overview.json: |
    {
      "dashboard": { ... }
    }
```

## Implementation Status

**Phase 1 (Complete ✅):**
- Metrics registry created
- All metrics defined and exported
- `/metrics` endpoints active on all services

**Phase 2-6 (In Progress):**
- Repository instrumentation (pending)
- Service instrumentation (pending)
- Handler interceptors (pending)
- Bot command metrics (pending)
- Dashboard creation (your task)

**Note:** Currently only Discord API metrics are instrumented. Database, service, and handler metrics are defined but not yet recording data. Dashboards can be created now for Discord metrics, with panels added for other metrics as instrumentation is completed.

## Quick Start

1. **Verify Prometheus is scraping**: Check Prometheus targets page
2. **Test metrics**: Query `hivemind_discord_api_calls_total` to verify data
3. **Create dashboard**: Start with Discord API metrics (proven working)
4. **Add panels progressively**: Add DB/service panels as instrumentation completes
5. **Set up alerts**: Focus on Discord rate limits and gateway status first

## Dashboard Export/Import

Export dashboards as JSON and store them in the repo at:
- `configs/grafana/hivemind-overview.json`
- `configs/grafana/hivemind-database.json`
- `configs/grafana/hivemind-discord.json`
- `configs/grafana/hivemind-services.json`

This allows version control and GitOps deployment via Argo CD.
