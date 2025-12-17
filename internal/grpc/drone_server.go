//go:build grpcserver

package grpcserver

import (
	"context"
	dronev1 "droneDeliveryManagement/api/drone/v1"
	"droneDeliveryManagement/internal/auth"
	"droneDeliveryManagement/internal/geo"
	"droneDeliveryManagement/models"
	"droneDeliveryManagement/repository"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// DroneServer implements DroneService RPCs.
type DroneServer struct {
	dronev1.UnimplementedDroneServiceServer
	Users  *repository.UserRepository
	Orders *repository.OrderRepository
	Drones *repository.DroneRepository
}

const (
	reasonDrone = "only drone" // Common error message reason.
)

// ...existing code...

// resolveDrone retrieves the drone from the database by serial number, falling back to name.
func (s *DroneServer) resolveDrone(ctx context.Context, principalName string) (*models.Drone, error) {
	dr, err := s.Drones.GetBySerial(ctx, principalName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get drone by serial: %v", err)
	}
	if dr == nil {
		dr, err = s.Drones.GetByName(ctx, principalName)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "get drone by name: %v", err)
		}
	}
	if dr == nil {
		return nil, status.Error(codes.NotFound, "drone not found")
	}
	return dr, nil
}

// ReserveOrder assigns the next available order to a drone if none is already assigned.
// Orders are prioritized by status (to pick up > placed) and placement date.
// The drone cannot be broken or already have an assignment.
func (s *DroneServer) ReserveOrder(ctx context.Context, _ *dronev1.ReserveOrderRequest) (*dronev1.ReserveOrderResponse, error) {
	p, err := auth.RequireDrone(ctx)
	if err != nil {
		return nil, err
	}

	dr, err := s.resolveDrone(ctx, p.Name)
	if err != nil {
		return nil, err
	}

	// Validate drone state.
	if dr.Status == models.DroneStatusBroken {
		return nil, status.Error(codes.FailedPrecondition, "drone is broken")
	}
	if dr.AssignedJob != nil {
		return nil, status.Error(codes.FailedPrecondition, "drone already has an assigned order")
	}

	// Find next available order.
	ord, err := s.Orders.FindNextAvailableForReservation(ctx, dr.ID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "find order: %v", err)
	}
	if ord == nil {
		return nil, status.Error(codes.FailedPrecondition, "no available orders to reserve")
	}

	// Assign order to drone.
	if err := s.Drones.AssignJob(ctx, dr.ID, ord.ID); err != nil {
		return nil, status.Errorf(codes.Aborted, "assign race: %v", err)
	}

	// Track drone in order's path for historical reference.
	if err := s.Orders.AppendDronePath(ctx, ord.ID, dr.ID); err != nil {
		return nil, status.Errorf(codes.Internal, "append drone path: %v", err)
	}

	return &dronev1.ReserveOrderResponse{Order: toProtoOrder(ord)}, nil
}

