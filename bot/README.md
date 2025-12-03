# Hivemind Discord Bot

Discord bot for managing collaborative knowledge bases (wikis, notes, and quotes) directly within Discord guilds.

## Setup

### 1. Create Discord Application

1. Go to https://discord.com/developers/applications
2. Click "New Application"
3. Give it a name (e.g., "Hivemind")
4. Go to the "Bot" tab and click "Add Bot"
5. Under "Token", click "Reset Token" and copy it
6. Under "Privileged Gateway Intents", enable:
   - Server Members Intent
   - Message Content Intent

### 2. Configure Bot

Copy the example config:
```bash
cp configs/example_bot.yaml configs/dev-bot.yaml
```

Edit `configs/dev-bot.yaml` and fill in:
- `bot.token`: Your bot token from step 1
- `bot.application_id`: Your application ID (from the "General Information" tab)
- `backend.service_token`: Service token for authenticating with the backend server
  - Generate one using: `./bin/hivemind-server token --type service`
- `backend.grpc_host`: Backend server host (default: localhost)
- `backend.grpc_port`: Backend server gRPC port (default: 50051)
- `web.base_url`: Web interface URL for generating links (optional)

Or use environment variables:
```bash
export DISCORD_BOT_TOKEN="your-token-here"
export DISCORD_APPLICATION_ID="your-app-id-here"
export BACKEND_SERVICE_TOKEN="your-service-token-here"
```

### 3. Invite Bot to Server

Generate an invite URL:
1. Go to your application's "OAuth2" → "URL Generator"
2. Select scopes: `bot`, `applications.commands`
3. Select permissions:
   - Read Messages/View Channels
   - Send Messages
   - Embed Links
   - Use Slash Commands
4. Copy the generated URL and open it in your browser
5. Select a server and authorize

### 4. Build and Run

```bash
# Build the bot
make bot

# Or build manually
go build -o bin/hivemind-bot ./bot

# Run the bot
./bin/hivemind-bot run --config configs/dev-bot.yaml

# Or with make
make run-bot
```

### 5. Register Commands

**Important**: Commands must be registered separately and are NOT auto-registered on bot startup.

For testing (instant, guild-specific):
```bash
./bin/hivemind-bot register --config configs/dev-bot.yaml --guild YOUR_GUILD_ID
```

For production (global, takes up to 1 hour):
```bash
./bin/hivemind-bot register --config configs/dev-bot.yaml --global
```

**Fixing Duplicate Commands:**

If you see duplicate commands in Discord, it's because commands were registered both globally AND for a specific guild. To fix:

```bash
# Remove guild-specific duplicates
./bin/hivemind-bot register --config configs/dev-bot.yaml --guild YOUR_GUILD_ID --cleanup

# Or remove global commands
./bin/hivemind-bot register --config configs/dev-bot.yaml --global --cleanup

# Then re-register in your preferred scope
./bin/hivemind-bot register --config configs/dev-bot.yaml --global
```

The register command uses bulk overwrite, so running it multiple times is safe and won't create duplicates.

**Troubleshooting: HTTP 403 "Missing Access" Error**

If you get this error when running `--cleanup` or `register`:
```
Error: failed to remove guild commands: HTTP 403 Forbidden, {"message": "Missing Access", "code": 50001}
```

This means the bot doesn't have the `applications.commands` scope in that guild. This happens when:
- The bot was invited before `applications.commands` was added to your OAuth URL
- The bot was invited with incomplete scopes

**Solution:** Re-invite the bot with the correct scopes:
1. Generate a new invite URL with BOTH scopes:
   - Go to Discord Developer Portal → Your App → OAuth2 → URL Generator
   - Select scopes: `bot` **AND** `applications.commands`
   - Select required permissions (Read Messages, Send Messages, Embed Links, etc.)
