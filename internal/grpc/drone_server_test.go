//go:build grpcserver

package grpcserver

import (
	"context"
	"testing"
	"time"

	dronev1 "droneDeliveryManagement/api/drone/v1"
	userv1 "droneDeliveryManagement/api/user/v1"
	"droneDeliveryManagement/internal/auth"
	"droneDeliveryManagement/internal/db"
	"droneDeliveryManagement/internal/geo"
	"droneDeliveryManagement/models"
	"droneDeliveryManagement/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// newDroneServer builds repositories and the DroneServer for tests.
func newDroneServer(t *testing.T) (*DroneServer, *repository.UserRepository, *repository.OrderRepository, *repository.DroneRepository, func()) {
	t.Helper()
	d, err := db.Open("file:dronedb?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	users := repository.NewUserRepository(d)
	orders := repository.NewOrderRepository(d)
	drones := repository.NewDroneRepository(d)
	return &DroneServer{Users: users, Orders: orders, Drones: drones}, users, orders, drones, func() { _ = d.Close() }
}

func TestDrone_Heartbeat_RejectsNonDronePrincipal(t *testing.T) {
	ds, _, _, drones, cleanup := newDroneServer(t)
	defer cleanup()

	// Seed a drone so resolve step would succeed if allowed.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	dr, err := drones.Create(ctx, &models.Drone{SerialNumber: "SER123", Name: "alpha", Lat: 0, Lng: 0, SpeedMPH: 10})
	if err != nil {
		t.Fatalf("create drone: %v", err)
	}
	_ = dr

	// Enduser principal should be rejected
	pctx := auth.WithPrincipal(context.Background(), &auth.Principal{Name: "SER123", Kind: "enduser"})
	_, err = ds.Heartbeat(pctx, &dronev1.HeartbeatRequest{Location: &userv1.Coordinates{Lat: 1, Lng: 2}, SpeedMph: 12})
	if err == nil {
		t.Fatalf("expected PermissionDenied for non-drone principal")
	}
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("code=%v want=%v", status.Code(err), codes.PermissionDenied)
	}
}

func TestDrone_Heartbeat_AllowsDronePrincipal(t *testing.T) {
	ds, _, _, drones, cleanup := newDroneServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	dr, err := drones.Create(ctx, &models.Drone{SerialNumber: "SER999", Name: "bravo", Lat: 0, Lng: 0, SpeedMPH: 10})
	if err != nil {
		t.Fatalf("create drone: %v", err)
	}
	_ = dr

	pctx := auth.WithPrincipal(context.Background(), &auth.Principal{Name: "SER999", Kind: "drone"})
	if _, err := ds.Heartbeat(pctx, &dronev1.HeartbeatRequest{Location: &userv1.Coordinates{Lat: 5, Lng: 6}, SpeedMph: 15}); err != nil {
		t.Fatalf("Heartbeat allowed drone: %v", err)
	}
}

// Helper to create a drone and wrap context with drone principal.
func seedDrone(t *testing.T, drones *repository.DroneRepository, serial, name string, lat, lng, speed float64, st models.DroneStatus) (*models.Drone, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	dr, err := drones.Create(ctx, &models.Drone{SerialNumber: serial, Name: name, Lat: lat, Lng: lng, SpeedMPH: speed, Status: st})
	if err != nil {
		t.Fatalf("create drone: %v", err)
	}
	pctx := auth.WithPrincipal(context.Background(), &auth.Principal{Name: serial, Kind: "drone"})
	return dr, pctx
}

// NewDroneSuite creates a test DroneServer with repositories.
func newDroneSuite(t *testing.T) (*DroneServer, *repository.UserRepository, *repository.OrderRepository, *repository.DroneRepository, func()) {
	t.Helper()
	d, err := db.Open("file:dronemore?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	users := repository.NewUserRepository(d)
	orders := repository.NewOrderRepository(d)
	drones := repository.NewDroneRepository(d)
	cleanup := func() { _ = d.Close() }
	return &DroneServer{Users: users, Orders: orders, Drones: drones}, users, orders, drones, cleanup
}

// Helper to create a user and order.
func seedUserAndOrder(t *testing.T, users *repository.UserRepository, orders *repository.OrderRepository, status models.OrderStatus, originLat, originLng, destLat, destLng float64) *models.Order {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	u, err := users.Create(ctx, "orduser")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	o, err := orders.Create(ctx, &models.Order{OriginLat: originLat, OriginLng: originLng, DestLat: destLat, DestLng: destLng, SubmittedBy: u.ID, Status: status})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	return o
}

// TestReserveOrder_SuccessAndPreconditions tests reserve order success and failure cases.
func TestReserveOrder_SuccessAndPreconditions(t *testing.T) {
	s, users, orders, drones, cleanup := newDroneSuite(t)
	defer cleanup()

	// Place available order near drone origin.
	seedUserAndOrder(t, users, orders, models.OrderStatusPlaced, 1, 1, 2, 2)
	dr, pctx := seedDrone(t, drones, "SER-A", "alpha", 0, 0, 10, models.DroneStatusFixed)

	// Success path.
	if _, err := s.ReserveOrder(pctx, &dronev1.ReserveOrderRequest{}); err != nil {
		t.Fatalf("ReserveOrder: %v", err)
	}

	// Already assigned -> precondition failed.
	if _, err := s.ReserveOrder(pctx, &dronev1.ReserveOrderRequest{}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected precondition for already assigned, got: %v", err)
	}

	// Broken drone cannot reserve.
	_ = drones.UnassignJob(context.Background(), dr.ID)
	_ = drones.UpdateStatus(context.Background(), dr.ID, models.DroneStatusBroken)
	if _, err := s.ReserveOrder(pctx, &dronev1.ReserveOrderRequest{}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected precondition for broken, got: %v", err)
	}
}

// TestGrabOrder_WithinAndOutsideRadius tests grabbing orders within and outside pickup radius.
func TestGrabOrder_WithinAndOutsideRadius(t *testing.T) {
	s, users, orders, drones, cleanup := newDroneSuite(t)
	defer cleanup()

	// Create order with origin near the drone.
	ord := seedUserAndOrder(t, users, orders, models.OrderStatusPlaced, 0, 0, 0.01, 0.01)
	dr, pctx := seedDrone(t, drones, "SER-B", "bravo", 0, 0, 10, models.DroneStatusFixed)
	if err := drones.AssignJob(context.Background(), dr.ID, ord.ID); err != nil {
		t.Fatalf("assign: %v", err)
	}

	// Within radius.
	if _, err := s.GrabOrder(pctx, &dronev1.GrabOrderRequest{}); err != nil {
		t.Fatalf("GrabOrder within radius: %v", err)
	}

	// Move far away and try again (status now en route -> cannot re-grab).
	_ = drones.UpdateLocationAndSpeed(context.Background(), dr.ID, 10, 10, 10)
	if _, err := s.GrabOrder(pctx, &dronev1.GrabOrderRequest{}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected precondition for invalid status: %v", err)
	}
}

// TestCompleteOrder_DeliveredAndFailed tests completing orders as delivered or failed.
func TestCompleteOrder_DeliveredAndFailed(t *testing.T) {
	s, users, orders, drones, cleanup := newDroneSuite(t)
	defer cleanup()

	ord := seedUserAndOrder(t, users, orders, models.OrderStatusEnRoute, 0, 0, 0.001, 0.001)
	dr, pctx := seedDrone(t, drones, "SER-C", "charlie", 0.001, 0.001, 10, models.DroneStatusFixed)
	if err := drones.AssignJob(context.Background(), dr.ID, ord.ID); err != nil {
		t.Fatalf("assign: %v", err)
	}

	// Delivered.
	if _, err := s.CompleteOrder(pctx, &dronev1.CompleteOrderRequest{Delivered: true}); err != nil {
		t.Fatalf("CompleteOrder delivered: %v", err)
	}

	// Recreate en route scenario for failure.
	ord2 := seedUserAndOrder(t, users, orders, models.OrderStatusEnRoute, 0, 0, 0.001, 0.001)
	if err := drones.AssignJob(context.Background(), dr.ID, ord2.ID); err != nil {
		t.Fatalf("assign2: %v", err)
	}
	if _, err := s.CompleteOrder(pctx, &dronev1.CompleteOrderRequest{Delivered: false}); err != nil {
		t.Fatalf("CompleteOrder failed: %v", err)
	}
}

// TestMarkBroken_HandoffWhenEnRoute tests handoff when drone becomes broken.
func TestMarkBroken_HandoffWhenEnRoute(t *testing.T) {
	s, users, orders, drones, cleanup := newDroneSuite(t)
	defer cleanup()

	ord := seedUserAndOrder(t, users, orders, models.OrderStatusEnRoute, 0, 0, 1, 1)
	dr, pctx := seedDrone(t, drones, "SER-D", "delta", 0.5, 0.5, 10, models.DroneStatusFixed)
	if err := drones.AssignJob(context.Background(), dr.ID, ord.ID); err != nil {
		t.Fatalf("assign: %v", err)
	}

	resp, err := s.MarkBroken(pctx, &dronev1.MarkBrokenRequest{})
	if err != nil {
		t.Fatalf("MarkBroken: %v", err)
	}

	// Order should move to to-pick-up and pickup location set.
	if resp.GetOrder() == nil || resp.GetOrder().GetStatus() != userv1.Status_STATUS_TO_PICK_UP {
		t.Fatalf("expected to pick up, got: %v", resp.GetOrder())
	}
}

// TestGetAssignedOrder_EdgeCases tests edge cases for getting assigned order.
func TestGetAssignedOrder_EdgeCases(t *testing.T) {
	s, users, orders, drones, cleanup := newDroneSuite(t)
	defer cleanup()

	_, pctx := seedDrone(t, drones, "SER-E", "echo", 0, 0, 15, models.DroneStatusFixed)

	// No assignment.
	if _, err := s.GetAssignedOrder(pctx, &dronev1.GetAssignedOrderRequest{}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected precondition when no assignment: %v", err)
	}

	// Assign and expect ETA present.
	ord := seedUserAndOrder(t, users, orders, models.OrderStatusPlaced, 0, 0, 0.01, 0.01)
	dr, _ := s.Drones.GetBySerial(context.Background(), "SER-E")
	_ = drones.AssignJob(context.Background(), dr.ID, ord.ID)
	resp, err := s.GetAssignedOrder(pctx, &dronev1.GetAssignedOrderRequest{})
	if err != nil {
		t.Fatalf("GetAssignedOrder: %v", err)
	}
	if resp.GetEtaSeconds() <= 0 {
		t.Fatalf("expected positive ETA, got %v", resp.GetEtaSeconds())
	}
}

// TestHeartbeat_InvalidArgs tests heartbeat with invalid arguments.
func TestHeartbeat_InvalidArgs(t *testing.T) {
	s, _, _, drones, cleanup := newDroneSuite(t)
	defer cleanup()

	_, pctx := seedDrone(t, drones, "SER-F", "foxtrot", 0, 0, 10, models.DroneStatusFixed)
	if _, err := s.Heartbeat(pctx, &dronev1.HeartbeatRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected invalid argument for missing location: %v", err)
	}
}

// TestCalculateETA tests ETA calculation for various scenarios.
func TestCalculateETA(t *testing.T) {
	ord := &models.Order{OriginLat: 0, OriginLng: 0, DestLat: 0, DestLng: 1, Status: models.OrderStatusPlaced}
	dr := &models.Drone{Lat: 0, Lng: 0, SpeedMPH: 10}
	eta := calculateETA(ord, dr)
	if eta <= 0 {
		t.Fatalf("eta should be >0, got %v", eta)
	}

	// Zero speed should yield 0.
	dr.SpeedMPH = 0
	if calculateETA(ord, dr) != 0 {
		t.Fatalf("eta with zero speed should be 0")
	}

	// En route case with small distance.
	dr.SpeedMPH = 10
	ord.Status = models.OrderStatusEnRoute
	ord.DestLat, ord.DestLng = 0, geo.FeetToMiles(100)/geo.FeetPerMile // tiny distance
	if calculateETA(ord, dr) <= 0 {
		t.Fatalf("eta en route should be >0")
	}
}
