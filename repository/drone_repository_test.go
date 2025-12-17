package repository

import (
	"context"
	"testing"

	"droneDeliveryManagement/internal/db"
	"droneDeliveryManagement/models"
)

func TestDroneRepository_CRUD_Status_Assignments(t *testing.T) {
	d, err := db.Open("file:dronerepo?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	drones := NewDroneRepository(d)
	orders := NewOrderRepository(d)
	users := NewUserRepository(d)
	ctx := context.Background()

	// Seed user and order for assignment
	u, err := users.Create(ctx, "u1")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	ord, err := orders.Create(ctx, &models.Order{OriginLat: 1, OriginLng: 2, DestLat: 3, DestLng: 4, SubmittedBy: u.ID, Status: models.OrderStatusPlaced})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	// Create drone
	dr, err := drones.Create(ctx, &models.Drone{SerialNumber: "S-1", Name: "alpha", Lat: 0, Lng: 0, SpeedMPH: 10, Status: models.DroneStatusFixed})
	if err != nil {
		t.Fatalf("create drone: %v", err)
	}
	if dr.ID == 0 {
		t.Fatalf("expected id assigned")
	}

	// GetBySerial and GetByName
	if got, _ := drones.GetBySerial(ctx, "S-1"); got == nil || got.ID != dr.ID {
		t.Fatalf("GetBySerial mismatch: %+v", got)
	}
	if got, _ := drones.GetByName(ctx, "alpha"); got == nil || got.ID != dr.ID {
		t.Fatalf("GetByName mismatch: %+v", got)
	}

	// Update location/speed
	if err := drones.UpdateLocationAndSpeed(ctx, dr.ID, 5, 6, 20); err != nil {
		t.Fatalf("update loc/speed: %v", err)
	}
	dr2, _ := drones.GetByID(ctx, dr.ID)
	if dr2.Lat != 5 || dr2.Lng != 6 || dr2.SpeedMPH != 20 {
		t.Fatalf("location/speed not updated: %+v", dr2)
	}

	// Update status
	if err := drones.UpdateStatus(ctx, dr.ID, models.DroneStatusBroken); err != nil {
		t.Fatalf("update status: %v", err)
	}
	dr3, _ := drones.GetByID(ctx, dr.ID)
	if dr3.Status != models.DroneStatusBroken {
		t.Fatalf("status not updated: %+v", dr3)
	}

	// Assign job and verify GetByOrderID
	if err := drones.AssignJob(ctx, dr.ID, ord.ID); err != nil {
		t.Fatalf("assign: %v", err)
	}
	dr4, _ := drones.GetByOrderID(ctx, ord.ID)
	if dr4 == nil || dr4.ID != dr.ID {
		t.Fatalf("GetByOrderID mismatch: %+v", dr4)
	}

	// Unassign job
	if err := drones.UnassignJob(ctx, dr.ID); err != nil {
		t.Fatalf("unassign: %v", err)
	}
	if got, _ := drones.GetByOrderID(ctx, ord.ID); got != nil {
		t.Fatalf("expected no drone for order after unassign, got: %+v", got)
	}

	// List admin filtered by status
	st := models.DroneStatusBroken
	list, err := drones.ListAdmin(ctx, ListDronesAdminParams{Status: &st, PageSize: 10})
	if err != nil || len(list) == 0 {
		t.Fatalf("ListAdmin: %v len=%d", err, len(list))
	}

	// Delete
	if err := drones.Delete(ctx, dr.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if gone, _ := drones.GetByID(ctx, dr.ID); gone != nil {
		t.Fatalf("expected drone deleted, got: %+v", gone)
	}
}
