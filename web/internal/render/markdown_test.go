package render

import (
	"html/template"
	"strings"
	"testing"
)

func TestMarkdown(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		contains    []string // Strings that should appear in output
		notContains []string // Strings that should NOT appear in output
	}{
		{
			name:     "heading level 1",
			input:    "# Hello World",
			contains: []string{"<h1>", "Hello World", "</h1>"},
		},
		{
			name:     "heading level 2",
			input:    "## Hello World",
			contains: []string{"<h2>", "Hello World", "</h2>"},
		},
		{
			name:     "heading level 3",
			input:    "### Hello World",
			contains: []string{"<h3>", "Hello World", "</h3>"},
		},
		{
			name:     "bold text",
			input:    "This is **bold** text",
			contains: []string{"<strong>", "bold", "</strong>"},
		},
		{
			name:     "italic text",
			input:    "This is *italic* text",
			contains: []string{"<em>", "italic", "</em>"},
		},
		{
			name:     "unordered list",
			input:    "- Item 1\n- Item 2\n- Item 3",
			contains: []string{"<ul>", "<li>", "Item 1", "Item 2", "Item 3", "</li>", "</ul>"},
		},
		{
			name:     "ordered list",
			input:    "1. First\n2. Second\n3. Third",
			contains: []string{"<ol>", "<li>", "First", "Second", "Third", "</li>", "</ol>"},
		},
		{
			name:     "inline code",
			input:    "Use `code` here",
			contains: []string{"<code>", "code", "</code>"},
		},
		{
			name:     "code block",
			input:    "```\nfunction test() {\n  return true;\n}\n```",
			contains: []string{"<pre>", "<code>", "function test()", "</code>", "</pre>"},
		},
		{
			name:     "link",
			input:    "[Google](https://google.com)",
			contains: []string{"<a", "href=\"https://google.com\"", "Google", "</a>"},
		},
		{
			name:     "paragraph",
			input:    "This is a paragraph.\n\nThis is another paragraph.",
			contains: []string{"<p>", "This is a paragraph", "</p>", "This is another paragraph"},
		},
		{
			name:     "multiple elements",
			input:    "# Title\n\nThis is **bold** and this is *italic*.\n\n- List item 1\n- List item 2",
			contains: []string{"<h1>", "Title", "</h1>", "<strong>", "bold", "</strong>", "<em>", "italic", "</em>", "<ul>", "<li>", "List item 1", "List item 2"},
		},
		{
			name:        "XSS prevention - script tag",
			input:       "<script>alert('xss')</script>",
			notContains: []string{"<script>"},
		},
		{
			name:        "XSS prevention - onclick",
			input:       "<div onclick=\"alert('xss')\">Click me</div>",
			notContains: []string{"onclick"},
		},
		{
			name:  "realistic note example",
			input: "# Hivemind Service\n\n## What I did today\n\n- Implemented markdown rendering\n- Added **automatic token refresh**\n- Fixed the `/view` endpoint\n\nUsed `blackfriday` library for parsing.",
			contains: []string{
				"<h1>", "Hivemind Service", "</h1>",
				"<h2>", "What I did today", "</h2>",
				"<ul>", "<li>", "Implemented markdown rendering", "</li>",
				"<strong>", "automatic token refresh", "</strong>",
				"<code>", "blackfriday", "</code>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Markdown(tt.input)
			resultStr := string(result)

			// Check that expected strings are present
			for _, expected := range tt.contains {
				if !strings.Contains(resultStr, expected) {
					t.Errorf("Expected output to contain %q, but it didn't.\nInput: %q\nOutput: %q", expected, tt.input, resultStr)
				}
			}

			// Check that unwanted strings are NOT present
			for _, unwanted := range tt.notContains {
				if strings.Contains(resultStr, unwanted) {
					t.Errorf("Expected output NOT to contain %q, but it did.\nInput: %q\nOutput: %q", unwanted, tt.input, resultStr)
				}
			}
		})
	}
}

func TestMarkdownReturnsTemplateHTML(t *testing.T) {
	input := "# Test"
	result := Markdown(input)

	// Verify it returns template.HTML type (not just string)
	_, ok := interface{}(result).(template.HTML)
	if !ok {
		t.Errorf("Markdown should return template.HTML type")
	}
}

func TestMarkdownEmptyInput(t *testing.T) {
	result := Markdown("")
	resultStr := string(result)

	// Empty input should produce minimal output
	if len(resultStr) > 10 { // Some whitespace/wrapper tags might be added
		t.Errorf("Expected minimal output for empty input, got: %q", resultStr)
	}
}
