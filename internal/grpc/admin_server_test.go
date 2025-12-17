//go:build grpcserver

package grpcserver

import (
	"context"
	"database/sql"
	"testing"
	"time"

	adminv1 "droneDeliveryManagement/api/admin/v1"
	"droneDeliveryManagement/internal/auth"
	"droneDeliveryManagement/internal/db"
	"droneDeliveryManagement/models"
	"droneDeliveryManagement/repository"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// openTestDB opens an in-memory SQLite database and returns the *sql.DB and cleanup.
func openTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	d, err := db.Open("file:admindb?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return d, func() { _ = d.Close() }
}

// newAdminServer builds repositories and the AdminServer for tests.
func newAdminServer(t *testing.T) (*AdminServer, *repository.UserRepository, *repository.OrderRepository, *repository.DroneRepository, func()) {
	t.Helper()
	d, cleanup := openTestDB(t)
	users := repository.NewUserRepository(d)
	orders := repository.NewOrderRepository(d)
	drones := repository.NewDroneRepository(d)
	return &AdminServer{Users: users, Orders: orders, Drones: drones}, users, orders, drones, cleanup
}

// createUserWithRole creates a user and sets its role.
func createUserWithRole(t *testing.T, users *repository.UserRepository, username, role string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := users.Create(ctx, username); err != nil {
		// Ignore if exists
		if _, err2 := users.GetByUsername(ctx, username); err2 != nil {
			t.Fatalf("create user %s: %v (get err: %v)", username, err, err2)
		}
	}
	if role != "" {
		if err := users.UpdateRoleByUsername(ctx, username, role); err != nil {
			t.Fatalf("update role: %v", err)
		}
	}
}

// Expose DB accessor for test-only use by extending repository via method on interface is not present;
// add a small helper using Go's ability to access unexported field via a method here.
// We add a small wrapper method on UserRepository through composition: implement here using type alias.

// To keep tests simple, add a tiny shim method via embedding using a local type alias.
type userRepoShim struct{ *repository.UserRepository } // no longer used; keep type to avoid import churn

// TestAdminAuth_SpoofRejected ensures that an end-user cannot spoof admin by setting kind=admin in JWT/principal.
func TestAdminAuth_SpoofRejected(t *testing.T) {
	as, users, _, _, cleanup := newAdminServer(t)
	defer cleanup()

	// Create end user "alice" (role defaults to 'end user').
	createUserWithRole(t, users, "alice", "end user")

	// Context with spoofed principal: kind=admin, name=alice
	ctx := auth.WithPrincipal(context.Background(), &auth.Principal{Name: "alice", Kind: "admin"})

	// Call an admin method
	_, err := as.GetDrones(ctx, &adminv1.GetDronesRequest{PageSize: 1})
	if err == nil {
		t.Fatalf("expected permission denied for spoofed admin, got nil error")
	}
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("error code = %v, want PermissionDenied", status.Code(err))
	}
}

// TestAdminAuth_RealAdminAllowed ensures a true admin is allowed.
func TestAdminAuth_RealAdminAllowed(t *testing.T) {
	as, users, _, _, cleanup := newAdminServer(t)
	defer cleanup()

	// Create admin user "root" with role 'admin'.
	createUserWithRole(t, users, "root", "admin")

	ctx := auth.WithPrincipal(context.Background(), &auth.Principal{Name: "root", Kind: "admin"})

	if _, err := as.GetDrones(ctx, &adminv1.GetDronesRequest{PageSize: 1}); err != nil {
		t.Fatalf("admin GetDrones: %v", err)
	}
}

