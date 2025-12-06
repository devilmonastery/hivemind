package bot

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
	"google.golang.org/protobuf/types/known/timestamppb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	discordpb "github.com/devilmonastery/hivemind/api/generated/go/discordpb"
	"github.com/devilmonastery/hivemind/bot/internal/bot/handlers"
	"github.com/devilmonastery/hivemind/bot/internal/config"
	botgrpc "github.com/devilmonastery/hivemind/bot/internal/grpc"
	botmetrics "github.com/devilmonastery/hivemind/bot/internal/metrics"
	"github.com/devilmonastery/hivemind/internal/client"
	"github.com/devilmonastery/hivemind/internal/pkg/metrics"
	"github.com/devilmonastery/hivemind/internal/pkg/urlutil"
)

// Bot represents the Discord bot instance
type Bot struct {
	config     *config.Config
	log        *slog.Logger
	session    *discordgo.Session
	grpcClient *client.Client

	// Autocomplete cache
	titlesCache *handlers.TitlesCache

	// Sync context for background jobs
	syncCtx    context.Context
	syncCancel context.CancelFunc
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
		discordgo.IntentsGuildMembers |
		discordgo.IntentsMessageContent

	// Wrap HTTP client with metrics transport for Discord API monitoring
	session.Client.Transport = botmetrics.NewDiscordMetricsTransport(nil)

	// Create gRPC client for backend communication
	grpcClient, err := botgrpc.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}

	log.Info("connected to backend",
		slog.String("host", cfg.Backend.GRPCHost),
		slog.Int("port", cfg.Backend.GRPCPort),
	)

	// Create context for background sync jobs
	syncCtx, syncCancel := context.WithCancel(context.Background())

	bot := &Bot{
		config:      cfg,
		log:         log,
		session:     session,
		grpcClient:  grpcClient,
		titlesCache: handlers.NewTitlesCache(1 * time.Minute),
		syncCtx:     syncCtx,
		syncCancel:  syncCancel,
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

	// Member events (for real-time membership sync)
	b.session.AddHandler(b.onGuildMemberAdd)
	b.session.AddHandler(b.onGuildMemberUpdate)
	b.session.AddHandler(b.onGuildMemberRemove)

	// Interaction handlers
	b.session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		handlers.HandleInteraction(s, i, b.config, b.log, b.grpcClient, b.titlesCache)
	})
}

// Start opens the Discord WebSocket connection
func (b *Bot) Start() error {
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("failed to open Discord connection: %w", err)
	}

	// Start background member sync job
	// Use leader election if running in Kubernetes, otherwise run directly
	if b.isKubernetes() {
		b.log.Info("detected Kubernetes environment, using leader election for syncs")
		go b.startWithLeaderElection(b.syncCtx)
	} else {
		b.log.Info("detected standalone environment, running syncs directly")
		go b.StartMemberSync(b.syncCtx)
	}

	return nil
}

// Stop gracefully closes the Discord connection
func (b *Bot) Stop(ctx context.Context) error {
	// Cancel sync context to stop background jobs
	if b.syncCancel != nil {
		b.syncCancel()
	}

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
	start := time.Now()
	status := "success"
	defer func() {
		if r := recover(); r != nil {
			status = "error"
			panic(r)
		}
		metrics.DiscordEvents.WithLabelValues("ready", status).Inc()
		metrics.DiscordEventProcessing.WithLabelValues("ready").Observe(float64(time.Since(start).Milliseconds()))
	}()

	b.log.Info("bot connected to Discord",
		slog.String("username", event.User.Username),
		slog.String("discriminator", event.User.Discriminator),
		slog.Int("guilds", len(event.Guilds)),
	)

	// Log reactions configuration
	if b.config.Features.Reactions.Enabled {
		b.log.Info("reactions enabled",
			slog.String("quote_emoji_id", b.config.Features.Reactions.QuoteEmojiID),
			slog.String("wiki_emoji_id", b.config.Features.Reactions.WikiEmojiID),
			slog.String("hivemind_emoji_id", b.config.Features.Reactions.HivemindEmojiID))
	} else {
		b.log.Info("reactions disabled")
	}

	// Mark gateway as connected
	metrics.DiscordGatewayConnected.WithLabelValues("0").Set(1)

	// Set bot status
	err := s.UpdateGameStatus(0, "/wiki • /note • /quote")
	if err != nil {
		b.log.Warn("failed to set bot status", slog.String("error", err.Error()))
		status = "error"
	}
}

