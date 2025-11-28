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
		"firehose.html",
		"view.html",
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
	requiredPages := []string{"firehose.html", "home.html", "snippets.html", "view.html", "login.html"} // Note: file names unchanged

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
		"firehose page":       filepath.Join(templatesPath, "pages", "firehose.html"),
		"notes page":       filepath.Join(templatesPath, "pages", "snippets.html"),
		"view page":           filepath.Join(templatesPath, "pages", "view.html"),
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

	// Read source file for firehose template
	firehoseFile := filepath.Join(templatesPath, "pages", "firehose.html")
	sourceBytes, err := os.ReadFile(firehoseFile)
	if err != nil {
		t.Fatalf("Failed to read firehose.html source: %v", err)
	}
	sourceStr := string(sourceBytes)

	// Verify that the source file contains expected firehose-specific content
	expectedInSource := []string{
		"Firehose - Hivemind",           // Title block
		"Recent Activity",               // Header
		"Recent notes from everyone", // Subtitle text
		"No recent activity",            // Empty state
	}

	for _, expected := range expectedInSource {
		if !strings.Contains(sourceStr, expected) {
			t.Errorf("Expected firehose source file to contain %q", expected)
		}
	}

	// Verify the template was actually loaded (name exists in template set)
	if !ts.Has("firehose.html") {
		t.Fatal("firehose.html template not found in loaded template set")
	}

	// NOW THE IMPORTANT PART: Verify that executing "firehose.html"
	// actually uses the firehose.html content, not some other template
	var buf bytes.Buffer
	testData := map[string]interface{}{
		"User":        nil,
		"Snippets":    []interface{}{}, // Note: struct field name unchanged
		"CurrentPage": "firehose",
	}

	// Use the TemplateSet's Execute method which renders the page
	err = ts.Execute(&buf, "firehose.html", testData)
	if err != nil {
		t.Fatalf("Failed to execute firehose.html template: %v", err)
	}

	renderedOutput := buf.String()

	// Verify that the RENDERED output contains the firehose-specific content
	// These strings should only appear if firehose.html was actually used
	expectedInRenderedOutput := []string{
		"Firehose - Hivemind",           // Title from firehose.html
		"Recent Activity",               // H2 header from firehose.html
		"Recent notes from everyone", // Subtitle specific to firehose
		"No recent activity",            // Empty state from firehose.html
	}

	for _, expected := range expectedInRenderedOutput {
		if !strings.Contains(renderedOutput, expected) {
			t.Errorf("ERROR: Executed 'firehose.html' but got wrong content!")
			t.Errorf("Missing expected string: %q", expected)
			t.Errorf("This means the template collision bug is still present.")

			// Also check if we got view.html content instead (the bug we're trying to catch)
			if strings.Contains(renderedOutput, "View Notes - Hivemind") {
				t.Errorf("DETECTED: Got 'View Notes - Hivemind' title instead of 'Firehose - Hivemind'")
				t.Errorf("This confirms that view.html definitions are overwriting firehose.html definitions")
			}

			t.Fatalf("Template content mismatch - aborting test")
		}
	}

	// Additional check: should NOT contain content from other pages
	unexpectedStrings := []string{
		"View Notes - Hivemind", // From view.html
		"Enter shareable link URL", // From view.html
	}

	for _, unexpected := range unexpectedStrings {
		if strings.Contains(renderedOutput, unexpected) {
			t.Errorf("ERROR: Rendered output contains content from wrong template: %q", unexpected)
			t.Errorf("This indicates template collision - firehose.html is using blocks from other templates")
		}
	}
}
