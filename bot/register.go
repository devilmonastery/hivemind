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
	)

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register slash commands with Discord",
		Long: `Register all slash commands with Discord API. 
Use --guild for testing (instant), or --global for production (takes up to 1 hour to propagate).
Automatically removes commands that are no longer in the registry.`,
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

			// Get command definitions
			commandDefs := commands.GetDefinitions()

			// Create a map of desired command names for quick lookup
			desiredCommands := make(map[string]bool)
			for _, cmd := range commandDefs {
				desiredCommands[cmd.Name] = true
			}

			// Fetch existing commands and delete ones not in registry
			if global {
				log.Info("fetching existing global commands")
				existingCmds, err := session.ApplicationCommands(cfg.Bot.ApplicationID, "")
				if err != nil {
					return fmt.Errorf("failed to fetch existing commands: %w", err)
				}
				for _, existingCmd := range existingCmds {
					if !desiredCommands[existingCmd.Name] {
						log.Info("deleting obsolete command", slog.String("name", existingCmd.Name))
						err := session.ApplicationCommandDelete(cfg.Bot.ApplicationID, "", existingCmd.ID)
						if err != nil {
							log.Error("failed to delete command", slog.String("name", existingCmd.Name), slog.String("error", err.Error()))
						}
					}
				}
			} else {
				if guildID == "" {
					return fmt.Errorf("--guild is required when not using --global")
				}
				log.Info("fetching existing guild commands", slog.String("guild_id", guildID))
				existingCmds, err := session.ApplicationCommands(cfg.Bot.ApplicationID, guildID)
				if err != nil {
					return fmt.Errorf("failed to fetch existing commands: %w", err)
				}
				for _, existingCmd := range existingCmds {
					if !desiredCommands[existingCmd.Name] {
						log.Info("deleting obsolete command", slog.String("name", existingCmd.Name))
						err := session.ApplicationCommandDelete(cfg.Bot.ApplicationID, guildID, existingCmd.ID)
						if err != nil {
							log.Error("failed to delete command", slog.String("name", existingCmd.Name), slog.String("error", err.Error()))
						}
					}
				}
			}

			if global {
				log.Info("registering commands globally (may take up to 1 hour)")
				for _, cmd := range commandDefs {
					_, err := session.ApplicationCommandCreate(cfg.Bot.ApplicationID, "", cmd)
					if err != nil {
						return fmt.Errorf("failed to register command %s: %w", cmd.Name, err)
					}
					log.Info("registered command", slog.String("name", cmd.Name))
				}
			} else {
				if guildID == "" {
					return fmt.Errorf("--guild is required when not using --global")
				}
				log.Info("registering commands for guild", slog.String("guild_id", guildID))
				for _, cmd := range commandDefs {
					_, err := session.ApplicationCommandCreate(cfg.Bot.ApplicationID, guildID, cmd)
					if err != nil {
						return fmt.Errorf("failed to register command %s: %w", cmd.Name, err)
					}
					log.Info("registered command", slog.String("name", cmd.Name), slog.String("guild", guildID))
				}
			}

			log.Info("command registration complete")
			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "configs/dev-bot.yaml", "path to configuration file")
	cmd.Flags().StringVar(&guildID, "guild", "", "guild ID for guild-specific commands (instant, for testing)")
	cmd.Flags().BoolVar(&global, "global", false, "register commands globally (slower, for production)")

	return cmd
}
