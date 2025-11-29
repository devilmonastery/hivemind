package bot

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"

	"github.com/devilmonastery/hivemind/bot/internal/bot/handlers"
	"github.com/devilmonastery/hivemind/bot/internal/config"
)

// Bot represents the Discord bot instance
type Bot struct {
	config  *config.Config
	log     *slog.Logger
	session *discordgo.Session
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

	bot := &Bot{
		config:  cfg,
		log:     log,
		session: session,
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
		handlers.HandleInteraction(s, i, b.config, b.log)
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

	// TODO: Register guild in database via gRPC
}

// onGuildDelete is called when the bot is removed from a guild
func (b *Bot) onGuildDelete(s *discordgo.Session, event *discordgo.GuildDelete) {
	b.log.Info("removed from guild",
		slog.String("guild_id", event.ID),
	)

	// TODO: Mark guild as disabled in database via gRPC
}
