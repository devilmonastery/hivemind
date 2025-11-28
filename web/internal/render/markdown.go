package render

import (
	"html/template"

	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
)

// Markdown converts markdown text to safe HTML for use in templates
func Markdown(markdown string) template.HTML {
	// Convert markdown to HTML
	unsafe := blackfriday.Run([]byte(markdown))

	// Sanitize the HTML to prevent XSS
	policy := bluemonday.UGCPolicy()
	safe := policy.SanitizeBytes(unsafe)

	return template.HTML(safe)
}
