//go:build grpcserver

package grpcserver

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	userv1 "droneDeliveryManagement/api/user/v1"
	"droneDeliveryManagement/internal/auth"
	"droneDeliveryManagement/models"
	"droneDeliveryManagement/repository"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server bundles dependencies and implements the UserOrderService.
type Server struct {
	userv1.UnimplementedUserOrderServiceServer
	Users  *repository.UserRepository
	Orders *repository.OrderRepository
	Drones *repository.DroneRepository
}

const (
	maxPageSize          = 100 // Maximum allowed page size for list operations.
	defaultPageSize      = 20  // Default page size for list operations.
	cursorSeparator      = "|" // Separator for cursor components.
	sqliteDateFormat     = "2006-01-02 15:04:05"
	endUserOrAdminReason = "enduser or admin"
)

// Authentication helpers centralized in internal/auth.

// resolveCurrentUser retrieves the authenticated user from the database.
func (s *Server) resolveCurrentUser(ctx context.Context, p *auth.Principal) (*models.User, error) {
	u, err := s.Users.GetByUsername(ctx, p.Name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get user: %v", err)
	}
	if u == nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	return u, nil
}

// SetOrder creates a new order for the authenticated user.
func (s *Server) SetOrder(ctx context.Context, req *userv1.SetOrderRequest) (*userv1.SetOrderResponse, error) {
	p, err := auth.RequireEndUserOrAdmin(ctx)
	if err != nil {
		return nil, err
	}

	u, err := s.resolveCurrentUser(ctx, p)
	if err != nil {
		return nil, err
	}

	// Create order from request.
	ord, err := s.Orders.Create(ctx, repositoryOrderFromReq(u.ID, req))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create order: %v", err)
	}

	return &userv1.SetOrderResponse{Order: toProtoOrder(ord)}, nil
}

func (s *Server) WithdrawOrder(ctx context.Context, req *userv1.WithdrawOrderRequest) (*userv1.WithdrawOrderResponse, error) {
	if req == nil || req.OrderId == 0 {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	p, err := auth.RequireEndUserOrAdmin(ctx)
	if err != nil {
		return nil, err
	}

	u, err := s.resolveCurrentUser(ctx, p)
	if err != nil {
		return nil, err
	}

	// Fetch order and verify ownership.
	ord, err := s.Orders.GetByID(ctx, req.OrderId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get order: %v", err)
	}
	if ord == nil {
		return nil, status.Error(codes.NotFound, "order not found")
	}
	if ord.SubmittedBy != u.ID {
		return nil, status.Error(codes.PermissionDenied, "cannot withdraw another user's order")
	}

	// Withdraw order.
	if err := s.Orders.Withdraw(ctx, req.OrderId); err != nil {
		return nil, status.Errorf(codes.Internal, "withdraw: %v", err)
	}

	// Fetch updated order.
	ord, err = s.Orders.GetByID(ctx, req.OrderId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get order: %v", err)
	}

	return &userv1.WithdrawOrderResponse{Order: toProtoOrder(ord)}, nil
}

// ListOrders retrieves paginated orders for the authenticated user.
func (s *Server) ListOrders(ctx context.Context, req *userv1.ListOrdersRequest) (*userv1.ListOrdersResponse, error) {
	p, err := auth.RequireEndUserOrAdmin(ctx)
	if err != nil {
		return nil, err
	}

	u, err := s.resolveCurrentUser(ctx, p)
	if err != nil {
		return nil, err
	}

	// Extract and validate pagination parameters.
	pageSize := int32(defaultPageSize)
	pageToken := ""
	if req != nil {
		if req.GetPageSize() > 0 {
			pageSize = req.GetPageSize()
		}
		pageToken = req.GetPageToken()
	}
	if pageSize > int32(maxPageSize) {
		pageSize = int32(maxPageSize)
	}

	// Decode cursor if provided.
	var afterSeconds int64
	var afterID int64
	if pageToken != "" {
		if err := decodeCursor(pageToken, &afterSeconds, &afterID); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid page_token: %v", err)
		}
	}

	// Fetch orders for the page.
	list, err := s.Orders.ListByUserIDPage(ctx, u.ID, int(pageSize), afterSeconds, afterID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list orders: %v", err)
	}

	// Convert to proto orders.
	out := make([]*userv1.Order, 0, len(list))
	for i := range list {
		out = append(out, toProtoOrder(&list[i]))
	}

	// Build next page token if we have a full page.
	nextToken := ""
	if int32(len(list)) == pageSize && len(list) > 0 {
		last := list[len(list)-1]
		sec, err := placementToUnixSeconds(last.PlacementAt)
		if err == nil {
			nextToken = encodeCursor(sec, last.ID)
		}
	}

	return &userv1.ListOrdersResponse{Orders: out, NextPageToken: nextToken}, nil
}

