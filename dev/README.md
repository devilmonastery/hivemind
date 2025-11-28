# Development Tools

This directory contains development and testing utilities for the Snippets application.

## Test Data Generation

### generate-test-data.go

Populates the database with realistic test data for load and performance testing.

**Features:**
- Creates thousands of test users with sequential naming
- Generates hundreds of snippets per user spread across a configurable time period
- Realistic date distribution (weighted towards recent dates)
- 50% of snippets include hashtags from a common pool
- Direct database insertion for maximum speed
- Progress bars and statistics
- Reads database configuration from existing server config files

**Usage:**

```bash
# Default: 2000 users, 200 snippets each, using configs/dev-server.yaml
go run dev/generate-test-data.go

# Custom values
go run dev/generate-test-data.go -users=100 -snippets=50 -days=180

# Use different config file
go run dev/generate-test-data.go -config=configs/snoodev-server.yaml

# Clean existing test data first, then generate new data
go run dev/generate-test-data.go -clean

# Dry run (show what would be created without actually creating it)
go run dev/generate-test-data.go -dry-run

# Custom user prefix
go run dev/generate-test-data.go -prefix=loadtest
```

**Command-line Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | `configs/dev-server.yaml` | Path to server configuration file |
| `-users` | `2000` | Number of test users to create |
| `-snippets` | `200` | Number of snippets per user |
| `-days` | `365` | Number of days to spread snippets across |
| `-clean` | `false` | Delete existing test data before generating |
| `-dry-run` | `false` | Show statistics without creating data |
| `-prefix` | `testuser` | Prefix for test user emails |

**Expected Performance:**

With direct database inserts:
- **Users**: ~2 seconds for 2000 users
- **Snippets**: ~60-120 seconds for 400,000 snippets (2000 users × 200 snippets)
- **Total**: ~2-3 minutes for full dataset

**Test Data Format:**

Users:
- Email: `testuser0001@example.com` through `testuser2000@example.com`
- Name: `Test User 1` through `Test User 2000`
- Provider: `local`
- Role: `user`
- Created dates spread across the time period

Snippets:
- Dates: Distributed across last N days (weighted towards recent)
- Content: 1-5 bullet points with realistic work log phrases
- Hashtags: 50% of snippets include 1-3 hashtags from pool:
  - `#project`, `#meeting`, `#bugfix`, `#feature`, `#review`, `#deployment`
  - `#testing`, `#documentation`, `#refactor`, `#research`, `#planning`
  - `#oncall`, `#incident`, `#performance`, `#security`, `#backend`, `#frontend`

**Prerequisites:**

The script requires the `progressbar` package:

```bash
go get github.com/schollz/progressbar/v3
```

### clean-test-data.go

Removes all test data from the database.

**Usage:**

```bash
# Interactive mode (will ask for confirmation)
go run dev/clean-test-data.go

# Skip confirmation prompt
go run dev/clean-test-data.go -yes

# Use different config file
go run dev/clean-test-data.go -config=configs/snoodev-server.yaml

# Clean users with different prefix
go run dev/clean-test-data.go -prefix=loadtest
```

**Command-line Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | `configs/dev-server.yaml` | Path to server configuration file |
| `-prefix` | `testuser` | Prefix for test user emails to clean |
| `-yes` | `false` | Skip confirmation prompt |

**What it deletes:**
- All users matching the prefix pattern (e.g., `testuser*@example.com`)
- All snippets belonging to those users
- Respects foreign key constraints (deletes snippets first)

## Other Development Scripts

### docker-compose.yaml

Docker Compose configuration for running PostgreSQL locally.

```bash
# Start PostgreSQL
docker-compose -f dev/docker-compose.yaml up -d

# Stop PostgreSQL
docker-compose -f dev/docker-compose.yaml down

# View logs
docker-compose -f dev/docker-compose.yaml logs -f
```

### start-server.sh

Convenience script to build and start the gRPC server with development configuration.

```bash
./dev/start-server.sh
```

## Best Practices

### Before Running Tests

1. **Start with a small dataset** to verify everything works:
   ```bash
   go run dev/generate-test-data.go -users=10 -snippets=10
   ```

2. **Clean up test data** after testing:
   ```bash
   go run dev/clean-test-data.go -yes
   ```

3. **Monitor database size** - 2000 users × 200 snippets = 400,000 records

### Configuration

Both scripts read database configuration from your server config file (`configs/dev-server.yaml` by default). Ensure your config file has correct database credentials:

```yaml
database:
  postgres:
    host: "localhost"
    port: 5432
    database: "snippets"
    user: "postgres"
    password: "postgres"
    sslmode: "disable"
```

### Performance Tuning

For faster generation:
- Use local PostgreSQL (avoid network latency)
- Use SSD storage
- Increase PostgreSQL's `max_connections` if needed
- Run with smaller batch sizes if memory is limited

### Troubleshooting

**"Failed to connect to database"**
- Verify PostgreSQL is running: `docker-compose -f dev/docker-compose.yaml ps`
- Check config file path and database credentials
- Try connecting manually: `psql -h localhost -U postgres -d snippets`

**"Failed to initialize ID generator"**
- This is usually fine - the ID generator uses node ID 999 for test data
- If it fails, check that the system clock is correct

**"Snippet generation is slow"**
- Progress bar updates may cause terminal slowdown
- Consider running with output redirected: `go run dev/generate-test-data.go > output.log 2>&1`
- Verify database has sufficient resources

**"Out of memory"**
- Reduce batch size (currently hardcoded at 500 users, 1000 snippets)
- Generate data in multiple passes with fewer users per run

## Contributing

When adding new development tools:
1. Add a descriptive comment header to the script
2. Include usage examples in this README
3. Use existing config package to load configuration
4. Add progress indicators for long-running operations
5. Include cleanup/rollback mechanisms where appropriate

