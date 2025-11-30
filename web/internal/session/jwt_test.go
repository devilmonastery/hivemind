package session

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func createTestToken(claims jwt.MapClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	// We don't sign it since ParseUnverified doesn't check signatures
	tokenString, _ := token.SigningString()
	// Add a fake signature to make it a valid JWT structure
	return tokenString + ".fake_signature"
}

func TestParseUserClaims_ValidToken(t *testing.T) {
	claims := jwt.MapClaims{
		"user_id":      "123",
		"email":        "test@example.com",
		"display_name": "Test User",
		"role":         "user",
		"exp":          float64(time.Now().Add(1 * time.Hour).Unix()),
	}

	tokenString := createTestToken(claims)
	user, err := ParseUserClaims(tokenString)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if user["UserID"] != "123" {
		t.Errorf("expected UserID=123, got %v", user["UserID"])
	}

	if user["Email"] != "test@example.com" {
		t.Errorf("expected Email=test@example.com, got %v", user["Email"])
	}
}

func TestParseUserClaims_ExpiredToken(t *testing.T) {
	claims := jwt.MapClaims{
		"user_id": "123",
		"email":   "test@example.com",
		"exp":     float64(time.Now().Add(-1 * time.Hour).Unix()),
	}

	tokenString := createTestToken(claims)
	_, err := ParseUserClaims(tokenString)

	if err != ErrTokenExpired {
		t.Errorf("expected ErrTokenExpired, got %v", err)
	}
}

func TestParseUserClaims_MissingUserID(t *testing.T) {
	claims := jwt.MapClaims{
		"email": "test@example.com",
		"exp":   float64(time.Now().Add(1 * time.Hour).Unix()),
	}

	tokenString := createTestToken(claims)
	_, err := ParseUserClaims(tokenString)

	if err != ErrMissingUserID {
		t.Errorf("expected ErrMissingUserID, got %v", err)
	}
}

func TestParseUserClaims_EmptyToken(t *testing.T) {
	_, err := ParseUserClaims("")

	if err != ErrNoToken {
		t.Errorf("expected ErrNoToken, got %v", err)
	}
}

func TestParseUserClaims_UsernameAsEmail(t *testing.T) {
	claims := jwt.MapClaims{
		"user_id":  "123",
		"username": "user@example.com",
		"exp":      float64(time.Now().Add(1 * time.Hour).Unix()),
	}

	tokenString := createTestToken(claims)
	user, err := ParseUserClaims(tokenString)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if user["Email"] != "user@example.com" {
		t.Errorf("expected Email=user@example.com, got %v", user["Email"])
	}
}

func TestIsTokenExpired_Valid(t *testing.T) {
	claims := jwt.MapClaims{
		"user_id": "123",
		"exp":     float64(time.Now().Add(1 * time.Hour).Unix()),
	}

	tokenString := createTestToken(claims)

	if IsTokenExpired(tokenString) {
		t.Error("expected token to not be expired")
	}
}

func TestIsTokenExpired_Expired(t *testing.T) {
	claims := jwt.MapClaims{
		"user_id": "123",
		"exp":     float64(time.Now().Add(-1 * time.Hour).Unix()),
	}

	tokenString := createTestToken(claims)

	if !IsTokenExpired(tokenString) {
		t.Error("expected token to be expired")
	}
}

func TestIsTokenExpired_EmptyToken(t *testing.T) {
	if !IsTokenExpired("") {
		t.Error("expected empty token to be treated as expired")
	}
}
