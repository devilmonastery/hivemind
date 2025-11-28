package handlers

import (
	"context"
	"strconv"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminpb "github.com/devilmonastery/hivemind/api/generated/go/adminpb"
	userpb "github.com/devilmonastery/hivemind/api/generated/go/userpb"
	"github.com/devilmonastery/hivemind/internal/domain/repositories"
	"github.com/devilmonastery/hivemind/internal/domain/services"
)

// AdminHandler handles admin gRPC requests
type AdminHandler struct {
	adminpb.UnimplementedAdminServiceServer
	userService *services.UserService
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(userService *services.UserService) *AdminHandler {
	return &AdminHandler{
		userService: userService,
	}
}

// timestampFromTime converts a time.Time to a protobuf timestamp
func timestampFromTime(t time.Time) *timestamppb.Timestamp {
	return timestamppb.New(t)
}

// GetUserDetails retrieves detailed information about a specific user
func (h *AdminHandler) GetUserDetails(ctx context.Context, req *adminpb.GetUserDetailsRequest) (*adminpb.GetUserDetailsResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	user, err := h.userService.GetUserByID(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user: %v", err)
	}

	if user == nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}

	// Convert domain user to proto user
	protoUser := &userpb.User{
		UserId:    user.ID,
		Email:     user.Email,
		Name:      user.DisplayName,
		Role:      userpb.Role(userpb.Role_value["ROLE_"+string(user.Role)]),
		UserType:  userpb.UserType(userpb.UserType_value["USER_TYPE_"+string(user.UserType)]),
		CreatedAt: timestampFromTime(user.CreatedAt),
		Disabled:  !user.IsActive,
	}

	if user.OIDCProvider != nil {
		protoUser.Provider = *user.OIDCProvider
	} else {
		protoUser.Provider = "local"
	}
	if user.LastLogin != nil {
		protoUser.LastSeen = timestampFromTime(*user.LastLogin)
	}

	return &adminpb.GetUserDetailsResponse{
		User: protoUser,
	}, nil
}

// ListAllUsers lists all users with pagination and filtering
func (h *AdminHandler) ListAllUsers(ctx context.Context, req *adminpb.ListAllUsersRequest) (*adminpb.ListAllUsersResponse, error) {
	// Set default pagination if not provided
	limit := int(req.PageSize)
	if limit <= 0 || limit > 100 {
		limit = 50 // default page size
	}

	offset := 0
	if req.PageToken != "" {
		if parsed, err := strconv.Atoi(req.PageToken); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Create list options
	opts := repositories.ListUsersOptions{
		Limit:     limit,
		Offset:    offset,
		SortBy:    "created_at",
		SortOrder: "desc",
	}

	users, total, err := h.userService.ListUsers(ctx, opts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list users: %v", err)
	}

	// Convert domain users to proto users
	protoUsers := make([]*userpb.User, len(users))
	for i, user := range users {
		protoUsers[i] = &userpb.User{
			UserId:    user.ID,
			Email:     user.Email,
			Name:      user.DisplayName,
			Role:      userpb.Role(userpb.Role_value["ROLE_"+string(user.Role)]),
			UserType:  userpb.UserType(userpb.UserType_value["USER_TYPE_"+string(user.UserType)]),
			CreatedAt: timestampFromTime(user.CreatedAt),
			Disabled:  !user.IsActive,
		}

		if user.OIDCProvider != nil {
			protoUsers[i].Provider = *user.OIDCProvider
		} else {
			protoUsers[i].Provider = "local"
		}
		if user.LastLogin != nil {
			protoUsers[i].LastSeen = timestampFromTime(*user.LastLogin)
		}
	}

	// Calculate next page token
	nextPageToken := ""
	if offset+limit < int(total) {
		nextPageToken = strconv.Itoa(offset + limit)
	}

	return &adminpb.ListAllUsersResponse{
		Users:         protoUsers,
		TotalCount:    int32(total),
		NextPageToken: nextPageToken,
	}, nil
}

// DeleteUser soft deletes a user (deactivates them)
func (h *AdminHandler) DeleteUser(ctx context.Context, req *adminpb.DeleteUserRequest) (*emptypb.Empty, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	// For now, use "system" as the deactivated_by user
	// TODO: Get actual admin user ID from context/token
	err := h.userService.DeactivateUser(ctx, req.UserId, "system")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to deactivate user: %v", err)
	}

	return &emptypb.Empty{}, nil
}
