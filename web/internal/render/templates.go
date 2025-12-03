package render

import (
	"crypto/md5"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/devilmonastery/hivemind/internal/pkg/urlutil"
)

// Version is set at build time using ldflags
var Version = "dev"

// TemplateSet holds all parsed page templates
// Each page is stored as a completely separate template.Template
// to avoid {{define "content"}} block collisions
type TemplateSet struct {
	pages map[string]*template.Template
	mu    sync.RWMutex
}

// Execute renders the specified page template
// pageName should be the filename like "firehose.html"
// This method always executes the "base" layout, which will use the
// {{define "content"}}, {{define "title"}}, etc. blocks from the specific page
func (ts *TemplateSet) Execute(w io.Writer, pageName string, data interface{}) error {
	ts.mu.RLock()
	tmpl, ok := ts.pages[pageName]
	ts.mu.RUnlock()

	if !ok {
		return fmt.Errorf("template %q not found", pageName)
	}

	// Always execute "base" - it will use the blocks defined when this page was parsed
	// Each page's template set has its own isolated "content", "title", etc. definitions
	// that were parsed together with base+components, so there's no collision
	return tmpl.ExecuteTemplate(w, "base", data)
}

// ExecuteTemplate executes a named template (like "snippet-fragment") from a specific page's template set
// This is useful for rendering partial templates/fragments that are defined within a page
// pageName is the page file (e.g., "snippets.html") and templateName is the named template within it
func (ts *TemplateSet) ExecuteTemplate(w io.Writer, pageName string, templateName string, data interface{}) error {
	ts.mu.RLock()
	tmpl, ok := ts.pages[pageName]
	ts.mu.RUnlock()

	if !ok {
		return fmt.Errorf("page template %q not found", pageName)
	}

	// Execute the named template from this page's template set
	return tmpl.ExecuteTemplate(w, templateName, data)
}

// Has checks if a template exists
func (ts *TemplateSet) Has(pageName string) bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	_, ok := ts.pages[pageName]
	return ok
}

// Names returns all available template names
func (ts *TemplateSet) Names() []string {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	names := make([]string, 0, len(ts.pages))
	for name := range ts.pages {
		names = append(names, name)
	}
	return names
}

