package commands

import "github.com/bwmarrin/discordgo"

// GetDefinitions returns all slash command definitions
func GetDefinitions() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name:        "ping",
			Description: "Check if the bot is alive",
		},
		{
			Name:        "wiki",
			Description: "Manage wiki pages",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "search",
					Description: "Search for wiki pages",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "query",
							Description: "Search query",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "view",
					Description: "View a wiki page",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "slug",
							Description: "Wiki page slug",
							Required:    true,
						},
					},
				},
			},
		},
		{
			Name:        "note",
			Description: "Manage personal notes",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "create",
					Description: "Create a new note",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "list",
					Description: "List your notes",
				},
			},
		},
		{
			Name:        "quote",
			Description: "Manage quotes",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "add",
					Description: "Add a new quote",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "random",
					Description: "Get a random quote",
				},
			},
		},
	}
}