// toProtoOrder converts a models.Order to a proto Order message.
func toProtoOrder(o *models.Order) *userv1.Order {
	if o == nil {
		return nil
	}
	return &userv1.Order{
		Id:            o.ID,
		Origin:        &userv1.Coordinates{Lat: o.OriginLat, Lng: o.OriginLng},
		Destination:   &userv1.Coordinates{Lat: o.DestLat, Lng: o.DestLng},
		Status:        toProtoStatus(o.Status),
		SubmittedBy:   o.SubmittedBy,
		PlacementDate: o.PlacementAt,
	}
}

// toProtoStatus converts a models.OrderStatus to a proto Status enum.
func toProtoStatus(s models.OrderStatus) userv1.Status {
	switch s {
	case models.OrderStatusPlaced:
		return userv1.Status_PLACED
	case models.OrderStatusDelivered:
		return userv1.Status_DELIVERED
	case models.OrderStatusEnRoute:
		return userv1.Status_EN_ROUTE
	case models.OrderStatusFailed:
		return userv1.Status_FAILED
	case models.OrderStatusToPickUp:
		return userv1.Status_TO_PICK_UP
	case models.OrderStatusWithdrawn:
		return userv1.Status_WITHDRAWN
	default:
		return userv1.Status_UNSPECIFIED
	}
}

// Helper comment: StartGRPC has been moved to server.go for better separation of concerns.

// repositoryOrderFromReq builds a models.Order from a SetOrderRequest proto message.
func repositoryOrderFromReq(userID int64, req *userv1.SetOrderRequest) *models.Order {
	return &models.Order{
		OriginLat:   req.GetOrigin().GetLat(),
		OriginLng:   req.GetOrigin().GetLng(),
		DestLat:     req.GetDestination().GetLat(),
		DestLng:     req.GetDestination().GetLng(),
		SubmittedBy: userID,
		Status:      models.OrderStatusPlaced,
	}
}

// encodeCursor builds an opaque next_page_token from placement unix seconds and order id.
func encodeCursor(seconds int64, id int64) string {
	raw := strconv.FormatInt(seconds, 10) + cursorSeparator + strconv.FormatInt(id, 10)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// decodeCursor parses an opaque page_token into placement unix seconds and order id.
func decodeCursor(token string, seconds *int64, id *int64) error {
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return fmt.Errorf("base64: %w", err)
	}
	parts := strings.SplitN(string(b), cursorSeparator, 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid cursor format")
	}
	sec, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return fmt.Errorf("parse seconds: %w", err)
	}
	pid, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return fmt.Errorf("parse id: %w", err)
	}
	*seconds = sec
	*id = pid
	return nil
}

// placementToUnixSeconds parses order placement dates into unix seconds.
// Supports RFC3339 format (e.g., 2006-01-02T15:04:05Z) and SQLite CURRENT_TIMESTAMP format.
func placementToUnixSeconds(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty placement_date")
	}

	// Try RFC3339 first.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Unix(), nil
	}

	// Try SQLite CURRENT_TIMESTAMP default format (UTC).
	if t, err := time.ParseInLocation(sqliteDateFormat, s, time.UTC); err == nil {
		return t.Unix(), nil
	}

	return 0, fmt.Errorf("unsupported placement_date format: %q", s)
}
