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
   - Message Content Intent (if needed)

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

For testing (instant, guild-specific):
```bash
./bin/hivemind-bot register --config configs/dev-bot.yaml --guild YOUR_GUILD_ID
```

For production (global, takes up to 1 hour):
```bash
./bin/hivemind-bot register --config configs/dev-bot.yaml --global
```

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
