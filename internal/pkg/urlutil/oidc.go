package urlutil

import (
	"fmt"
	"strings"
)

// OIDCDiscoveryURL builds the OIDC discovery document URL for the given issuer.
// Returns a URL like: {issuer}/.well-known/openid-configuration
// Ensures no double slashes by trimming trailing slash from issuer.
func OIDCDiscoveryURL(issuer string) string {
	issuer = strings.TrimRight(issuer, "/")
	return fmt.Sprintf("%s/.well-known/openid-configuration", issuer)
}
