package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/bwmarrin/discordgo"
	"github.com/spf13/cobra"

	"github.com/devilmonastery/hivemind/bot/internal/bot/commands"
	"github.com/devilmonastery/hivemind/bot/internal/config"
)

func newRegisterCommand() *cobra.Command {
	var (
		configPath string
		guildID    string
		global     bool
		cleanup    bool
	)

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register slash commands with Discord",
		Long: `Register all slash commands with Discord API. 
Use --guild for testing (instant), or --global for production (takes up to 1 hour to propagate).
Use --cleanup to remove ALL commands without registering new ones (useful for fixing duplicates).

Note: Using bulk overwrite to prevent duplicates. This replaces ALL commands atomically.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			log := slog.New(slog.NewTextHandler(os.Stdout, nil))

			// Load configuration
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create Discord session
			session, err := discordgo.New("Bot " + cfg.Bot.Token)
			if err != nil {
				return fmt.Errorf("failed to create Discord session: %w", err)
			}

			// Cleanup mode: remove all commands
			if cleanup {
				if global {
					log.Info("removing all global commands")
					_, err := session.ApplicationCommandBulkOverwrite(cfg.Bot.ApplicationID, "", []*discordgo.ApplicationCommand{})
					if err != nil {
						return fmt.Errorf("failed to remove global commands: %w", err)
					}
					log.Info("all global commands removed")
				} else {
					if guildID == "" {
						return fmt.Errorf("--guild is required when not using --global")
					}
					log.Info("removing all guild commands", slog.String("guild_id", guildID))
					_, err := session.ApplicationCommandBulkOverwrite(cfg.Bot.ApplicationID, guildID, []*discordgo.ApplicationCommand{})
					if err != nil {
						return fmt.Errorf("failed to remove guild commands: %w", err)
					}
					log.Info("all guild commands removed", slog.String("guild_id", guildID))
				}
				return nil
			}

			// Get command definitions
			commandDefs := commands.GetDefinitions()

			// Use bulk overwrite to atomically replace all commands (prevents duplicates)
			if global {
				log.Info("registering commands globally using bulk overwrite (may take up to 1 hour)",
					slog.Int("command_count", len(commandDefs)))

				registeredCmds, err := session.ApplicationCommandBulkOverwrite(cfg.Bot.ApplicationID, "", commandDefs)
				if err != nil {
					return fmt.Errorf("failed to bulk register commands: %w", err)
				}

				for _, registeredCmd := range registeredCmds {
					log.Info("registered command", slog.String("name", registeredCmd.Name))
				}
			} else {
				if guildID == "" {
					return fmt.Errorf("--guild is required when not using --global")
				}
				log.Info("registering commands for guild using bulk overwrite",
					slog.String("guild_id", guildID),
					slog.Int("command_count", len(commandDefs)))

				registeredCmds, err := session.ApplicationCommandBulkOverwrite(cfg.Bot.ApplicationID, guildID, commandDefs)
				if err != nil {
					return fmt.Errorf("failed to bulk register commands: %w", err)
				}

				for _, registeredCmd := range registeredCmds {
					log.Info("registered command", slog.String("name", registeredCmd.Name))
				}
			}

			log.Info("command registration complete - old commands automatically removed")
			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "configs/dev-bot.yaml", "path to configuration file")
	cmd.Flags().StringVar(&guildID, "guild", "", "guild ID for guild-specific commands (instant, for testing)")
	cmd.Flags().BoolVar(&global, "global", false, "register commands globally (slower, for production)")
	cmd.Flags().BoolVar(&cleanup, "cleanup", false, "remove all commands without registering new ones")

	return cmd
}