// GrabOrder transitions an assigned order from placed/to pick up to en route.
// The drone must be within the pickup radius (100 feet) of the pickup location.
func (s *DroneServer) GrabOrder(ctx context.Context, _ *dronev1.GrabOrderRequest) (*dronev1.GrabOrderResponse, error) {
	p, err := auth.RequireDrone(ctx)
	if err != nil {
		return nil, err
	}

	dr, err := s.resolveDrone(ctx, p.Name)
	if err != nil {
		return nil, err
	}

	if dr.AssignedJob == nil {
		return nil, status.Error(codes.FailedPrecondition, "no assigned order")
	}

	ord, err := s.Orders.GetByID(ctx, *dr.AssignedJob)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get order: %v", err)
	}
	if ord == nil {
		_ = s.Drones.UnassignJob(ctx, dr.ID)
		return nil, status.Error(codes.NotFound, "order not found")
	}

	// Validate order status is grabbable.
	if ord.Status != models.OrderStatusPlaced && ord.Status != models.OrderStatusToPickUp {
		return nil, status.Errorf(codes.FailedPrecondition, "cannot grab order with status %s", ord.Status)
	}

	// Determine pickup target based on order status.
	targetLat, targetLng := ord.OriginLat, ord.OriginLng
	if ord.Status == models.OrderStatusToPickUp && ord.PickupLat != nil && ord.PickupLng != nil {
		targetLat, targetLng = *ord.PickupLat, *ord.PickupLng
	}

	// Validate drone is within pickup radius.
	distance := geo.HaversineMiles(dr.Lat, dr.Lng, targetLat, targetLng)
	if distance > geo.FeetToMiles(geo.RadiusFeet) {
		return nil, status.Error(codes.FailedPrecondition, "not within pickup radius")
	}

	// Transition order to en route.
	if err := s.Orders.UpdateStatus(ctx, ord.ID, models.OrderStatusEnRoute); err != nil {
		return nil, status.Errorf(codes.Internal, "set en route: %v", err)
	}

	ord, _ = s.Orders.GetByID(ctx, ord.ID)
	return &dronev1.GrabOrderResponse{Order: toProtoOrder(ord)}, nil
}

// CompleteOrder marks an order as delivered or failed when drone reaches destination.
// Once completed, the drone's assignment is cleared.
func (s *DroneServer) CompleteOrder(ctx context.Context, req *dronev1.CompleteOrderRequest) (*dronev1.CompleteOrderResponse, error) {
	p, err := auth.RequireDrone(ctx)
	if err != nil {
		return nil, err
	}

	dr, err := s.resolveDrone(ctx, p.Name)
	if err != nil {
		return nil, err
	}

	if dr.AssignedJob == nil {
		return nil, status.Error(codes.FailedPrecondition, "no assigned order")
	}

	ord, err := s.Orders.GetByID(ctx, *dr.AssignedJob)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get order: %v", err)
	}
	if ord == nil {
		_ = s.Drones.UnassignJob(ctx, dr.ID)
		return nil, status.Error(codes.NotFound, "order not found")
	}

	// Validate drone is within destination radius.
	distance := geo.HaversineMiles(dr.Lat, dr.Lng, ord.DestLat, ord.DestLng)
	if distance > geo.FeetToMiles(geo.RadiusFeet) {
		return nil, status.Error(codes.FailedPrecondition, "not within destination radius")
	}

	// Mark order as delivered or failed.
	finalStatus := models.OrderStatusFailed
	if req.GetDelivered() {
		finalStatus = models.OrderStatusDelivered
	}
	if err := s.Orders.UpdateStatus(ctx, ord.ID, finalStatus); err != nil {
		return nil, status.Errorf(codes.Internal, "update status: %v", err)
	}

	// Clear drone assignment.
	if err := s.Drones.UnassignJob(ctx, dr.ID); err != nil {
		return nil, status.Errorf(codes.Internal, "unassign: %v", err)
	}

	ord, _ = s.Orders.GetByID(ctx, ord.ID)
	return &dronev1.CompleteOrderResponse{Order: toProtoOrder(ord)}, nil
}

