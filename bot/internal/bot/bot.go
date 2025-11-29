package bot

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"

	discordpb "github.com/devilmonastery/hivemind/api/generated/go/discordpb"
	"github.com/devilmonastery/hivemind/bot/internal/bot/handlers"
	"github.com/devilmonastery/hivemind/bot/internal/config"
	botgrpc "github.com/devilmonastery/hivemind/bot/internal/grpc"
	"github.com/devilmonastery/hivemind/internal/client"
)

// Bot represents the Discord bot instance
type Bot struct {
	config     *config.Config
	log        *slog.Logger
	session    *discordgo.Session
	grpcClient *client.Client
}

// New creates a new Bot instance
func New(cfg *config.Config, log *slog.Logger) (*Bot, error) {
	// Create Discord session
	session, err := discordgo.New("Bot " + cfg.Bot.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	// Set intents
	session.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsGuildMembers

	// Create gRPC client for backend communication
	grpcClient, err := botgrpc.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}

	log.Info("connected to backend",
		slog.String("host", cfg.Backend.GRPCHost),
		slog.Int("port", cfg.Backend.GRPCPort),
	)

	bot := &Bot{
		config:     cfg,
		log:        log,
		session:    session,
		grpcClient: grpcClient,
	}

	// Register handlers
	bot.registerHandlers()

	return bot, nil
}

// registerHandlers sets up all event and interaction handlers
func (b *Bot) registerHandlers() {
	// Ready event
	b.session.AddHandler(b.onReady)

	// Guild events
	b.session.AddHandler(b.onGuildCreate)
	b.session.AddHandler(b.onGuildDelete)

	// Interaction handlers
	b.session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		handlers.HandleInteraction(s, i, b.config, b.log, b.grpcClient)
	})
}

// Start opens the Discord WebSocket connection
func (b *Bot) Start() error {
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("failed to open Discord connection: %w", err)
	}
	return nil
}

// Stop gracefully closes the Discord connection
func (b *Bot) Stop(ctx context.Context) error {
	// Close gRPC connection
	if b.grpcClient != nil {
		if err := b.grpcClient.Close(); err != nil {
			b.log.Warn("failed to close gRPC client", slog.String("error", err.Error()))
		}
	}

	// Close Discord session
	if b.session != nil {
		return b.session.Close()
	}
	return nil
}

// onReady is called when the bot successfully connects to Discord
func (b *Bot) onReady(s *discordgo.Session, event *discordgo.Ready) {
	b.log.Info("bot connected to Discord",
		slog.String("username", event.User.Username),
		slog.String("discriminator", event.User.Discriminator),
		slog.Int("guilds", len(event.Guilds)),
	)

	// Set bot status
	err := s.UpdateGameStatus(0, "/wiki • /note • /quote")
	if err != nil {
		b.log.Warn("failed to set bot status", slog.String("error", err.Error()))
	}
}

// onGuildCreate is called when the bot joins a guild or becomes available
func (b *Bot) onGuildCreate(s *discordgo.Session, event *discordgo.GuildCreate) {
	b.log.Info("guild available",
		slog.String("guild_id", event.ID),
		slog.String("guild_name", event.Name),
		slog.Int("member_count", event.MemberCount),
	)

	// Register/update guild in database via gRPC
	ctx := context.Background()
	discordClient := discordpb.NewDiscordServiceClient(b.grpcClient.Conn())

	iconURL := ""
	if event.Icon != "" {
		iconURL = fmt.Sprintf("https://cdn.discordapp.com/icons/%s/%s.png", event.ID, event.Icon)
	}

	_, err := discordClient.UpsertGuild(ctx, &discordpb.UpsertGuildRequest{
		GuildId:        event.ID,
		GuildName:      event.Name,
		IconUrl:        iconURL,
		OwnerDiscordId: event.OwnerID,
	})
	if err != nil {
		b.log.Error("failed to upsert guild",
			slog.String("guild_id", event.ID),
			slog.String("error", err.Error()))
	} else {
		b.log.Info("guild registered in database",
			slog.String("guild_id", event.ID),
			slog.String("guild_name", event.Name))
	}
}

// onGuildDelete is called when the bot is removed from a guild
func (b *Bot) onGuildDelete(s *discordgo.Session, event *discordgo.GuildDelete) {
	b.log.Info("removed from guild",
		slog.String("guild_id", event.ID),
	)

	// Disable guild in database via gRPC
	ctx := context.Background()
	discordClient := discordpb.NewDiscordServiceClient(b.grpcClient.Conn())

	_, err := discordClient.DisableGuild(ctx, &discordpb.DisableGuildRequest{
		GuildId: event.ID,
	})
	if err != nil {
		b.log.Error("failed to disable guild",
			slog.String("guild_id", event.ID),
			slog.String("error", err.Error()))
	} else {
		b.log.Info("guild disabled in database",
			slog.String("guild_id", event.ID))
	}
}
