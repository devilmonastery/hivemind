package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/devilmonastery/hivemind/internal/config"
	"github.com/golang-jwt/jwt/v5"
)

// GenericOIDCProvider implements Provider interface using OIDC discovery
type GenericOIDCProvider struct {
	name           string
	discoveryCache *OIDCDiscoveryCache
	jwksCache      *JWKSCache
	issuer         string
}

// NewGenericOIDCProvider creates a new generic OIDC provider
func NewGenericOIDCProvider(name string, issuer string, discoveryCache *OIDCDiscoveryCache) *GenericOIDCProvider {
	return &GenericOIDCProvider{
		name:           name,
		issuer:         issuer,
		discoveryCache: discoveryCache,
		// JWKS cache will be initialized lazily when we know the JWKS URL
		jwksCache: nil,
	}
}

// Name returns the provider identifier
func (p *GenericOIDCProvider) Name() string {
	return p.name
}

// GetAuthorizationURL builds the authorization URL using discovered endpoints
func (p *GenericOIDCProvider) GetAuthorizationURL(clientID, redirectURI, state, codeChallenge string) string {
	// Return a template - the actual URL will be built from discovery
	return fmt.Sprintf(
		"{authorization_endpoint}?"+
			"client_id=%s&"+
			"redirect_uri={redirect_uri}&"+
			"response_type=code&"+
			"scope={scopes}&"+
			"state={state}&"+
			"code_challenge={code_challenge}&"+
			"code_challenge_method=S256&"+
			"prompt=consent",
		clientID,
	)
}

// fetchUserinfo fetches user information from the userinfo endpoint
func (p *GenericOIDCProvider) fetchUserinfo(ctx context.Context, discovery *OIDCDiscoveryDocument, accessToken string) (map[string]interface{}, error) {
	if discovery.UserinfoEndpoint == "" {
		return nil, fmt.Errorf("userinfo endpoint not available")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", discovery.UserinfoEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create userinfo request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch userinfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo endpoint returned status %d", resp.StatusCode)
	}

	var userinfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userinfo); err != nil {
		return nil, fmt.Errorf("failed to decode userinfo: %w", err)
	}

	return userinfo, nil
}