// seedOrders creates orders with different statuses and placement dates.
func seedOrders(t *testing.T, orders *repository.OrderRepository, users *repository.UserRepository, n int) []int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	u, err := users.Create(ctx, "adminseeder")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	ids := make([]int64, 0, n)
	statuses := []string{"placed", "to pick up", "en route", "delivered", "failed", "withdrawn"}
	for i := 0; i < n; i++ {
		st := statuses[i%len(statuses)]
		o, err := orders.Create(ctx, &models.Order{
			OriginLat:   1 + float64(i),
			OriginLng:   2 + float64(i),
			DestLat:     3 + float64(i),
			DestLng:     4 + float64(i),
			SubmittedBy: u.ID,
			Status:      models.OrderStatus(st),
		})
		if err != nil {
			t.Fatalf("create order %d: %v", i, err)
		}
		ids = append(ids, o.ID)
	}
	return ids
}

// TestAdmin_GetOrders_FilterAndPagination tests filtering and pagination of orders.
func TestAdmin_GetOrders_FilterAndPagination(t *testing.T) {
	d, err := db.Open("file:adminmore?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	users := repository.NewUserRepository(d)
	orders := repository.NewOrderRepository(d)
	drones := repository.NewDroneRepository(d)
	s := &AdminServer{Users: users, Orders: orders, Drones: drones}

	// Create real admin.
	ctx := context.Background()
	if _, err := users.Create(ctx, "root"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := users.UpdateRoleByUsername(ctx, "root", "admin"); err != nil {
		t.Fatalf("role: %v", err)
	}
	actx := auth.WithPrincipal(ctx, &auth.Principal{Name: "root", Kind: "admin"})

	seedOrders(t, orders, users, 8)

	// Filter by status: DELIVERED
	resp, err := s.GetOrders(actx, &adminv1.GetOrdersRequest{StatusFilter: []uint32{3}, PageSize: 5}) // 3 = delivered
	if err != nil {
		t.Fatalf("GetOrders filter: %v", err)
	}
	for _, o := range resp.GetOrders() {
		if o.GetStatus() != 3 {
			t.Fatalf("unexpected status in filter result: %v", o.GetStatus())
		}
	}

	// Pagination: small page size and follow token.
	token := ""
	total := 0
	for i := 0; i < 10; i++ {
		r, err := s.GetOrders(actx, &adminv1.GetOrdersRequest{PageSize: 2, PageToken: token})
		if err != nil {
			t.Fatalf("page %d: %v", i, err)
		}
		total += len(r.GetOrders())
		if r.GetNextPageToken() == "" {
			break
		}
		token = r.GetNextPageToken()
	}
	if total == 0 {
		t.Fatalf("expected some orders via pagination")
	}
}

// TestAdmin_UpdateDroneStatus tests updating drone status.
func TestAdmin_UpdateDroneStatus(t *testing.T) {
	d, err := db.Open("file:adminmore2?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	users := repository.NewUserRepository(d)
	orders := repository.NewOrderRepository(d)
	drones := repository.NewDroneRepository(d)
	s := &AdminServer{Users: users, Orders: orders, Drones: drones}

	ctx := context.Background()
	if _, err := users.Create(ctx, "root"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := users.UpdateRoleByUsername(ctx, "root", "admin"); err != nil {
		t.Fatalf("role: %v", err)
	}
	actx := auth.WithPrincipal(ctx, &auth.Principal{Name: "root", Kind: "admin"})

	dr, err := drones.Create(ctx, &models.Drone{SerialNumber: "S-77", Name: "d-77", Status: models.DroneStatusFixed})
	if err != nil {
		t.Fatalf("create drone: %v", err)
	}

	// Set to broken then back to fixed.
	if _, err := s.UpdateDroneStatus(actx, &adminv1.UpdateDroneStatusRequest{DroneId: dr.ID, Status: adminv1.DroneStatus_DRONE_STATUS_BROKEN}); err != nil {
		t.Fatalf("set broken: %v", err)
	}
	if _, err := s.UpdateDroneStatus(actx, &adminv1.UpdateDroneStatusRequest{DroneId: dr.ID, Status: adminv1.DroneStatus_DRONE_STATUS_FIXED}); err != nil {
		t.Fatalf("set fixed: %v", err)
	}
}
