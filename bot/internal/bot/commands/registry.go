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
							Type:         discordgo.ApplicationCommandOptionString,
							Name:         "title",
							Description:  "Wiki page title",
							Required:     true,
							Autocomplete: true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "edit",
					Description: "Create or edit a wiki page",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:         discordgo.ApplicationCommandOptionString,
							Name:         "title",
							Description:  "Wiki page title (leave empty for new page)",
							Required:     false,
							Autocomplete: true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "merge",
					Description: "Merge two wiki pages",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:         discordgo.ApplicationCommandOptionString,
							Name:         "source",
							Description:  "Source page to merge from (will be deleted)",
							Required:     true,
							Autocomplete: true,
						},
						{
							Type:         discordgo.ApplicationCommandOptionString,
							Name:         "target",
							Description:  "Target page to merge into (will keep this page)",
							Required:     true,
							Autocomplete: true,
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
					Name:        "view",
					Description: "View a specific note",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:         discordgo.ApplicationCommandOptionString,
							Name:         "title",
							Description:  "Note title (partial match supported)",
							Required:     true,
							Autocomplete: true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "search",
					Description: "Search your notes",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "query",
							Description: "Search query",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionBoolean,
							Name:        "guild",
							Description: "Only search notes from this guild",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "tags",
							Description: "Filter by tags (comma-separated)",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "limit",
							Description: "Maximum number of notes to return",
							Required:    false,
						},
					},
				},
			},
		},
		{
			Name:        "quote",
			Description: "Manage guild quotes",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "random",
					Description: "Get a random quote from this guild",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "tags",
							Description: "Filter by tags (comma-separated, optional)",
							Required:    false,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "search",
					Description: "Search quotes in this guild",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "query",
							Description: "Search query",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "tags",
							Description: "Filter by tags (comma-separated, optional)",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "limit",
							Description: "Maximum number of quotes to return",
							Required:    false,
						},
					},
				},
			},
		},
		// Message context menu commands (right-click on messages)
		{
			Name: "Save as Quote",
			Type: discordgo.MessageApplicationCommand,
		},
		{
			Name: "Create Note",
			Type: discordgo.MessageApplicationCommand,
		},
		{
			Name: "Add to Wiki",
			Type: discordgo.MessageApplicationCommand,
		},
		// User context menu commands (right-click on users)
		{
			Name: "Edit Note for User",
			Type: discordgo.UserApplicationCommand,
		},
		{
			Name: "View Note for User",
			Type: discordgo.UserApplicationCommand,
		},
	}
}
