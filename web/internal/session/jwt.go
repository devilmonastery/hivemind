package session

import (
	"errors"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	// ErrNoToken is returned when no token is found in the session
	ErrNoToken = errors.New("no token in session")

	// ErrInvalidToken is returned when the token cannot be parsed
	ErrInvalidToken = errors.New("invalid token")

	// ErrTokenExpired is returned when the token has expired
	ErrTokenExpired = errors.New("token expired")

	// ErrMissingUserID is returned when the token is missing the required user_id claim
	ErrMissingUserID = errors.New("token missing user_id claim")
)

// ParseUserClaims parses a JWT token and extracts user information
// Returns a map with user fields or an error if the token is invalid or expired
func ParseUserClaims(tokenString string) (map[string]interface{}, error) {
	if tokenString == "" {
		return nil, ErrNoToken
	}

	// Parse JWT without verification (since it's already verified by the backend)
	// We just need to extract the claims
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	// Check if token is expired first
	// exp claim is a NumericDate (Unix timestamp)
	if exp, ok := claims["exp"].(float64); ok {
		expTime := time.Unix(int64(exp), 0)
		if time.Now().After(expTime) {
			return nil, ErrTokenExpired
		}
	}

	// Extract user info from claims
	user := make(map[string]interface{})

	// Email can be in either "email" or "username" claim
	// (our JWT uses "username" for the email address)
	if email, ok := claims["email"].(string); ok {
		user["Email"] = email
	} else if username, ok := claims["username"].(string); ok {
		user["Email"] = username
	}

	if userID, ok := claims["user_id"].(string); ok {
		user["UserID"] = userID
	}

	if displayName, ok := claims["display_name"].(string); ok {
		user["DisplayName"] = displayName
	}

	if role, ok := claims["role"].(string); ok {
		user["Role"] = role
	}

	if picture, ok := claims["picture"].(string); ok {
		user["Picture"] = picture
	}

	if timezone, ok := claims["timezone"].(string); ok {
		user["Timezone"] = timezone
	}

	// Validate we have at least a user_id
	if _, hasUserID := user["UserID"]; !hasUserID {
		return nil, ErrMissingUserID
	}

	return user, nil
}

// IsTokenExpired checks if a JWT token is expired without extracting all claims
// Returns true if expired or if the token cannot be parsed
func IsTokenExpired(tokenString string) bool {
	if tokenString == "" {
		return true
	}

	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return true
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return true
	}

	if exp, ok := claims["exp"].(float64); ok {
		expTime := time.Unix(int64(exp), 0)
		return time.Now().After(expTime)
	}

	// No expiration claim means we can't determine - treat as not expired
	// (the backend will reject it if it's actually invalid)
	return false
}

// GetValidatedUser retrieves and validates the user from the session
// Returns nil if not authenticated, expired, or invalid
func (m *Manager) GetValidatedUser(r *http.Request) (map[string]interface{}, error) {
	tokenString, err := m.GetToken(r)
	if err != nil {
		if err == http.ErrNoCookie {
			return nil, ErrNoToken
		}
		return nil, err
	}

	return ParseUserClaims(tokenString)
}
