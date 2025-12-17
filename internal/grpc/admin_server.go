//go:build grpcserver

package grpcserver

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	adminv1 "droneDeliveryManagement/api/admin/v1"
	userv1 "droneDeliveryManagement/api/user/v1"
	"droneDeliveryManagement/internal/auth"
	"droneDeliveryManagement/models"
	"droneDeliveryManagement/repository"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AdminServer implements admin.v1.AdminService.
type AdminServer struct {
	adminv1.UnimplementedAdminServiceServer
	Users  *repository.UserRepository
	Orders *repository.OrderRepository
	Drones *repository.DroneRepository
}

// Authentication is centralized in internal/auth.

// GetOrders lists orders with optional filters and cursor pagination.
func (s *AdminServer) GetOrders(ctx context.Context, req *adminv1.GetOrdersRequest) (*adminv1.GetOrdersResponse, error) {
	if _, err := auth.RequireAdmin(ctx, s.Users); err != nil {
		return nil, err
	}
	if req == nil {
		req = &adminv1.GetOrdersRequest{}
	}
	size := int(req.GetPageSize())
	if size <= 0 {
		size = defaultPageSize
	}
	if size > maxPageSize {
		size = maxPageSize
	}

	var afterSec, afterID int64
	if strings.TrimSpace(req.GetPageToken()) != "" {
		if err := decodeCursor(req.GetPageToken(), &afterSec, &afterID); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid page_token: %v", err)
		}
	}

	// Build filters
	var statuses []models.OrderStatus
	for _, st := range req.GetStatusFilter() {
		switch st {
		case userv1.Status_PLACED:
			statuses = append(statuses, models.OrderStatusPlaced)
		case userv1.Status_DELIVERED:
			statuses = append(statuses, models.OrderStatusDelivered)
		case userv1.Status_EN_ROUTE:
			statuses = append(statuses, models.OrderStatusEnRoute)
		case userv1.Status_FAILED:
			statuses = append(statuses, models.OrderStatusFailed)
		case userv1.Status_TO_PICK_UP:
			statuses = append(statuses, models.OrderStatusToPickUp)
		case userv1.Status_WITHDRAWN:
			statuses = append(statuses, models.OrderStatusWithdrawn)
		}
	}
	var submittedBy *int64
	if req.SubmittedBy != nil {
		v := req.GetSubmittedBy()
		submittedBy = &v
	}
	var from, to *string
	if req.PlacementFrom != nil {
		v := strings.TrimSpace(req.GetPlacementFrom())
		if v != "" {
			from = &v
		}
	}
	if req.PlacementTo != nil {
		v := strings.TrimSpace(req.GetPlacementTo())
		if v != "" {
			to = &v
		}
	}

	list, err := s.Orders.ListAdmin(ctx, repository.ListOrdersAdminParams{
		Statuses:      statuses,
		SubmittedBy:   submittedBy,
		PlacementFrom: from,
		PlacementTo:   to,
		PageSize:      size,
		AfterSeconds:  afterSec,
		AfterID:       afterID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list orders: %v", err)
	}
	resp := &adminv1.GetOrdersResponse{}
	resp.Orders = make([]*userv1.Order, 0, len(list))
	var lastSec, lastID int64
	for i := range list {
		resp.Orders = append(resp.Orders, toProtoOrder(&list[i]))
		sec, err := placementToUnixSeconds(list[i].PlacementAt)
		if err == nil {
			lastSec = sec
			lastID = list[i].ID
		}
	}
	if len(list) == size && lastID != 0 {
		resp.NextPageToken = encodeCursor(lastSec, lastID)
	}
	return resp, nil
}

// UpdateOrderLocation updates both origin and destination of an order.
func (s *AdminServer) UpdateOrderLocation(ctx context.Context, req *adminv1.UpdateOrderLocationRequest) (*adminv1.UpdateOrderLocationResponse, error) {
	if _, err := auth.RequireAdmin(ctx, s.Users); err != nil {
		return nil, err
	}
	if req == nil || req.OrderId == 0 || req.Origin == nil || req.Destination == nil {
		return nil, status.Error(codes.InvalidArgument, "order_id, origin and destination are required")
	}
	if err := s.Orders.UpdateLocations(ctx, req.GetOrderId(), req.GetOrigin().GetLat(), req.GetOrigin().GetLng(), req.GetDestination().GetLat(), req.GetDestination().GetLng()); err != nil {
		if err == sql.ErrNoRows {
			return nil, status.Error(codes.NotFound, "order not found")
		}
		return nil, status.Errorf(codes.Internal, "update order: %v", err)
	}
	ord, err := s.Orders.GetByID(ctx, req.GetOrderId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get order: %v", err)
	}
	if ord == nil {
		return nil, status.Error(codes.NotFound, "order not found")
	}
	return &adminv1.UpdateOrderLocationResponse{Order: toProtoOrder(ord)}, nil
}

