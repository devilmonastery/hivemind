package handlers

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonpb "github.com/devilmonastery/hivemind/api/generated/go/commonpb"
	tokenspb "github.com/devilmonastery/hivemind/api/generated/go/tokenspb"
	"github.com/devilmonastery/hivemind/internal/domain/entities"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
	"github.com/devilmonastery/hivemind/internal/domain/services"
)

// TokenHandler handles token gRPC requests
type TokenHandler struct {
	tokenspb.UnimplementedTokenServiceServer
	tokenService *services.TokenService
}

// NewTokenHandler creates a new token handler
func NewTokenHandler(tokenService *services.TokenService) *TokenHandler {
	return &TokenHandler{
		tokenService: tokenService,
	}
}

// CreateAPIToken creates a new API token for the authenticated user
func (h *TokenHandler) CreateAPIToken(ctx context.Context, req *tokenspb.CreateAPITokenRequest) (*tokenspb.CreateAPITokenResponse, error) {
	// TODO: Extract user ID from authentication context
	// For now, use a test user ID
	userID := "test_user_1"

	if req.DeviceName == "" {
		return nil, status.Error(codes.InvalidArgument, "device_name is required")
	}

	// Default scopes if none provided
	scopes := req.Scopes
	if len(scopes) == 0 {
		scopes = []string{"hivemind:read", "hivemind:write"}
	}

	// Default expiration to 90 days if not provided
	expiresInDays := req.ExpiresInDays
	if expiresInDays <= 0 {
		expiresInDays = 90
	}
	expiresIn := time.Duration(expiresInDays) * 24 * time.Hour

	// Create token
	token, plainToken, err := h.tokenService.CreateToken(
		ctx,
		userID,
		req.DeviceName,
		scopes,
		expiresIn,
		"api_create", // created by API call
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create token: %v", err)
	}

	// Convert to proto
	protoToken, err := tokenToProto(token)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert token: %v", err)
	}

	return &tokenspb.CreateAPITokenResponse{
		ApiToken:  plainToken,
		TokenInfo: protoToken,
	}, nil
}

// ListAPITokens lists API tokens for the authenticated user
func (h *TokenHandler) ListAPITokens(ctx context.Context, req *tokenspb.ListAPITokensRequest) (*tokenspb.ListAPITokensResponse, error) {
	// TODO: Extract user ID from authentication context
	userID := "test_user_1"

	// Set pagination defaults
	limit := int(req.PageSize)
	if limit <= 0 {
		limit = 50
	}

	opts := repositories.ListTokensOptions{
		Limit:     limit,
		Offset:    0, // TODO: Parse page_token for offset
		SortBy:    "created_at",
		SortOrder: "desc",
	}

	tokens, _, err := h.tokenService.ListUserTokens(ctx, userID, opts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list tokens: %v", err)
	}

	// Filter out revoked tokens if not requested
	if !req.IncludeRevoked {
		filteredTokens := make([]*entities.APIToken, 0, len(tokens))
		for _, token := range tokens {
			if !token.IsRevoked() {
				filteredTokens = append(filteredTokens, token)
			}
		}
		tokens = filteredTokens
	}

	// Convert to proto
	protoTokens := make([]*commonpb.APIToken, len(tokens))
	for i, token := range tokens {
		protoToken, err := tokenToProto(token)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to convert token: %v", err)
		}
		protoTokens[i] = protoToken
	}

	return &tokenspb.ListAPITokensResponse{
		Tokens:        protoTokens,
		NextPageToken: "", // TODO: Implement proper pagination
	}, nil
}

// GetAPIToken gets details of a specific API token
func (h *TokenHandler) GetAPIToken(ctx context.Context, req *tokenspb.GetAPITokenRequest) (*tokenspb.GetAPITokenResponse, error) {
	if req.TokenId == "" {
		return nil, status.Error(codes.InvalidArgument, "token_id is required")
	}

	token, err := h.tokenService.GetTokenByID(ctx, req.TokenId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get token: %v", err)
	}

	if token == nil {
		return nil, status.Error(codes.NotFound, "token not found")
	}

	// TODO: Verify user owns this token or is admin
	// For now, allow access to any token

	protoToken, err := tokenToProto(token)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert token: %v", err)
	}

	return &tokenspb.GetAPITokenResponse{
		Token: protoToken,
	}, nil
}

// UpdateAPIToken updates an API token (rename device, change scopes)
func (h *TokenHandler) UpdateAPIToken(ctx context.Context, req *tokenspb.UpdateAPITokenRequest) (*tokenspb.UpdateAPITokenResponse, error) {
	if req.TokenId == "" {
		return nil, status.Error(codes.InvalidArgument, "token_id is required")
	}

	token, err := h.tokenService.GetTokenByID(ctx, req.TokenId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get token: %v", err)
	}

	if token == nil {
		return nil, status.Error(codes.NotFound, "token not found")
	}

	// TODO: Verify user owns this token or is admin

	// Update fields if provided
	if req.DeviceName != "" {
		token.DeviceName = req.DeviceName
	}
	if len(req.Scopes) > 0 {
		token.Scopes = req.Scopes
	}

	// Update in database
	if err := h.tokenService.UpdateToken(ctx, token); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update token: %v", err)
	}

	protoToken, err := tokenToProto(token)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert token: %v", err)
	}

	return &tokenspb.UpdateAPITokenResponse{
		Token: protoToken,
	}, nil
}

// RevokeAPIToken revokes an API token
func (h *TokenHandler) RevokeAPIToken(ctx context.Context, req *tokenspb.RevokeAPITokenRequest) (*emptypb.Empty, error) {
	if req.TokenId == "" {
		return nil, status.Error(codes.InvalidArgument, "token_id is required")
	}

	// TODO: Extract user ID from authentication context
	revokedBy := "api_user" // placeholder

	if err := h.tokenService.RevokeToken(ctx, req.TokenId, revokedBy); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to revoke token: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// ValidateToken validates a token (internal use)
func (h *TokenHandler) ValidateToken(ctx context.Context, req *tokenspb.ValidateTokenRequest) (*tokenspb.ValidateTokenResponse, error) {
	if req.Token == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}

	// TODO: Parse required scope from request
	var requiredScope *entities.TokenScope

	user, token, err := h.tokenService.ValidateToken(ctx, req.Token, requiredScope, nil, nil)
	if err != nil {
		return &tokenspb.ValidateTokenResponse{
			Valid: false,
		}, nil
	}

	// Convert token to proto
	protoToken, err := tokenToProto(token)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert token: %v", err)
	}

	return &tokenspb.ValidateTokenResponse{
		Valid:         true,
		UserId:        user.ID,
		GrantedScopes: token.Scopes,
		ExpiresAt:     timestamppb.New(token.ExpiresAt),
		TokenInfo:     protoToken,
	}, nil
}

// tokenToProto converts a domain APIToken entity to a protobuf APIToken
func tokenToProto(token *entities.APIToken) (*commonpb.APIToken, error) {
	protoToken := &commonpb.APIToken{
		TokenId:    token.ID,
		UserId:     token.UserID,
		DeviceName: token.DeviceName,
		Scopes:     token.Scopes,
		ExpiresAt:  timestamppb.New(token.ExpiresAt),
		CreatedAt:  timestamppb.New(token.CreatedAt),
		Revoked:    token.IsRevoked(),
	}

	if token.LastUsed != nil {
		protoToken.LastUsed = timestamppb.New(*token.LastUsed)
	}

	if token.RevokedAt != nil {
		protoToken.RevokedAt = timestamppb.New(*token.RevokedAt)
	}

	return protoToken, nil
}
