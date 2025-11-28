package oidc

import (
	"context"
	"fmt"
	"time"

	"github.com/devilmonastery/hivemind/internal/config"
)

// Global discovery cache shared by all providers
var globalDiscoveryCache = NewOIDCDiscoveryCache(24 * time.Hour)

// Provider defines the interface for OIDC identity providers
// Each provider (Google, GitHub, Okta, etc.) implements this interface
type Provider interface {
	// Name returns the provider identifier (e.g., "google", "github")
	Name() string

	// ValidateIDToken validates an ID token and extracts claims
	// accessToken is optional and used if the provider requires userinfo endpoint
	// Returns the extracted claims or an error if validation fails
	ValidateIDToken(ctx context.Context, idToken string, accessToken string, cfg config.ProviderConfig) (*Claims, error)

	// GetAuthorizationURL builds the OAuth authorization URL for the provider
	// Parameters: clientID, redirectURI, state (CSRF token), codeChallenge (PKCE)
	// Returns the full authorization URL to redirect the user to
	GetAuthorizationURL(clientID, redirectURI, state, codeChallenge string) string
}

// Registry holds all registered OIDC providers
type Registry struct {
	providers map[string]Provider
}

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider to the registry
func (r *Registry) Register(provider Provider) {
	r.providers[provider.Name()] = provider
}

// Get retrieves a provider by name
func (r *Registry) Get(name string) (Provider, error) {
	provider, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
	return provider, nil
}

// List returns all registered provider names
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// DefaultRegistry is the global provider registry
var DefaultRegistry = NewRegistry()

// RegisterProvider is a convenience function to register a provider in the default registry
func RegisterProvider(provider Provider) {
	DefaultRegistry.Register(provider)
}

// GetProvider is a convenience function to get a provider from the default registry
func GetProvider(name string) (Provider, error) {
	return DefaultRegistry.Get(name)
}

// InitializeProviders sets up providers based on configuration
// This should be called at application startup with the auth config
func InitializeProviders(providers []config.ProviderConfig) error {
	for _, providerCfg := range providers {
		if providerCfg.Issuer == "" {
			return fmt.Errorf("provider %s: issuer is required for OIDC discovery", providerCfg.Name)
		}

		provider := NewGenericOIDCProvider(providerCfg.Name, providerCfg.Issuer, globalDiscoveryCache)
		RegisterProvider(provider)
	}
	return nil
}

// GetDiscoveryForProvider fetches the OIDC discovery document for an issuer
func GetDiscoveryForProvider(ctx context.Context, issuer string) (*OIDCDiscoveryDocument, error) {
	return globalDiscoveryCache.GetDiscovery(ctx, issuer)
}
