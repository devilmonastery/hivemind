package cli

import (
	"fmt"
	"os"

	"github.com/charmbracelet/glamour"
	"golang.org/x/term"
)

// renderMarkdown renders markdown content, using glamour for terminal output or plain text otherwise
func renderMarkdown(markdown string, theme string) (string, error) {
	// If stdout is a terminal, render styled markdown using glamour
	if term.IsTerminal(int(os.Stdout.Fd())) {
		// Use glamour.Render with the style name directly
		rendered, err := glamour.Render(markdown, theme)
		if err != nil {
			// Fall back to plain markdown if rendering fails
			return markdown, nil
		}
		return rendered, nil
	}

	// For non-terminal output (pipes, redirects), return plain markdown
	return markdown, nil
}

// printMarkdown renders and prints markdown using the configured theme
func printMarkdown(markdown string) error {
	config, err := LoadConfig()
	if err != nil {
		// Fall back to auto theme if config loading fails
		config = DefaultConfig()
	}

	rendered, err := renderMarkdown(markdown, getTheme(config))
	if err != nil {
		return err
	}

	fmt.Print(rendered)
	return nil
}

// getTheme returns the theme from the current context, or "auto" if config is unavailable
func getTheme(config *Config) string {
	if config == nil {
		return "auto"
	}

	ctx, err := config.GetCurrentContext()
	if err != nil {
		return "auto"
	}

	return ctx.Rendering.Theme
}