// ValidateIDToken validates an ID token using discovered JWKS endpoint
// If email is not in the ID token, it fetches it from the userinfo endpoint using the access token
func (p *GenericOIDCProvider) ValidateIDToken(ctx context.Context, idToken string, accessToken string, cfg config.ProviderConfig) (*Claims, error) {
	// Get discovery document to find JWKS URL
	discovery, err := p.discoveryCache.GetDiscovery(ctx, p.issuer)
	if err != nil {
		return nil, fmt.Errorf("failed to get discovery document: %w", err)
	}

	// Initialize JWKS cache if needed
	if p.jwksCache == nil {
		p.jwksCache = NewJWKSCache(discovery.JWKSURI, 1*time.Hour)
	}

	// Parse the token without verification first to get the kid (key ID)
	token, err := jwt.Parse(idToken, func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method is RS256
		if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Get the key ID from the token header
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("missing kid in token header")
		}

		// Fetch the public key from the JWKS endpoint
		publicKey, err := p.jwksCache.GetKey(ctx, kid)
		if err != nil {
			return nil, fmt.Errorf("failed to get public key: %w", err)
		}

		return publicKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	// Extract claims
	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims format")
	}

	// Validate issuer
	iss, _ := mapClaims["iss"].(string)
	if iss != p.issuer && iss != discovery.Issuer {
		return nil, fmt.Errorf("invalid issuer: %s (expected %s)", iss, p.issuer)
	}

	// Validate audience (client ID)
	// The aud claim can be a string or an array of strings per OIDC spec
	var aud string
	switch v := mapClaims["aud"].(type) {
	case string:
		aud = v
	case []interface{}:
		if len(v) > 0 {
			if audStr, ok := v[0].(string); ok {
				aud = audStr
			}
		}
	}

	// Only validate audience if it's present (some providers may not include it in all flows)
	if aud != "" && aud != cfg.ClientID {
		return nil, fmt.Errorf("invalid audience: %s (expected %s)", aud, cfg.ClientID)
	}

	// Debug: Log all claims to see what Discord is sending
	fmt.Printf("[DEBUG] ID Token claims for provider %s: %+v\n", p.name, mapClaims)

	// Extract required claims
	sub, _ := mapClaims["sub"].(string)
	if sub == "" {
		return nil, fmt.Errorf("missing sub claim")
	}

	// Email might not be in ID token - some providers (like Discord) only include it in userinfo endpoint
	// We'll try to get it from the ID token, but won't fail if it's missing
	email, _ := mapClaims["email"].(string)
	emailVerified, _ := mapClaims["email_verified"].(bool)

	// Extract optional claims with provider-specific logic (from ID token first)
	name := p.extractName(mapClaims)
	picture := p.extractPicture(mapClaims, sub)

	// If email is missing from ID token, fetch it from userinfo endpoint
	if email == "" && accessToken != "" && discovery.UserinfoEndpoint != "" {
		fmt.Printf("[DEBUG] Email not in ID token, fetching from userinfo endpoint for %s\n", p.name)
		userinfo, err := p.fetchUserinfo(ctx, discovery, accessToken)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch userinfo: %w", err)
		}

		fmt.Printf("[DEBUG] Userinfo response: %+v\n", userinfo)

		// Extract email from userinfo
		if userinfoEmail, ok := userinfo["email"].(string); ok {
			email = userinfoEmail
		}
		// Try both "email_verified" (standard OIDC) and "verified" (some providers)
		if verified, ok := userinfo["email_verified"].(bool); ok {
			emailVerified = verified
		} else if verified, ok := userinfo["verified"].(bool); ok {
			emailVerified = verified
		}

		// Also try to extract name and picture if not already present
		if name == "" {
			// Try various name fields in order of preference
			if userinfoName, ok := userinfo["name"].(string); ok && userinfoName != "" {
				name = userinfoName
			} else if userinfoName, ok := userinfo["global_name"].(string); ok && userinfoName != "" {
				name = userinfoName
			} else if userinfoName, ok := userinfo["preferred_username"].(string); ok && userinfoName != "" {
				name = userinfoName
			} else if userinfoName, ok := userinfo["nickname"].(string); ok && userinfoName != "" {
				name = userinfoName
			} else if userinfoName, ok := userinfo["username"].(string); ok && userinfoName != "" {
				name = userinfoName
			}
		}
		if picture == "" {
			// Try standard picture claim first
			if picUrl, ok := userinfo["picture"].(string); ok && picUrl != "" {
				picture = picUrl
			}
		}
	}

	// Email is required
	if email == "" {
		return nil, fmt.Errorf("email not available in ID token or userinfo")
	}

	// Extract timestamps
	iat, _ := mapClaims["iat"].(float64)
	exp, _ := mapClaims["exp"].(float64)

	claims := &Claims{
		Subject:       sub,
		Email:         email,
		EmailVerified: emailVerified,
		Name:          name,
		Picture:       picture,
		IssuedAt:      time.Unix(int64(iat), 0),
		ExpiresAt:     time.Unix(int64(exp), 0),
	}

	return claims, nil
}

// extractName extracts the display name from claims (provider-specific logic)
func (p *GenericOIDCProvider) extractName(claims jwt.MapClaims) string {
	// Try various name fields in order of preference
	if globalName, ok := claims["global_name"].(string); ok && globalName != "" {
		return globalName // Discord's preferred display name
	}
	if username, ok := claims["username"].(string); ok && username != "" {
		return username // Discord fallback
	}
	if name, ok := claims["name"].(string); ok && name != "" {
		return name // Standard OIDC claim (Google, etc.)
	}
	if preferredUsername, ok := claims["preferred_username"].(string); ok && preferredUsername != "" {
		return preferredUsername // Some providers use this
	}
	return ""
}

// extractPicture extracts the profile picture URL (provider-specific logic)
func (p *GenericOIDCProvider) extractPicture(claims jwt.MapClaims, sub string) string {
	// Standard OIDC picture claim
	if picture, ok := claims["picture"].(string); ok && picture != "" {
		return picture
	}
	return ""
}