// GetDrones lists drones with optional filters and simple id-based cursor pagination.
func (s *AdminServer) GetDrones(ctx context.Context, req *adminv1.GetDronesRequest) (*adminv1.GetDronesResponse, error) {
	if _, err := auth.RequireAdmin(ctx, s.Users); err != nil {
		return nil, err
	}
	if req == nil {
		req = &adminv1.GetDronesRequest{}
	}
	size := int(req.GetPageSize())
	if size <= 0 {
		size = defaultPageSize
	}
	if size > maxPageSize {
		size = maxPageSize
	}

	var afterID int64
	if t := strings.TrimSpace(req.GetPageToken()); t != "" {
		// use simple integer token
		var v int64
		_, err := fmt.Sscanf(t, "%d", &v)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid page_token")
		}
		afterID = v
	}

	// map status
	var st *models.DroneStatus
	if req.Status != nil {
		switch req.GetStatus() {
		case adminv1.DroneStatus_DRONE_STATUS_FIXED:
			v := models.DroneStatusFixed
			st = &v
		case adminv1.DroneStatus_DRONE_STATUS_BROKEN:
			v := models.DroneStatusBroken
			st = &v
		default:
		}
	}

	list, err := s.Drones.ListAdmin(ctx, repository.ListDronesAdminParams{
		Status:               st,
		AssignedOnly:         boolPtr(req.AssignedOnly),
		UnassignedOnly:       boolPtr(req.UnassignedOnly),
		NameOrSerialContains: strPtr(req.NameOrSerialContains),
		PageSize:             size,
		AfterID:              afterID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list drones: %v", err)
	}
	out := make([]*adminv1.Drone, 0, len(list))
	var last int64
	for i := range list {
		out = append(out, toProtoAdminDrone(&list[i]))
		last = list[i].ID
	}
	resp := &adminv1.GetDronesResponse{Drones: out}
	if len(list) == size && last != 0 {
		resp.NextPageToken = fmt.Sprintf("%d", last)
	}
	return resp, nil
}

// UpdateDroneStatus marks a drone as fixed or broken and returns updated drone.
func (s *AdminServer) UpdateDroneStatus(ctx context.Context, req *adminv1.UpdateDroneStatusRequest) (*adminv1.UpdateDroneStatusResponse, error) {
	if _, err := auth.RequireAdmin(ctx, s.Users); err != nil {
		return nil, err
	}
	if req == nil || req.GetDroneId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "drone_id is required")
	}
	var st models.DroneStatus
	switch req.GetStatus() {
	case adminv1.DroneStatus_DRONE_STATUS_FIXED:
		st = models.DroneStatusFixed
	case adminv1.DroneStatus_DRONE_STATUS_BROKEN:
		st = models.DroneStatusBroken
	default:
		return nil, status.Error(codes.InvalidArgument, "status must be FIXED or BROKEN")
	}
	if err := s.Drones.UpdateStatus(ctx, req.GetDroneId(), st); err != nil {
		if err == sql.ErrNoRows {
			return nil, status.Error(codes.NotFound, "drone not found")
		}
		return nil, status.Errorf(codes.Internal, "update status: %v", err)
	}
	d, err := s.Drones.GetByID(ctx, req.GetDroneId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get drone: %v", err)
	}
	if d == nil {
		return nil, status.Error(codes.NotFound, "drone not found")
	}
	return &adminv1.UpdateDroneStatusResponse{Drone: toProtoAdminDrone(d)}, nil
}

func toProtoAdminDrone(d *models.Drone) *adminv1.Drone {
	if d == nil {
		return nil
	}
	out := &adminv1.Drone{
		Id:           d.ID,
		SerialNumber: d.SerialNumber,
		Name:         d.Name,
		Lat:          d.Lat,
		Lng:          d.Lng,
		SpeedMph:     d.SpeedMPH,
	}
	if d.AssignedJob != nil {
		v := *d.AssignedJob
		out.AssignedJob = &v
	}
	switch d.Status {
	case models.DroneStatusFixed:
		out.Status = adminv1.DroneStatus_DRONE_STATUS_FIXED
	case models.DroneStatusBroken:
		out.Status = adminv1.DroneStatus_DRONE_STATUS_BROKEN
	default:
		out.Status = adminv1.DroneStatus_DRONE_STATUS_UNSPECIFIED
	}
	return out
}

func boolPtr(v *bool) *bool {
	if v == nil {
		return nil
	}
	b := *v
	return &b
}
func strPtr(v *string) *string {
	if v == nil {
		return nil
	}
	s := *v
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
