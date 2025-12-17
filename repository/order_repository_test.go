package repository

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"droneDeliveryManagement/internal/db"
	"droneDeliveryManagement/models"
)

// TestAppendAndCheckDronePath tests the drone_path functionality
func TestAppendAndCheckDronePath(t *testing.T) {
	// Use a test database
	testDB := "test_drone_path.db"
	os.Remove(testDB) // Clean up before test
	defer os.Remove(testDB)

	d, err := db.Open(testDB)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer d.Close()

	orderRepo := NewOrderRepository(d)
	droneRepo := NewDroneRepository(d)
	userRepo := NewUserRepository(d)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a user
	u, err := userRepo.Create(ctx, "testuser")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Create an order
	ord := &models.Order{
		OriginLat:   37.7749,
		OriginLng:   -122.4194,
		DestLat:     34.0522,
		DestLng:     -118.2437,
		SubmittedBy: u.ID,
		Status:      models.OrderStatusPlaced,
	}
	ord, err = orderRepo.Create(ctx, ord)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	// Verify initial drone_path is empty
	if ord.DronePath != "" {
		t.Errorf("initial drone_path should be empty, got: %s", ord.DronePath)
	}

	// Create two drones
	drone1 := &models.Drone{
		SerialNumber: "DRONE-001",
		Lat:          37.7749,
		Lng:          -122.4194,
		SpeedMPH:     50.0,
		Status:       models.DroneStatusFixed,
		Name:         "Drone-1",
	}
	drone1, err = droneRepo.Create(ctx, drone1)
	if err != nil {
		t.Fatalf("create drone1: %v", err)
	}

	drone2 := &models.Drone{
		SerialNumber: "DRONE-002",
		Lat:          37.7749,
		Lng:          -122.4194,
		SpeedMPH:     50.0,
		Status:       models.DroneStatusFixed,
		Name:         "Drone-2",
	}
	drone2, err = droneRepo.Create(ctx, drone2)
	if err != nil {
		t.Fatalf("create drone2: %v", err)
	}

	// Test AppendDronePath
	err = orderRepo.AppendDronePath(ctx, ord.ID, drone1.ID)
	if err != nil {
		t.Fatalf("append drone1 to path: %v", err)
	}

	ord, _ = orderRepo.GetByID(ctx, ord.ID)
	expectedPath := fmt.Sprintf("%d", drone1.ID)
	if ord.DronePath != expectedPath {
		t.Errorf("after first append, expected path '%s', got '%s'", expectedPath, ord.DronePath)
	}

	// Test IsDroneInPath for drone1
	inPath, err := orderRepo.IsDroneInPath(ctx, ord.ID, drone1.ID)
	if err != nil {
		t.Fatalf("check drone1 in path: %v", err)
	}
	if !inPath {
		t.Error("drone1 should be in path")
	}

	// Test IsDroneInPath for drone2 (should not be in path)
	inPath, err = orderRepo.IsDroneInPath(ctx, ord.ID, drone2.ID)
	if err != nil {
		t.Fatalf("check drone2 in path: %v", err)
	}
	if inPath {
		t.Error("drone2 should NOT be in path")
	}

	// Append drone2
	err = orderRepo.AppendDronePath(ctx, ord.ID, drone2.ID)
	if err != nil {
		t.Fatalf("append drone2 to path: %v", err)
	}

	ord, _ = orderRepo.GetByID(ctx, ord.ID)
	expectedPath = fmt.Sprintf("%d,%d", drone1.ID, drone2.ID)
	if ord.DronePath != expectedPath {
		t.Errorf("after second append, expected path '%s', got '%s'", expectedPath, ord.DronePath)
	}

	// Verify both drones are now in path
	inPath, err = orderRepo.IsDroneInPath(ctx, ord.ID, drone1.ID)
	if err != nil {
		t.Fatalf("check drone1 in path after append: %v", err)
	}
	if !inPath {
		t.Error("drone1 should still be in path")
	}

	inPath, err = orderRepo.IsDroneInPath(ctx, ord.ID, drone2.ID)
	if err != nil {
		t.Fatalf("check drone2 in path after append: %v", err)
	}
	if !inPath {
		t.Error("drone2 should be in path")
	}

	t.Log("✅ All drone_path tests passed")
}