// LoadTemplates parses and loads all HTML templates with custom functions
// If path is empty, defaults to "web/templates"
// Returns a TemplateSet where each page is completely isolated
func LoadTemplates(path string) (*TemplateSet, error) {
	if path == "" {
		path = "web/templates"
	}

	funcMap := template.FuncMap{
		"renderMarkdown": Markdown,
		"formatDate": func(t interface{}) string {
			// Handle protobuf Timestamp
			if ts, ok := t.(interface{ AsTime() time.Time }); ok {
				return ts.AsTime().Format("2006-01-02 15:04")
			}
			// Handle time.Time
			if tt, ok := t.(time.Time); ok {
				return tt.Format("2006-01-02 15:04")
			}
			// Fallback
			return fmt.Sprintf("%v", t)
		},
		"formatRelative": func(t interface{}) string {
			// Handle protobuf Timestamp
			if ts, ok := t.(interface{ AsTime() time.Time }); ok {
				return ts.AsTime().Format("2006-01-02 15:04")
			}
			// Handle time.Time
			if tt, ok := t.(time.Time); ok {
				return tt.Format("2006-01-02 15:04")
			}
			// Fallback
			return fmt.Sprintf("%v", t)
		},
		"untilInt": func(n int) []int {
			result := make([]int, n)
			for i := 0; i < n; i++ {
				result[i] = i
			}
			return result
		},
		"intensityColor": func(intensity int) string {
			switch intensity {
			case 0:
				return "intensity-0"
			case 1:
				return "intensity-1"
			case 2:
				return "intensity-2"
			case 3:
				return "intensity-3"
			case 4:
				return "intensity-4"
			default:
				return "intensity-0"
			}
		},
		"beeVariant": func(date string) int {
			// Hash the date string to get a deterministic bee variant (1-6)
			hash := md5.Sum([]byte(date))
			// Use first byte of hash modulo 6, then add 1 to get 1-6 range
			return int(hash[0]%6) + 1
		},
		"list": func(items ...string) []string {
			return items
		},
		"dict": func(values ...interface{}) map[string]interface{} {
			if len(values)%2 != 0 {
				return nil
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil
				}
				dict[key] = values[i+1]
			}
			return dict
		},
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"initials": func(name string) string {
			if name == "" {
				return "?"
			}

			// Split on spaces and take first letter of each word
			words := strings.Fields(name)
			if len(words) == 0 {
				return "?"
			}

			var result strings.Builder
			for i, word := range words {
				if i >= 2 { // Maximum of 2 initials
					break
				}
				if len(word) > 0 {
					result.WriteString(strings.ToUpper(string(word[0])))
				}
			}

			if result.Len() == 0 {
				return "?"
			}

			return result.String()
		},
		"avatarColors": func(name string) string {
			if name == "" {
				return "from-gray-400 to-gray-600"
			}

			// Create deterministic hash from username
			hash := md5.Sum([]byte(strings.ToLower(name)))
			hashValue := int(hash[0])

			// Curated color palette for avatars (brighter gradients for dark theme)
			colors := []string{
				"from-blue-400 to-blue-500",
				"from-green-400 to-green-500",
				"from-purple-400 to-purple-500",
				"from-pink-400 to-pink-500",
				"from-indigo-400 to-indigo-500",
				"from-red-400 to-red-500",
				"from-yellow-400 to-yellow-500",
				"from-teal-400 to-teal-500",
				"from-orange-400 to-orange-500",
				"from-cyan-400 to-cyan-500",
				"from-emerald-400 to-emerald-500",
				"from-violet-400 to-violet-500",
				"from-rose-400 to-rose-500",
				"from-sky-400 to-sky-500",
				"from-lime-400 to-lime-500",
				"from-amber-400 to-amber-500",
			}

			// Select color deterministically based on hash
			return colors[hashValue%len(colors)]
		},
		"avatarURL": func(discordID, guildID, guildAvatarHash, userAvatarHash string) string {
			return urlutil.ConstructAvatarURL(discordID, guildID, guildAvatarHash, userAvatarHash, 128)
		},
		"mul": func(a, b int) int {
			return a * b
		},
		"div": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"assetURL": func(filename string) string {
			// Insert version into path for cache busting: /static/{version}/css/styles.css
			return "/static/" + Version + "/" + filename
		},
		"title": func(s string) string {
			if s == "" {
				return ""
			}
			// Simple title case: capitalize first letter and lowercase the rest
			return strings.ToUpper(string(s[0])) + strings.ToLower(s[1:])
		},
		"isImageURL": func(urlStr string) bool {
			// Parse URL to get clean path without query parameters
			u, err := url.Parse(urlStr)
			if err != nil {
				return false
			}
			lower := strings.ToLower(u.Path)
			return strings.HasSuffix(lower, ".jpg") ||
				strings.HasSuffix(lower, ".jpeg") ||
				strings.HasSuffix(lower, ".png") ||
				strings.HasSuffix(lower, ".gif") ||
				strings.HasSuffix(lower, ".webp") ||
				strings.HasSuffix(lower, ".bmp")
		},
		"isVideoURL": func(urlStr string) bool {
			// Parse URL to get clean path without query parameters
			u, err := url.Parse(urlStr)
			if err != nil {
				return false
			}
			lower := strings.ToLower(u.Path)
			return strings.HasSuffix(lower, ".mp4") ||
				strings.HasSuffix(lower, ".webm") ||
				strings.HasSuffix(lower, ".mov") ||
				strings.HasSuffix(lower, ".avi") ||
				strings.HasSuffix(lower, ".mkv")
		},
		"thumbnailURL": func(urlStr string) string {
			// Discord CDN supports width/height parameters for resizing
			if strings.Contains(urlStr, "cdn.discordapp.com") || strings.Contains(urlStr, "media.discordapp.net") {
				u, err := url.Parse(urlStr)
				if err != nil {
					return urlStr // fallback to original
				}
				q := u.Query()
				q.Set("width", "128")
				u.RawQuery = q.Encode()
				return u.String()
			}
			return urlStr
		},
		"fileExtension": func(urlStr string) string {
			u, err := url.Parse(urlStr)
			if err != nil {
				return "FILE"
			}
			ext := filepath.Ext(u.Path)
			if ext != "" {
				return strings.ToUpper(strings.TrimPrefix(ext, "."))
			}
			return "FILE"
		},
		"hasPrefix": func(s, prefix string) bool {
			return strings.HasPrefix(s, prefix)
		},
	}

	// Get file paths
	baseFile := filepath.Join(path, "layouts", "base.html")
	componentFiles, err := filepath.Glob(filepath.Join(path, "components", "*.html"))
	if err != nil {
		return nil, fmt.Errorf("failed to list component templates: %w", err)
	}

	pageFiles, err := filepath.Glob(filepath.Join(path, "pages", "*.html"))
	if err != nil {
		return nil, fmt.Errorf("failed to list page templates: %w", err)
	}

	if len(pageFiles) == 0 {
		return nil, fmt.Errorf("no page templates found in %s/pages", path)
	}

	// Get partial templates (HTMX content fragments)
	partialFiles, err := filepath.Glob(filepath.Join(path, "partials", "*.html"))
	if err != nil {
		return nil, fmt.Errorf("failed to list partial templates: %w", err)
	}

	// Create template set
	ts := &TemplateSet{
		pages: make(map[string]*template.Template),
	}

	// Parse each page into its OWN completely isolated template
	for _, pageFile := range pageFiles {
		pageName := filepath.Base(pageFile)

		// Build list of files: base + components + this page ONLY
		filesToParse := []string{baseFile}
		filesToParse = append(filesToParse, componentFiles...)
		filesToParse = append(filesToParse, pageFile)

		// Create a completely new, isolated template for this page
		// This template will have its own "content", "title", etc. definitions
		pageTemplate, err := template.New("base").Funcs(funcMap).ParseFiles(filesToParse...)
		if err != nil {
			return nil, fmt.Errorf("failed to parse template %s: %w", pageName, err)
		}

		ts.pages[pageName] = pageTemplate
	}

	// Parse each partial template (for HTMX content swaps)
	// Partials also get base + components so they can use the same template functions
	for _, partialFile := range partialFiles {
		partialName := filepath.Base(partialFile)

		// Build list of files: base + components + this partial ONLY
		filesToParse := []string{baseFile}
		filesToParse = append(filesToParse, componentFiles...)
		filesToParse = append(filesToParse, partialFile)

		// Create a completely new, isolated template for this partial
		partialTemplate, err := template.New("base").Funcs(funcMap).ParseFiles(filesToParse...)
		if err != nil {
			return nil, fmt.Errorf("failed to parse partial template %s: %w", partialName, err)
		}

		ts.pages[partialName] = partialTemplate
	}

	return ts, nil
}

// LogTemplateNames logs all available template names
func LogTemplateNames(ts *TemplateSet, logger *slog.Logger) {
	logger.Info("loaded templates", slog.Int("count", len(ts.Names())))
	for _, name := range ts.Names() {
		logger.Debug("template", slog.String("name", name))
	}
}

// GetTemplateNames returns a slice of all loaded template names
// Useful for testing and debugging
func GetTemplateNames(ts *TemplateSet) []string {
	return ts.Names()
}
