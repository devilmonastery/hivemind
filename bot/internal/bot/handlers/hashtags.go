package handlers

import (
	"regexp"
	"sort"
	"strings"
)

// hashtagRegex matches hashtags with alphanumeric characters, underscores, and hyphens
// Supports formats like: #tag, #my-tag, #tag_name, #tag123
var hashtagRegex = regexp.MustCompile(`#([\w-]+)`)

// extractHashtags parses hashtags from text content
// Returns a sorted list of unique tags
// Supported formats: #word, #word-with-hyphens, #word_with_underscores, #word123
// Excludes Discord channel mentions in angle brackets (e.g., <#123456789>)
func extractHashtags(text string) []string {
	// Find all hashtags
	matches := hashtagRegex.FindAllStringSubmatchIndex(text, -1)

	// Use a map to deduplicate tags
	tagMap := make(map[string]bool)
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		// Check if this hashtag is part of a Discord channel mention (<#...>)
		// match[0] is the start of the full match (including #)
		// match[1] is the end of the full match
		if match[0] > 0 && text[match[0]-1] == '<' {
			// Check if there's a closing > after the match
			if match[1] < len(text) && text[match[1]] == '>' {
				continue // Skip Discord channel mentions
			}
		}

		tag := strings.ToLower(text[match[2]:match[3]])
		tagMap[tag] = true
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