// TestFindNextAvailableForReservation tests the order prioritization logic for drone reservations
func TestFindNextAvailableForReservation(t *testing.T) {
	// Use a test database
	testDB := "test_find_next_order.db"
	os.Remove(testDB) // Clean up before test
	defer os.Remove(testDB)

	d, err := db.Open(testDB)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer d.Close()

	orderRepo := NewOrderRepository(d)
	droneRepo := NewDroneRepository(d)
	userRepo := NewUserRepository(d)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a user
	u, err := userRepo.Create(ctx, "testuser")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Create three drones
	drone1 := &models.Drone{
		SerialNumber: "DRONE-P1",
		Lat:          37.7749,
		Lng:          -122.4194,
		SpeedMPH:     50.0,
		Status:       models.DroneStatusFixed,
		Name:         "Drone-Priority-1",
	}
	drone1, err = droneRepo.Create(ctx, drone1)
	if err != nil {
		t.Fatalf("create drone1: %v", err)
	}

	drone2 := &models.Drone{
		SerialNumber: "DRONE-P2",
		Lat:          37.7749,
		Lng:          -122.4194,
		SpeedMPH:     50.0,
		Status:       models.DroneStatusFixed,
		Name:         "Drone-Priority-2",
	}
	drone2, err = droneRepo.Create(ctx, drone2)
	if err != nil {
		t.Fatalf("create drone2: %v", err)
	}

	drone3 := &models.Drone{
		SerialNumber: "DRONE-P3",
		Lat:          37.7749,
		Lng:          -122.4194,
		SpeedMPH:     50.0,
		Status:       models.DroneStatusFixed,
		Name:         "Drone-Priority-3",
	}
	drone3, err = droneRepo.Create(ctx, drone3)
	if err != nil {
		t.Fatalf("create drone3: %v", err)
	}

	// Create three orders:
	// 1. Order 1: status='placed'
	// 2. Order 2: status='to pick up' (should have higher priority)
	// 3. Order 3: status='placed' with drone1 in path (drone1 should skip this)

	ord1 := &models.Order{
		OriginLat:   37.7749,
		OriginLng:   -122.4194,
		DestLat:     34.0522,
		DestLng:     -118.2437,
		SubmittedBy: u.ID,
		Status:      models.OrderStatusPlaced,
	}
	ord1, err = orderRepo.Create(ctx, ord1)
	if err != nil {
		t.Fatalf("create order1: %v", err)
	}

	ord2 := &models.Order{
		OriginLat:   37.7749,
		OriginLng:   -122.4194,
		DestLat:     34.0522,
		DestLng:     -118.2437,
		SubmittedBy: u.ID,
		Status:      models.OrderStatusToPickUp,
	}
	ord2, err = orderRepo.Create(ctx, ord2)
	if err != nil {
		t.Fatalf("create order2: %v", err)
	}

	ord3 := &models.Order{
		OriginLat:   37.7749,
		OriginLng:   -122.4194,
		DestLat:     34.0522,
		DestLng:     -118.2437,
		SubmittedBy: u.ID,
		Status:      models.OrderStatusPlaced,
	}
	ord3, err = orderRepo.Create(ctx, ord3)
	if err != nil {
		t.Fatalf("create order3: %v", err)
	}

	// Add drone1 to order3's drone_path (so drone1 should skip it)
	err = orderRepo.AppendDronePath(ctx, ord3.ID, drone1.ID)
	if err != nil {
		t.Fatalf("append drone1 to order3 path: %v", err)
	}

	// Test 1: drone1 should get order2 (to pick up) even though ord1 was created first (placed)
	nextOrd, err := orderRepo.FindNextAvailableForReservation(ctx, drone1.ID)
	if err != nil {
		t.Fatalf("find next for drone1: %v", err)
	}
	if nextOrd == nil {
		t.Fatal("expected to find an order for drone1")
	}
	if nextOrd.ID != ord2.ID {
		t.Errorf("drone1 should get order2 (to pick up), but got order %d", nextOrd.ID)
	}
	if nextOrd.Status != models.OrderStatusToPickUp {
		t.Errorf("expected status 'to pick up', got '%s'", nextOrd.Status)
	}

	// Assign ord2 to drone1 and add to path
	err = droneRepo.AssignJob(ctx, drone1.ID, ord2.ID)
	if err != nil {
		t.Fatalf("assign order2 to drone1: %v", err)
	}
	err = orderRepo.AppendDronePath(ctx, ord2.ID, drone1.ID)
	if err != nil {
		t.Fatalf("append drone1 to order2 path: %v", err)
	}

	// Test 2: drone2 should get order1 (placed) since ord2 is now assigned and ord3 has drone1 in path
	nextOrd, err = orderRepo.FindNextAvailableForReservation(ctx, drone2.ID)
	if err != nil {
		t.Fatalf("find next for drone2: %v", err)
	}
	if nextOrd == nil {
		t.Fatal("expected to find an order for drone2")
	}
	if nextOrd.ID != ord1.ID {
		t.Errorf("drone2 should get order1 (placed), but got order %d", nextOrd.ID)
	}
	if nextOrd.Status != models.OrderStatusPlaced {
		t.Errorf("expected status 'placed', got '%s'", nextOrd.Status)
	}

	// Test 3: drone1 should get order1 (placed) since ord2 is assigned and ord3 has drone1 in path
	nextOrd, err = orderRepo.FindNextAvailableForReservation(ctx, drone1.ID)
	if err != nil {
		t.Fatalf("find next for drone1 (second time): %v", err)
	}
	if nextOrd == nil {
		t.Fatal("expected to find an order for drone1")
	}
	if nextOrd.ID != ord1.ID {
		t.Errorf("drone1 should get order1 (placed), but got order %d", nextOrd.ID)
	}

	// Test 4: drone3 should get order3 (placed) since ord2 is assigned and ord1 will be assigned
	nextOrd, err = orderRepo.FindNextAvailableForReservation(ctx, drone3.ID)
	if err != nil {
		t.Fatalf("find next for drone3: %v", err)
	}
	if nextOrd == nil {
		t.Fatal("expected to find an order for drone3")
	}
	if nextOrd.ID != ord1.ID {
		t.Errorf("drone3 should get order1 (placed before ord3), but got order %d", nextOrd.ID)
	}

	t.Log("✅ All FindNextAvailableForReservation tests passed")
}
