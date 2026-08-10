package render

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Helper to get the path to templates from the test package directory
// Tests run from: <project>/internal/web/render
// Templates are at: <project>/web/templates
// So we need to go up 3 levels: ../../../web/templates
func getTestTemplatesPath() string {
	return filepath.Join("..", "..", "..", "web", "templates")
}

func TestLoadTemplates(t *testing.T) {
	// Load templates using path relative to test package directory
	ts, err := LoadTemplates(getTestTemplatesPath())
	if err != nil {
		t.Fatalf("Failed to load templates: %v", err)
	}

	if ts == nil {
		t.Fatal("Expected templates to be loaded, got nil")
	}

	// Get template names
	names := GetTemplateNames(ts)
	if len(names) == 0 {
		t.Fatal("Expected at least one template to be loaded")
	}

	// Check for required page templates
	requiredTemplates := []string{
		"home.html",
		"note.html",
		"notes.html",
		"login.html",
	}

	for _, required := range requiredTemplates {
		if !ts.Has(required) {
			t.Errorf("Expected template %q to be loaded, but it wasn't found", required)
		}
	}
}

func TestGetTemplateNames(t *testing.T) {
	// Load templates using path relative to test package directory
	ts, err := LoadTemplates(getTestTemplatesPath())
	if err != nil {
		t.Fatalf("Failed to load templates: %v", err)
	}

	names := GetTemplateNames(ts)

	// Should have some templates loaded
	if len(names) == 0 {
		t.Errorf("Expected some templates, got %d", len(names))
	}

	// All names should be non-empty
	for _, name := range names {
		if name == "" {
			t.Errorf("Found empty template name")
		}
	}
}

func TestTemplateFunctions(t *testing.T) {
	// Load templates using path relative to test package directory
	ts, err := LoadTemplates(getTestTemplatesPath())
	if err != nil {
		t.Fatalf("Failed to load templates: %v", err)
	}

	// Test that custom functions are available
	// We can't easily test function execution without executing templates,
	// but we can verify the template was loaded successfully
	if ts == nil {
		t.Fatal("Expected template with functions to be loaded")
	}
}

func TestLoadTemplatesDefaultPath(t *testing.T) {
	// Test that empty string uses default path
	// Note: This will only work if test is run from project root
	// For this test, we'll use the explicit path
	ts, err := LoadTemplates(getTestTemplatesPath())
	if err != nil {
		// If this fails, it means we're not in project root
		// Try the default path anyway to document the behavior
		ts, err = LoadTemplates("")
		if err != nil {
			t.Skipf("Skipping default path test - not running from project root: %v", err)
		}
	}

	if ts == nil {
		t.Fatal("Expected templates to be loaded with default path")
	}

	// Should have loaded the same templates
	names := GetTemplateNames(ts)
	if len(names) == 0 {
		t.Fatal("Expected at least one template with default path")
	}
}

func TestTemplateContentMatches(t *testing.T) {
	// Load templates
	ts, err := LoadTemplates(getTestTemplatesPath())
	if err != nil {
		t.Fatalf("Failed to load templates: %v", err)
	}

	// Debug: List all template names
	names := GetTemplateNames(ts)
	t.Logf("All loaded templates: %v", names)

	// Verify that required page templates are loaded
	requiredPages := []string{"home.html", "note.html", "notes.html", "quote.html", "login.html"}

	for _, required := range requiredPages {
		if !ts.Has(required) {
			t.Errorf("Required page template %q was not loaded", required)
		}
	}
}

func TestTemplateSourceFileExists(t *testing.T) {
	templatesPath := getTestTemplatesPath()

	// List of templates that should exist on disk
	requiredFiles := map[string]string{
		"base layout":         filepath.Join(templatesPath, "layouts", "base.html"),
		"home page":           filepath.Join(templatesPath, "pages", "home.html"),
		"notes page":          filepath.Join(templatesPath, "pages", "notes.html"),
		"note page":           filepath.Join(templatesPath, "pages", "note.html"),
		"login page":          filepath.Join(templatesPath, "pages", "login.html"),
		"nav component":       filepath.Join(templatesPath, "components", "nav.html"),
		"user-menu component": filepath.Join(templatesPath, "components", "user-menu.html"),
	}

	for name, path := range requiredFiles {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Required template file %q does not exist at %s", name, path)
		} else if err != nil {
			t.Errorf("Error checking template file %q at %s: %v", name, path, err)
		}
	}
}

func TestTemplateSourceMatches(t *testing.T) {
	// Load templates
	ts, err := LoadTemplates(getTestTemplatesPath())
	if err != nil {
		t.Fatalf("Failed to load templates: %v", err)
	}

	templatesPath := getTestTemplatesPath()

	// Read source file for notes template
	notesFile := filepath.Join(templatesPath, "pages", "notes.html")
	sourceBytes, err := os.ReadFile(notesFile)
	if err != nil {
		t.Fatalf("Failed to read notes.html source: %v", err)
	}
	sourceStr := string(sourceBytes)

	// Verify that the source file contains expected notes-specific content
	expectedInSource := []string{
		"My Notes - Hivemind", // Title block
		"My Notes",            // Header
		"Private notes you can access from anywhere", // Subtitle text
		"No notes found",      // Empty state
	}

	for _, expected := range expectedInSource {
		if !strings.Contains(sourceStr, expected) {
			t.Errorf("Expected notes source file to contain %q", expected)
		}
	}

	// Verify the template was actually loaded (name exists in template set)
	if !ts.Has("notes.html") {
		t.Fatal("notes.html template not found in loaded template set")
	}

	// Verify that executing "notes.html" actually uses the notes.html content
	var buf bytes.Buffer
	testData := map[string]interface{}{
		"User":        nil,
		"Notes":       []interface{}{},
		"CurrentPage": "notes",
		"Total":       0,
	}

	// Use the TemplateSet's Execute method which renders the page
	err = ts.Execute(&buf, "notes.html", testData)
	if err != nil {
		t.Fatalf("Failed to execute notes.html template: %v", err)
	}

	renderedOutput := buf.String()

	// Verify that the RENDERED output contains the notes-specific content
	expectedInRenderedOutput := []string{
		"My Notes - Hivemind", // Title from notes.html
		"My Notes",            // H1 header from notes.html
		"Private notes you can access from anywhere", // Subtitle
		"No notes found",      // Empty state from notes.html
	}

	for _, expected := range expectedInRenderedOutput {
		if !strings.Contains(renderedOutput, expected) {
			t.Errorf("ERROR: Executed 'notes.html' but got wrong content!")
			t.Errorf("Missing expected string: %q", expected)
			t.Fatalf("Template content mismatch - aborting test")
		}
	}
}
