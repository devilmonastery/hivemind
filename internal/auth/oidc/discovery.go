package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// OIDCDiscoveryDocument represents the OIDC discovery document
type OIDCDiscoveryDocument struct {
	Issuer                 string   `json:"issuer"`
	AuthorizationEndpoint  string   `json:"authorization_endpoint"`
	TokenEndpoint          string   `json:"token_endpoint"`
	UserinfoEndpoint       string   `json:"userinfo_endpoint"`
	JWKSURI                string   `json:"jwks_uri"`
	ResponseTypesSupported []string `json:"response_types_supported"`
}

// cachedDiscovery holds a discovery document with its expiration time
type cachedDiscovery struct {
	doc       *OIDCDiscoveryDocument
	expiresAt time.Time
}

// OIDCDiscoveryCache caches OIDC discovery documents
type OIDCDiscoveryCache struct {
	cache map[string]*cachedDiscovery
	mu    sync.RWMutex
	ttl   time.Duration
}

// NewOIDCDiscoveryCache creates a new discovery cache with the specified TTL
func NewOIDCDiscoveryCache(ttl time.Duration) *OIDCDiscoveryCache {
	return &OIDCDiscoveryCache{
		cache: make(map[string]*cachedDiscovery),
		ttl:   ttl,
	}
}

// GetDiscovery fetches or retrieves from cache the OIDC discovery document
func (c *OIDCDiscoveryCache) GetDiscovery(ctx context.Context, issuer string) (*OIDCDiscoveryDocument, error) {
	// Check cache first (read lock)
	c.mu.RLock()
	cached, exists := c.cache[issuer]
	c.mu.RUnlock()

	if exists && time.Now().Before(cached.expiresAt) {
		return cached.doc, nil
	}

	// Cache miss or expired - fetch (write lock)
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock (another goroutine might have updated)
	cached, exists = c.cache[issuer]
	if exists && time.Now().Before(cached.expiresAt) {
		return cached.doc, nil
	}

	// Fetch discovery document
	discoveryURL := fmt.Sprintf("%s/.well-known/openid-configuration", issuer)
	req, err := http.NewRequestWithContext(ctx, "GET", discoveryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch discovery document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var doc OIDCDiscoveryDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("failed to decode discovery document: %w", err)
	}

	// Validate required fields
	if doc.Issuer == "" || doc.AuthorizationEndpoint == "" || doc.TokenEndpoint == "" || doc.JWKSURI == "" {
		return nil, fmt.Errorf("incomplete discovery document from %s", issuer)
	}

	// Cache it
	c.cache[issuer] = &cachedDiscovery{
		doc:       &doc,
		expiresAt: time.Now().Add(c.ttl),
	}

	return &doc, nil
}
