package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"droneDeliveryManagement/models"
)

// OrderRepository is the core repository for Order entities.
// It handles basic CRUD operations and query building.
type OrderRepository struct {
	db *sql.DB
}

// NewOrderRepository creates a new OrderRepository.
func NewOrderRepository(db *sql.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

// Create inserts a new order. Status defaults to 'placed' if empty.
func (r *OrderRepository) Create(ctx context.Context, o *models.Order) (*models.Order, error) {
	if o == nil {
		return nil, errors.New("order is nil")
	}
	if o.Status == "" {
		o.Status = models.OrderStatusPlaced
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// Use INSERT and then query back to capture placement_date
	res, err := r.db.ExecContext(ctx, `INSERT INTO orders (origin_lat, origin_lng, dest_lat, dest_lng, status, submitted_by) VALUES (?,?,?,?,?,?)`,
		o.OriginLat, o.OriginLng, o.DestLat, o.DestLng, string(o.Status), o.SubmittedBy)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	o2, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if o2 == nil {
		return nil, fmt.Errorf("created order not found: id=%d", id)
	}
	return o2, nil
}

// GetByID fetches an order by its ID.
func (r *OrderRepository) GetByID(ctx context.Context, id int64) (*models.Order, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var o models.Order
	var status string
	var pickupLat, pickupLng sql.NullFloat64
	var dronePath sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT id, origin_lat, origin_lng, dest_lat, dest_lng, status, placement_date, submitted_by, pickup_lat, pickup_lng, drone_path FROM orders WHERE id = ?`, id).
		Scan(&o.ID, &o.OriginLat, &o.OriginLng, &o.DestLat, &o.DestLng, &status, &o.PlacementAt, &o.SubmittedBy, &pickupLat, &pickupLng, &dronePath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	o.Status = models.OrderStatus(status)
	if pickupLat.Valid {
		v := pickupLat.Float64
		o.PickupLat = &v
	}
	if pickupLng.Valid {
		v := pickupLng.Float64
		o.PickupLng = &v
	}
	if dronePath.Valid {
		o.DronePath = dronePath.String
	}
	return &o, nil
}

// GetByUserID returns the most recent order for the given user (by placement_date desc).
func (r *OrderRepository) GetByUserID(ctx context.Context, userID int64) (*models.Order, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var o models.Order
	var status string
	var pickupLat, pickupLng sql.NullFloat64
	var dronePath sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT id, origin_lat, origin_lng, dest_lat, dest_lng, status, placement_date, submitted_by, pickup_lat, pickup_lng, drone_path FROM orders WHERE submitted_by = ? ORDER BY placement_date DESC, id DESC LIMIT 1`, userID).
		Scan(&o.ID, &o.OriginLat, &o.OriginLng, &o.DestLat, &o.DestLng, &status, &o.PlacementAt, &o.SubmittedBy, &pickupLat, &pickupLng, &dronePath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	o.Status = models.OrderStatus(status)
	if pickupLat.Valid {
		v := pickupLat.Float64
		o.PickupLat = &v
	}
	if pickupLng.Valid {
		v := pickupLng.Float64
		o.PickupLng = &v
	}
	if dronePath.Valid {
		o.DronePath = dronePath.String
	}
	return &o, nil
}

// Delete removes an order by ID.
func (r *OrderRepository) Delete(ctx context.Context, id int64) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := r.db.ExecContext(ctx, `DELETE FROM orders WHERE id = ?`, id)
	return err
}

// UpdateStatus updates the status of an order.
func (r *OrderRepository) UpdateStatus(ctx context.Context, id int64, status models.OrderStatus) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := r.db.ExecContext(ctx, `UPDATE orders SET status = ? WHERE id = ?`, string(status), id)
	return err
}

// UpdatePickupLocation sets pickup_lat and pickup_lng for an order (used for handoff).
func (r *OrderRepository) UpdatePickupLocation(ctx context.Context, id int64, lat, lng float64) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := r.db.ExecContext(ctx, `UPDATE orders SET pickup_lat = ?, pickup_lng = ? WHERE id = ?`, lat, lng, id)
	return err
}

// UpdateLocations updates both origin and destination coordinates for an order.
func (r *OrderRepository) UpdateLocations(ctx context.Context, id int64, originLat, originLng, destLat, destLng float64) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	res, err := r.db.ExecContext(ctx, `UPDATE orders SET origin_lat = ?, origin_lng = ?, dest_lat = ?, dest_lng = ? WHERE id = ?`, originLat, originLng, destLat, destLng, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// IsDroneInPath checks if a drone ID is already in the order's drone_path.
func (r *OrderRepository) IsDroneInPath(ctx context.Context, orderID int64, droneID int64) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var dronePath sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT drone_path FROM orders WHERE id = ?`, orderID).Scan(&dronePath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if !dronePath.Valid || dronePath.String == "" {
		return false, nil
	}
	// Parse comma-delimited path
	droneIDStr := fmt.Sprintf("%d", droneID)
	paths := strings.Split(dronePath.String, ",")
	for _, p := range paths {
		if strings.TrimSpace(p) == droneIDStr {
			return true, nil
		}
	}
	return false, nil
}

// AppendDronePath adds a drone ID to the order's drone_path (comma-delimited).
func (r *OrderRepository) AppendDronePath(ctx context.Context, orderID int64, droneID int64) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	droneIDStr := fmt.Sprintf("%d", droneID)
	_, err := r.db.ExecContext(ctx, `
UPDATE orders SET drone_path = CASE 
  WHEN drone_path IS NULL OR drone_path = '' THEN ?
  ELSE drone_path || ',' || ?
END WHERE id = ?`, droneIDStr, droneIDStr, orderID)
	return err
}

// AddDroneToPath is an alias for AppendDronePath for consistency with interfaces.
func (r *OrderRepository) AddDroneToPath(ctx context.Context, orderID int64, droneID int64) error {
	return r.AppendDronePath(ctx, orderID, droneID)
}

// Update updates an order.
func (r *OrderRepository) Update(ctx context.Context, o *models.Order) error {
	if o == nil {
		return errors.New("order is nil")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := r.db.ExecContext(ctx,
		`UPDATE orders SET origin_lat = ?, origin_lng = ?, dest_lat = ?, dest_lng = ?, status = ?, pickup_lat = ?, pickup_lng = ?, drone_path = ? WHERE id = ?`,
		o.OriginLat, o.OriginLng, o.DestLat, o.DestLng, string(o.Status), o.PickupLat, o.PickupLng, o.DronePath, o.ID)
	return err
}

// UpdateAssignedDrone updates the assigned drone for an order (via orders table if tracked).
// Note: Drone assignment is tracked in drones.assigned_job, not orders table.
// This is here for interface completeness.
func (r *OrderRepository) UpdateAssignedDrone(ctx context.Context, id int64, droneID *int64) error {
	// This is a no-op since assignment is tracked in drones table.
	// Kept for interface compatibility.
	return nil
}

// FindByAssignedDrone finds an order assigned to a specific drone.
func (r *OrderRepository) FindByAssignedDrone(ctx context.Context, droneID int64) (*models.Order, error) {
	return r.GetAssignedOrderForDrone(ctx, droneID)
}