// onGuildCreate is called when the bot joins a guild or becomes available
func (b *Bot) onGuildCreate(s *discordgo.Session, event *discordgo.GuildCreate) {
	start := time.Now()
	status := "success"
	defer func() {
		if r := recover(); r != nil {
			status = "error"
			panic(r)
		}
		metrics.DiscordEvents.WithLabelValues("guild_create", status).Inc()
		metrics.DiscordEventProcessing.WithLabelValues("guild_create").Observe(float64(time.Since(start).Milliseconds()))
	}()

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
		iconURL = urlutil.DiscordCDNIconURL(event.ID, event.Icon)
	}

	_, err := discordClient.UpsertGuild(ctx, &discordpb.UpsertGuildRequest{
		GuildId:        event.ID,
		GuildName:      event.Name,
		IconUrl:        iconURL,
		OwnerDiscordId: event.OwnerID,
	})
	if err != nil {
		status = "error"
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
	start := time.Now()
	status := "success"
	defer func() {
		if r := recover(); r != nil {
			status = "error"
			panic(r)
		}
		metrics.DiscordEvents.WithLabelValues("guild_delete", status).Inc()
		metrics.DiscordEventProcessing.WithLabelValues("guild_delete").Observe(float64(time.Since(start).Milliseconds()))
	}()

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
		status = "error"
		b.log.Error("failed to disable guild",
			slog.String("guild_id", event.ID),
			slog.String("error", err.Error()))
	} else {
		b.log.Info("guild disabled in database",
			slog.String("guild_id", event.ID))
	}
}

// onGuildMemberAdd is called when a member joins a guild
func (b *Bot) onGuildMemberAdd(s *discordgo.Session, event *discordgo.GuildMemberAdd) {
	start := time.Now()
	status := "success"
	defer func() {
		if r := recover(); r != nil {
			status = "error"
			panic(r)
		}
		metrics.DiscordEvents.WithLabelValues("guild_member_add", status).Inc()
		metrics.DiscordEventProcessing.WithLabelValues("guild_member_add").Observe(float64(time.Since(start).Milliseconds()))
	}()

	b.log.Info("member joined guild",
		slog.String("guild_id", event.GuildID),
		slog.String("discord_id", event.User.ID),
		slog.String("username", event.User.Username))

	ctx := context.Background()
	discordClient := discordpb.NewDiscordServiceClient(b.grpcClient.Conn())

	// Prepare guild member data
	req := &discordpb.UpsertGuildMemberRequest{
		GuildId:         event.GuildID,
		DiscordId:       event.User.ID,
		JoinedAt:        timestamppb.New(event.JoinedAt),
		Roles:           event.Roles,
		DiscordUsername: event.User.Username,
	}

	if event.User.GlobalName != "" {
		req.DiscordGlobalName = event.User.GlobalName
	}
	if event.User.Avatar != "" {
		req.AvatarHash = event.User.Avatar
	}
	if event.Nick != "" {
		req.GuildNick = event.Nick
	}
	if event.Avatar != "" {
		req.GuildAvatarHash = event.Avatar
	}

	_, err := discordClient.UpsertGuildMember(ctx, req)
	if err != nil {
		status = "error"
		b.log.Error("failed to upsert guild member",
			slog.String("guild_id", event.GuildID),
			slog.String("discord_id", event.User.ID),
			slog.String("error", err.Error()))
	}
}

// onGuildMemberUpdate is called when a member's data changes (nickname, roles, avatar)
func (b *Bot) onGuildMemberUpdate(s *discordgo.Session, event *discordgo.GuildMemberUpdate) {
	start := time.Now()
	status := "success"
	defer func() {
		if r := recover(); r != nil {
			status = "error"
			panic(r)
		}
		metrics.DiscordEvents.WithLabelValues("guild_member_update", status).Inc()
		metrics.DiscordEventProcessing.WithLabelValues("guild_member_update").Observe(float64(time.Since(start).Milliseconds()))
	}()

	b.log.Debug("member updated in guild",
		slog.String("guild_id", event.GuildID),
		slog.String("discord_id", event.User.ID))

	ctx := context.Background()
	discordClient := discordpb.NewDiscordServiceClient(b.grpcClient.Conn())

	// Prepare updated member data
	req := &discordpb.UpsertGuildMemberRequest{
		GuildId:         event.GuildID,
		DiscordId:       event.User.ID,
		JoinedAt:        timestamppb.New(event.JoinedAt),
		Roles:           event.Roles,
		DiscordUsername: event.User.Username, // Include for discord_users update
	}

	if event.Nick != "" {
		req.GuildNick = event.Nick
	}
	if event.Avatar != "" {
		req.GuildAvatarHash = event.Avatar
	}
	if event.User.GlobalName != "" {
		req.DiscordGlobalName = event.User.GlobalName
	}
	if event.User.Avatar != "" {
		req.AvatarHash = event.User.Avatar
	}

	_, err := discordClient.UpsertGuildMember(ctx, req)
	if err != nil {
		status = "error"
		b.log.Error("failed to update guild member",
			slog.String("guild_id", event.GuildID),
			slog.String("discord_id", event.User.ID),
			slog.String("error", err.Error()))
	}
}

