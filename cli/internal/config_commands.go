package cli

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration and contexts",
		Long:  `Manage CLI configuration including server contexts, similar to kubectl contexts.`,
	}

	// Add subcommands
	cmd.AddCommand(newCurrentContextCommand())
	cmd.AddCommand(newUseContextCommand())
	cmd.AddCommand(newListContextsCommand())
	cmd.AddCommand(newAddContextCommand())
	cmd.AddCommand(newDeleteContextCommand())
	cmd.AddCommand(newConfigShowCommand()) // Keep legacy show command for compatibility

	return cmd
}

// current-context command
func newCurrentContextCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "current-context",
		Short: "Display the current context",
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			fmt.Println(config.CurrentContext)
			return nil
		},
	}
}

// use-context command
func newUseContextCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "use-context CONTEXT_NAME",
		Short: "Switch to a different context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			contextName := args[0]

			config, err := LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if err := config.SetCurrentContext(contextName); err != nil {
				return err
			}

			if err := SaveConfig(config); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Printf("Switched to context %q\n", contextName)
			return nil
		},
	}
}

// list-contexts command
func newListContextsCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "list-contexts",
		Aliases: []string{"get-contexts"},
		Short:   "List all available contexts",
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if len(config.Contexts) == 0 {
				fmt.Println("No contexts configured")
				return nil
			}

			// Sort context names for consistent output
			names := make([]string, 0, len(config.Contexts))
			for name := range config.Contexts {
				names = append(names, name)
			}
			sort.Strings(names)

			// Use tabwriter for aligned output
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "CURRENT\tNAME\tSERVER\tTHEME")

			for _, name := range names {
				ctx := config.Contexts[name]
				current := " "
				if name == config.CurrentContext {
					current = "*"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					current,
					name,
					ctx.ServerAddress(),
					ctx.Rendering.Theme,
				)
			}
			w.Flush()

			return nil
		},
	}
}

// add-context command
func newAddContextCommand() *cobra.Command {
	var (
		address string
		port    int
		theme   string
	)

	cmd := &cobra.Command{
		Use:   "add-context CONTEXT_NAME",
		Short: "Add or update a context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			contextName := args[0]

			config, err := LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create new context
			ctx := &Context{}
			ctx.Server.Address = address
			ctx.Server.Port = port
			ctx.Rendering.Theme = theme

			// Add or update the context
			config.AddContext(contextName, ctx)

			// If this is the first context, make it current
			if len(config.Contexts) == 1 {
				config.CurrentContext = contextName
			}

			if err := SaveConfig(config); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Printf("Context %q added/updated\n", contextName)
			return nil
		},
	}

	cmd.Flags().StringVar(&address, "address", "localhost", "Server address")
	cmd.Flags().IntVar(&port, "port", 9091, "Server port")
	cmd.Flags().StringVar(&theme, "theme", "auto", "Rendering theme")
	cmd.MarkFlagRequired("address")

	return cmd
}

// delete-context command
func newDeleteContextCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete-context CONTEXT_NAME",
		Short: "Delete a context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			contextName := args[0]

			config, err := LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if err := config.DeleteContext(contextName); err != nil {
				return err
			}

			if err := SaveConfig(config); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Printf("Context %q deleted\n", contextName)
			return nil
		},
	}
}

// show command (legacy) - now shows current context
func newConfigShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current context configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			ctx, err := config.GetCurrentContext()
			if err != nil {
				return fmt.Errorf("failed to get current context: %w", err)
			}

			fmt.Printf("Current context: %s\n", config.CurrentContext)
			fmt.Printf("  Server Address: %s\n", ctx.Server.Address)
			fmt.Printf("  Server Port: %d\n", ctx.Server.Port)
			fmt.Printf("  Full Address: %s\n", ctx.ServerAddress())
			fmt.Printf("  Glamour Theme: %s\n", ctx.Rendering.Theme)

			configPath, _ := GetConfigPath()
			fmt.Printf("  Config File: %s\n", configPath)

			return nil
		},
	}
}
