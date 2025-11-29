package handlers

import (
	"sync"
	"time"
)

// TitlesCacheEntry holds cached titles with expiration
type TitlesCacheEntry struct {
	Titles    []TitleSuggestion
	ExpiresAt time.Time
}

// TitleSuggestion represents a cached title
type TitleSuggestion struct {
	ID    string
	Title string
}

// TitlesCache manages autocomplete caches with thread-safe operations
type TitlesCache struct {
	wikiCache sync.Map // map[guildID]TitlesCacheEntry
	noteCache sync.Map // map[userID:guildID]TitlesCacheEntry
	ttl       time.Duration
}

// NewTitlesCache creates a new titles cache
func NewTitlesCache(ttl time.Duration) *TitlesCache {
	return &TitlesCache{
		ttl: ttl,
	}
}

// GetWikiTitles returns cached wiki titles or nil if cache miss/expired
func (c *TitlesCache) GetWikiTitles(guildID string) []TitleSuggestion {
	val, ok := c.wikiCache.Load(guildID)
	if !ok {
		return nil
	}

	entry := val.(TitlesCacheEntry)
	if time.Now().After(entry.ExpiresAt) {
		c.wikiCache.Delete(guildID)
		return nil
	}

	return entry.Titles
}

// SetWikiTitles caches wiki titles for a guild
func (c *TitlesCache) SetWikiTitles(guildID string, titles []TitleSuggestion) {
	c.wikiCache.Store(guildID, TitlesCacheEntry{
		Titles:    titles,
		ExpiresAt: time.Now().Add(c.ttl),
	})
}

// InvalidateWikiTitles removes cached wiki titles for a guild
func (c *TitlesCache) InvalidateWikiTitles(guildID string) {
	c.wikiCache.Delete(guildID)
}

// GetNoteTitles returns cached note titles or nil if cache miss/expired
func (c *TitlesCache) GetNoteTitles(userID, guildID string) []TitleSuggestion {
	key := userID + ":" + guildID
	val, ok := c.noteCache.Load(key)
	if !ok {
		return nil
	}

	entry := val.(TitlesCacheEntry)
	if time.Now().After(entry.ExpiresAt) {
		c.noteCache.Delete(key)
		return nil
	}

	return entry.Titles
}

// SetNoteTitles caches note titles for a user in a guild
func (c *TitlesCache) SetNoteTitles(userID, guildID string, titles []TitleSuggestion) {
	key := userID + ":" + guildID
	c.noteCache.Store(key, TitlesCacheEntry{
		Titles:    titles,
		ExpiresAt: time.Now().Add(c.ttl),
	})
}

// InvalidateNoteTitles removes cached note titles for a user in a guild
func (c *TitlesCache) InvalidateNoteTitles(userID, guildID string) {
	key := userID + ":" + guildID
	c.noteCache.Delete(key)
}

// FilterTitles filters cached titles by query (case-insensitive substring match)
func FilterTitles(titles []TitleSuggestion, query string, limit int) []TitleSuggestion {
	if len(titles) == 0 {
		return nil
	}

	// Normalize query
	queryLower := toLowerString(query)

	// Filter matching titles
	filtered := make([]TitleSuggestion, 0, limit)
	for _, title := range titles {
		if len(filtered) >= limit {
			break
		}
		if containsSubstring(toLowerString(title.Title), queryLower) {
			filtered = append(filtered, title)
		}
	}

	return filtered
}

// containsSubstring checks if s contains query (both already lowercase)
func containsSubstring(s, query string) bool {
	if query == "" {
		return true
	}
	// Simple substring search
	for i := 0; i <= len(s)-len(query); i++ {
		if s[i:i+len(query)] == query {
			return true
		}
	}
	return false
}

// toLowerString converts string to lowercase
func toLowerString(s string) string {
	result := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			result = append(result, r+32)
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}
