package handlers

import (
	"regexp"
	"sort"
	"strings"
)

var hashtagRegex = regexp.MustCompile(`#(\w+)`)

// extractHashtags parses hashtags from text content
// Returns a sorted list of unique tags
func extractHashtags(text string) []string {
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