// onGuildMemberRemove is called when a member leaves or is kicked from a guild
func (b *Bot) onGuildMemberRemove(s *discordgo.Session, event *discordgo.GuildMemberRemove) {
	start := time.Now()
	status := "success"
	defer func() {
		if r := recover(); r != nil {
			status = "error"
			panic(r)
		}
		metrics.DiscordEvents.WithLabelValues("guild_member_remove", status).Inc()
		metrics.DiscordEventProcessing.WithLabelValues("guild_member_remove").Observe(float64(time.Since(start).Milliseconds()))
	}()

	b.log.Info("member left guild",
		slog.String("guild_id", event.GuildID),
		slog.String("discord_id", event.User.ID),
		slog.String("username", event.User.Username))

	ctx := context.Background()
	discordClient := discordpb.NewDiscordServiceClient(b.grpcClient.Conn())

	_, err := discordClient.RemoveGuildMember(ctx, &discordpb.RemoveGuildMemberRequest{
		GuildId:   event.GuildID,
		DiscordId: event.User.ID,
	})
	if err != nil {
		status = "error"
		b.log.Error("failed to remove guild member",
			slog.String("guild_id", event.GuildID),
			slog.String("discord_id", event.User.ID),
			slog.String("error", err.Error()))
	}
}

// isKubernetes checks if the bot is running inside a Kubernetes cluster
// by checking for the service account token that Kubernetes automatically mounts
func (b *Bot) isKubernetes() bool {
	_, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token")
	return err == nil
}

// startWithLeaderElection starts the sync job using Kubernetes leader election
// Only one replica will run syncs at a time, preventing duplicate work
func (b *Bot) startWithLeaderElection(ctx context.Context) {
	config, err := rest.InClusterConfig()
	if err != nil {
		b.log.Error("failed to get k8s config, falling back to direct sync",
			slog.String("error", err.Error()))
		b.StartMemberSync(ctx)
		return
	}

	client := kubernetes.NewForConfigOrDie(config)

	id := os.Getenv("HOSTNAME")
	if id == "" {
		b.log.Warn("HOSTNAME not set, falling back to direct sync")
		b.StartMemberSync(ctx)
		return
	}

	// Get namespace from environment variable or service account mount
	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		// Try reading from service account namespace file
		nsBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err == nil && len(nsBytes) > 0 {
			namespace = string(nsBytes)
			b.log.Info("detected namespace from service account",
				slog.String("namespace", namespace))
		} else {
			namespace = "default"
			b.log.Warn("could not detect namespace, falling back to default",
				slog.String("namespace", namespace))
		}
	} else {
		b.log.Info("using namespace from POD_NAMESPACE env var",
			slog.String("namespace", namespace))
	}

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      "hivemind-bot-sync-leader",
			Namespace: namespace,
		},
		Client: client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: id,
		},
	}

	b.log.Info("starting leader election for sync job",
		slog.String("identity", id),
		slog.String("namespace", namespace))

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				b.log.Info("elected as sync leader, starting member sync job",
					slog.String("identity", id))
				b.StartMemberSync(ctx)
			},
			OnStoppedLeading: func() {
				b.log.Warn("lost sync leadership, stopping sync job",
					slog.String("identity", id))
				// Don't exit - keep handling Discord interactions
				// Just stop the sync job by letting the context cancel
			},
			OnNewLeader: func(identity string) {
				if identity != id {
					b.log.Info("new sync leader elected",
						slog.String("leader", identity),
						slog.String("self", id))
				}
			},
		},
	})
}
