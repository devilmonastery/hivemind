package oidc

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"
)

// JWKSCache caches public keys from a JWKS endpoint
type JWKSCache struct {
	url        string
	keys       map[string]interface{}
	lastFetch  time.Time
	cacheTTL   time.Duration
	httpClient *http.Client
}

// NewJWKSCache creates a new JWKS cache
func NewJWKSCache(url string, ttl time.Duration) *JWKSCache {
	return &JWKSCache{
		url:      url,
		keys:     make(map[string]interface{}),
		cacheTTL: ttl,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetKey retrieves a public key by key ID
func (j *JWKSCache) GetKey(ctx context.Context, kid string) (interface{}, error) {
	// Check if we need to refresh the cache
	if time.Since(j.lastFetch) > j.cacheTTL || len(j.keys) == 0 {
		if err := j.refresh(ctx); err != nil {
			return nil, err
		}
	}

	key, ok := j.keys[kid]
	if !ok {
		// Try refreshing once more in case the key was just rotated
		if err := j.refresh(ctx); err != nil {
			return nil, err
		}
		key, ok = j.keys[kid]
		if !ok {
			return nil, fmt.Errorf("key not found: %s", kid)
		}
	}

	return key, nil
}

// refresh fetches the latest JWKS from the provider
func (j *JWKSCache) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", j.url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := j.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var jwks struct {
		Keys []json.RawMessage `json:"keys"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("failed to decode JWKS: %w", err)
	}

	// Parse each key
	newKeys := make(map[string]interface{})
	for _, keyData := range jwks.Keys {
		// Parse the JWK
		var keyInfo struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			Alg string `json:"alg"`
			Use string `json:"use"`
			N   string `json:"n"`
			E   string `json:"e"`
		}

		if err := json.Unmarshal(keyData, &keyInfo); err != nil {
			continue
		}

		// Only process RSA keys
		if keyInfo.Kty != "RSA" {
			continue
		}

		// Decode N (modulus) and E (exponent) from base64url
		nBytes, err := base64.RawURLEncoding.DecodeString(keyInfo.N)
		if err != nil {
			continue
		}

		eBytes, err := base64.RawURLEncoding.DecodeString(keyInfo.E)
		if err != nil {
			continue
		}

		// Convert E to int
		var eInt int
		for _, b := range eBytes {
			eInt = eInt<<8 + int(b)
		}

		// Create RSA public key
		pubKey := &rsa.PublicKey{
			N: new(big.Int).SetBytes(nBytes),
			E: eInt,
		}

		newKeys[keyInfo.Kid] = pubKey
	}

	if len(newKeys) == 0 {
		return fmt.Errorf("no valid keys found in JWKS")
	}

	j.keys = newKeys
	j.lastFetch = time.Now()

	return nil
}