// MarkBroken marks a drone as broken and hands off any en route order.
// If the drone is carrying an order in en route status, the order is transitioned to "to pick up"
// with the pickup location set to the drone's current location for handoff.
func (s *DroneServer) MarkBroken(ctx context.Context, _ *dronev1.MarkBrokenRequest) (*dronev1.MarkBrokenResponse, error) {
	p, err := auth.RequireDrone(ctx)
	if err != nil {
		return nil, err
	}

	dr, err := s.resolveDrone(ctx, p.Name)
	if err != nil {
		return nil, err
	}

	var affected *models.Order
	if dr.AssignedJob != nil {
		ord, err := s.Orders.GetByID(ctx, *dr.AssignedJob)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "get order: %v", err)
		}
		if ord != nil && ord.Status == models.OrderStatusEnRoute {
			// Handoff: transition order to "to pick up" at drone's current location.
			if err := s.Orders.UpdateStatus(ctx, ord.ID, models.OrderStatusToPickUp); err != nil {
				return nil, status.Errorf(codes.Internal, "update status: %v", err)
			}
			if err := s.Orders.UpdatePickupLocation(ctx, ord.ID, dr.Lat, dr.Lng); err != nil {
				return nil, status.Errorf(codes.Internal, "update pickup location: %v", err)
			}
			affected = ord
		}
		_ = s.Drones.UnassignJob(ctx, dr.ID)
	}

	if err := s.Drones.UpdateStatus(ctx, dr.ID, models.DroneStatusBroken); err != nil {
		return nil, status.Errorf(codes.Internal, "update drone status: %v", err)
	}

	if affected != nil {
		affected, _ = s.Orders.GetByID(ctx, affected.ID)
	}

	return &dronev1.MarkBrokenResponse{Order: toProtoOrder(affected)}, nil
}

// Heartbeat updates the drone's location and speed.
func (s *DroneServer) Heartbeat(ctx context.Context, req *dronev1.HeartbeatRequest) (*dronev1.HeartbeatResponse, error) {
	p, err := auth.RequireDrone(ctx)
	if err != nil {
		return nil, err
	}

	if req == nil || req.Location == nil {
		return nil, status.Error(codes.InvalidArgument, "location required")
	}

	dr, err := s.resolveDrone(ctx, p.Name)
	if err != nil {
		return nil, err
	}

	if err := s.Drones.UpdateLocationAndSpeed(ctx, dr.ID, req.Location.GetLat(), req.Location.GetLng(), req.GetSpeedMph()); err != nil {
		return nil, status.Errorf(codes.Internal, "update location: %v", err)
	}

	return &dronev1.HeartbeatResponse{}, nil
}

// calculateETA computes the expected time of arrival in seconds based on order and drone state.
func calculateETA(ord *models.Order, dr *models.Drone) float64 {
	if dr.SpeedMPH <= 0 {
		return 0
	}

	switch ord.Status {
	case models.OrderStatusPlaced, models.OrderStatusToPickUp:
		startLat, startLng := ord.OriginLat, ord.OriginLng
		if ord.Status == models.OrderStatusToPickUp && ord.PickupLat != nil && ord.PickupLng != nil {
			startLat, startLng = *ord.PickupLat, *ord.PickupLng
		}
		distToPickup := geo.HaversineMiles(dr.Lat, dr.Lng, startLat, startLng)
		distToDestination := geo.HaversineMiles(startLat, startLng, ord.DestLat, ord.DestLng)
		return (distToPickup + distToDestination) / dr.SpeedMPH * 3600
	case models.OrderStatusEnRoute:
		distToDestination := geo.HaversineMiles(dr.Lat, dr.Lng, ord.DestLat, ord.DestLng)
		return distToDestination / dr.SpeedMPH * 3600
	default:
		return 0
	}
}

// GetAssignedOrder retrieves details of the currently assigned order with ETA.
func (s *DroneServer) GetAssignedOrder(ctx context.Context, _ *dronev1.GetAssignedOrderRequest) (*dronev1.GetAssignedOrderResponse, error) {
	p, err := auth.RequireDrone(ctx)
	if err != nil {
		return nil, err
	}

	dr, err := s.resolveDrone(ctx, p.Name)
	if err != nil {
		return nil, err
	}

	if dr.AssignedJob == nil {
		return nil, status.Error(codes.FailedPrecondition, "no assigned order")
	}

	ord, err := s.Orders.GetByID(ctx, *dr.AssignedJob)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get order: %v", err)
	}
	if ord == nil {
		return nil, status.Error(codes.Internal, "assigned order not found")
	}

	etaSeconds := calculateETA(ord, dr)
	return &dronev1.GetAssignedOrderResponse{Order: toProtoOrder(ord), EtaSeconds: etaSeconds}, nil
}
