package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := newRootCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hivemind-bot",
		Short: "Hivemind Discord bot for collaborative knowledge base",
		Long: `Hivemind Discord bot provides slash commands for managing wikis, notes, and quotes
directly within Discord guilds.`,
	}

	cmd.AddCommand(newRunCommand())
	cmd.AddCommand(newRegisterCommand())

	return cmd
}
