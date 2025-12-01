package textutil

import (
	"regexp"
	"sort"
	"strings"
)

// hashtagRegex matches hashtags with alphanumeric characters, underscores, and hyphens
// Supports formats like: #tag, #my-tag, #tag_name, #tag123
var hashtagRegex = regexp.MustCompile(`#([\w-]+)`)

// ExtractHashtags parses hashtags from text content
// Returns a sorted list of unique tags
// Supported formats: #word, #word-with-hyphens, #word_with_underscores, #word123
func ExtractHashtags(text string) []string {
	// Find all hashtags
	matches := hashtagRegex.FindAllStringSubmatch(text, -1)

	// Use a map to deduplicate tags
	tagMap := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			tag := strings.ToLower(match[1])
			tagMap[tag] = true
		}
	}

	// Convert map to slice
	tags := make([]string, 0, len(tagMap))
	for tag := range tagMap {
		tags = append(tags, tag)
	}

	// Sort for consistent ordering
	sort.Strings(tags)

	return tags
}
