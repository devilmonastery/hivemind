package auth

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/devilmonastery/hivemind/internal/domain/repositories"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
)

// UserContext contains authenticated user information
type UserContext struct {
	UserID      string
	Username    string
	DisplayName string
	Picture     string
	Timezone    string
	Role        string
	TokenID     string
}

// contextKey is the key for storing user info in context
type contextKey string

const userContextKey contextKey = "user"

// GetUserFromContext extracts the authenticated user from the context
func GetUserFromContext(ctx context.Context) (*UserContext, error) {
	user, ok := ctx.Value(userContextKey).(*UserContext)
	if !ok || user == nil {
		return nil, status.Error(codes.Unauthenticated, "no authenticated user in context")
	}
	return user, nil
}

// SetUserInContext stores the authenticated user in the context
func SetUserInContext(ctx context.Context, user *UserContext) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// CanReadSnippet checks if the user can read a snippet for the given user ID
// Anyone can read any snippet
func CanReadSnippet(ctx context.Context, targetUserID string) error {
	_, err := GetUserFromContext(ctx)
	if err != nil {
		return err
	}
	// Anyone authenticated can read any snippet
	return nil
}

// CanWriteSnippet checks if the user can write/edit a snippet for the given user ID
// Users can only write their own notes, unless they're an admin
func CanWriteSnippet(ctx context.Context, targetUserID string) error {
	user, err := GetUserFromContext(ctx)
	if err != nil {
		return err
	}

	// Admins can write any snippet
	if user.Role == "admin" {
		return nil
	}

	// Users can only write their own notes
	if user.UserID != targetUserID {
		return status.Errorf(codes.PermissionDenied, "you can only modify your own notes")
	}

	return nil
}

// RequireAdmin checks if the user is an admin
func RequireAdmin(ctx context.Context) error {
	user, err := GetUserFromContext(ctx)
	if err != nil {
		return err
	}

	if user.Role != "admin" {
		return status.Error(codes.PermissionDenied, "admin access required")
	}

	return nil
}

// CanAccessGuildContent checks if the authenticated user can access content from a specific guild
// Users must be members of the guild to access guild-restricted content (wikis, quotes, notes)
// Admin users can access any guild content regardless of membership
func CanAccessGuildContent(
	ctx context.Context,
	guildMemberRepo repositories.GuildMemberRepository,
	discordUserRepo repositories.DiscordUserRepository,
	guildID string,
) error {
	user, err := GetUserFromContext(ctx)
	if err != nil {
		return err
	}

	// Admins can access any guild content
	if user.Role == "admin" {
		return nil
	}

	// Get user's Discord ID
	discordUser, err := discordUserRepo.GetByUserID(ctx, user.UserID)
	if err != nil {
		return status.Errorf(codes.PermissionDenied, "Discord account not linked - you must link your Discord account to access guild content")
	}

	// Check guild membership
	isMember, err := guildMemberRepo.IsMember(ctx, guildID, discordUser.DiscordID)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to check guild membership: %v", err)
	}

	if !isMember {
		return status.Error(codes.PermissionDenied, "you must be a member of this Discord server to access its content")
	}

	return nil
}