2. Visit the generated URL
3. Select the same server (Discord will update the bot's scopes)
4. Try the cleanup/register command again

You don't need to kick the bot first - re-inviting with the correct scopes will update its permissions.

### 6. Test

In your Discord server, type `/ping` and the bot should respond with "🏓 Pong!"

## Development

### Project Structure

```
bot/
├── main.go                          # Entry point
├── run.go                           # Run command
├── register.go                      # Command registration
├── internal/
│   ├── config/                      # Configuration
│   └── bot/
│       ├── bot.go                   # Bot instance
│       ├── handlers/                # Interaction handlers
│       └── commands/                # Command definitions
```

### Adding New Commands

1. Add command definition to `internal/bot/commands/registry.go`
2. Add handler to `internal/bot/handlers/handlers.go`
3. Register commands with Discord using `./bin/hivemind-bot register`

## Running Multiple Replicas

The bot supports running multiple replicas for high availability and load distribution. Discord automatically load-balances slash command interactions across all connected bot instances.

### Local Development / Docker

In standalone mode (local dev or Docker Compose), each bot instance runs its own background sync jobs. This results in duplicate work but is safe - all operations are idempotent.

### Kubernetes Deployment

When running in Kubernetes, the bot automatically detects the environment and uses **leader election** for background sync jobs:

- Only one replica (the "leader") runs the periodic guild member sync
- If the leader fails, another replica automatically takes over
- All replicas handle Discord interactions normally
- This prevents duplicate Discord API calls and database writes

**Required Kubernetes Setup:**

1. **Service Account with RBAC permissions** for leader election
2. **Environment variable**: `POD_NAMESPACE` (set via Downward API)
3. **Lease resource access** in the namespace

Example deployment snippet:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hivemind-bot
spec:
  replicas: 3  # Multiple replicas for HA
  template:
    spec:
      serviceAccountName: hivemind-bot  # Service account with RBAC
      containers:
      - name: bot
        image: your-registry/hivemind-bot:latest
        env:
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: DISCORD_BOT_TOKEN
          valueFrom:
            secretKeyRef:
              name: hivemind-bot-secrets
              key: discord-token
```

See `configs/k8s/` for complete RBAC and deployment examples.

**How it works:**
- Bot detects Kubernetes by checking for `/var/run/secrets/kubernetes.io/serviceaccount/token`
- Uses a Lease resource named `hivemind-bot-sync-leader` for coordination
- Leader holds lease for 15s, renews every 10s
- Failed leaders are detected within 2-5 seconds

## Background Jobs

The bot runs periodic background jobs:

### Guild Member Sync

- **Frequency**: Every 24 hours
- **Purpose**: Syncs guild member data (nicknames, roles, usernames) with the database
- **Maintenance**: Automatically updates the `user_display_names` table for efficient display name lookups
- **Behavior**:
  - Standalone/Docker: All instances run sync jobs independently (idempotent, safe but duplicate work)
  - Kubernetes: Only the leader replica runs sync jobs (via leader election)

The sync ensures that display names (guild nick > global name > username) are always up-to-date in queries without expensive real-time joins.

## Commands

### Wiki Commands
- `/wiki search <query>` - Search for wiki pages
- `/wiki view <title>` - View a specific wiki page
- `/wiki edit <title>` - Edit or create a wiki page
- `/wiki merge <source> <target>` - Merge one wiki page into another

### Note Commands
- `/note create` - Create a new note
- `/note view <title>` - View a note by title
- `/note search <query>` - Search your notes

### Quote Commands
- `/quote add <text>` - Add a new quote
- `/quote random [tags]` - Get a random quote
- `/quote search <query>` - Search quotes

### Context Menu Actions
Right-click on a message to:
- **Save as Quote** - Save the message as a quote
- **Add to Note** - Add message to a note
- **Add as Wiki** - Add message to a wiki page
- **Add Note for User** - Create a private note about a user
- **View Note for User** - View your notes about a user

### Utility Commands
- `/ping` - Test if bot is alive
