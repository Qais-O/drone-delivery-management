package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"droneDeliveryManagement/models"
)

// ListByUserID returns all orders for a user ordered by placement_date desc.
func (r *OrderRepository) ListByUserID(ctx context.Context, userID int64) ([]models.Order, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	rows, err := r.db.QueryContext(ctx, `SELECT id, origin_lat, origin_lng, dest_lat, dest_lng, status, placement_date, submitted_by, pickup_lat, pickup_lng, drone_path FROM orders WHERE submitted_by = ? ORDER BY placement_date DESC, id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanOrderRows(rows)
}

// Withdraw sets the status of the order to withdrawn.
func (r *OrderRepository) Withdraw(ctx context.Context, id int64) error {
	return r.UpdateStatus(ctx, id, models.OrderStatusWithdrawn)
}

// ListByUserIDPage returns a page of orders for a user ordered by placement_date desc, id desc.
// Uses keyset pagination with a numeric cursor (placement unix seconds, id).
func (r *OrderRepository) ListByUserIDPage(ctx context.Context, userID int64, pageSize int, afterSeconds int64, afterID int64) ([]models.Order, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var rows *sql.Rows
	var err error
	if afterSeconds > 0 && afterID > 0 {
		// Keyset pagination using numeric time to avoid string-format pitfalls
		rows, err = r.db.QueryContext(ctx, `
SELECT id, origin_lat, origin_lng, dest_lat, dest_lng, status, placement_date, submitted_by, pickup_lat, pickup_lng, drone_path
FROM orders
WHERE submitted_by = ?
  AND (
        CAST(strftime('%s', placement_date) AS INTEGER) < ?
        OR (CAST(strftime('%s', placement_date) AS INTEGER) = ? AND id < ?)
      )
ORDER BY placement_date DESC, id DESC
LIMIT ?`, userID, afterSeconds, afterSeconds, afterID, pageSize)
	} else {
		rows, err = r.db.QueryContext(ctx, `
SELECT id, origin_lat, origin_lng, dest_lat, dest_lng, status, placement_date, submitted_by, pickup_lat, pickup_lng, drone_path
FROM orders
WHERE submitted_by = ?
ORDER BY placement_date DESC, id DESC
LIMIT ?`, userID, pageSize)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanOrderRows(rows)
}

// ListOrdersAdminParams represents filters and pagination for ListAdmin (admin).
type ListOrdersAdminParams struct {
	Statuses      []models.OrderStatus
	SubmittedBy   *int64
	PlacementFrom *string // optional inclusive lower bound on placement_date
	PlacementTo   *string // optional inclusive upper bound on placement_date
	PageSize      int
	AfterSeconds  int64 // keyset cursor: placement_date unix seconds
	AfterID       int64 // keyset cursor: order id
}

// ListAdmin returns orders matching filters ordered by placement_date desc, id desc with keyset pagination.
func (r *OrderRepository) ListAdmin(ctx context.Context, p ListOrdersAdminParams) ([]models.Order, error) {
	if p.PageSize <= 0 {
		p.PageSize = 20
	}
	if p.PageSize > 100 {
		p.PageSize = 100
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var where []string
	var args []any

	if len(p.Statuses) > 0 {
		placeholders := make([]string, len(p.Statuses))
		for i, s := range p.Statuses {
			placeholders[i] = "?"
			args = append(args, string(s))
		}
		where = append(where, "status IN ("+strings.Join(placeholders, ",")+")")
	}
	if p.SubmittedBy != nil {
		where = append(where, "submitted_by = ?")
		args = append(args, *p.SubmittedBy)
	}
	if p.PlacementFrom != nil {
		where = append(where, "placement_date >= ?")
		args = append(args, *p.PlacementFrom)
	}
	if p.PlacementTo != nil {
		where = append(where, "placement_date <= ?")
		args = append(args, *p.PlacementTo)
	}
	if p.AfterSeconds > 0 && p.AfterID > 0 {
		where = append(where, "(CAST(strftime('%s', placement_date) AS INTEGER) < ? OR (CAST(strftime('%s', placement_date) AS INTEGER) = ? AND id < ?))")
		args = append(args, p.AfterSeconds, p.AfterSeconds, p.AfterID)
	}

	query := `SELECT id, origin_lat, origin_lng, dest_lat, dest_lng, status, placement_date, submitted_by, pickup_lat, pickup_lng, drone_path FROM orders`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY placement_date DESC, id DESC LIMIT ?"
	args = append(args, p.PageSize)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanOrderRows(rows)
}

// FindNextAvailableForReservation selects the next order available to be reserved by a drone.
// Priority: status 'to pick up' first, then 'placed'; earliest placement_date asc, then id asc.
// Excludes orders already assigned to any drone and orders which already include the requesting drone in their drone_path.
func (r *OrderRepository) FindNextAvailableForReservation(ctx context.Context, droneID int64) (*models.Order, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// LEFT JOIN to find orders with no drone currently assigned. Also exclude orders that
	// already have this drone in their drone_path using instr on a comma-padded string.
	row := r.db.QueryRowContext(ctx, `
SELECT o.id, o.origin_lat, o.origin_lng, o.dest_lat, o.dest_lng, o.status, o.placement_date, o.submitted_by, o.pickup_lat, o.pickup_lng, o.drone_path
FROM orders o
LEFT JOIN drones d ON d.assigned_job = o.id
WHERE d.id IS NULL
  AND o.status IN ('to pick up','placed')
  AND (o.drone_path IS NULL OR instr(',' || o.drone_path || ',', ',' || ? || ',') = 0)
ORDER BY CASE WHEN o.status = 'to pick up' THEN 0 ELSE 1 END, o.placement_date ASC, o.id ASC
LIMIT 1`, droneID)
	var o models.Order
	var status string
	var pickupLat, pickupLng sql.NullFloat64
	var dronePath sql.NullString
	if err := row.Scan(&o.ID, &o.OriginLat, &o.OriginLng, &o.DestLat, &o.DestLng, &status, &o.PlacementAt, &o.SubmittedBy, &pickupLat, &pickupLng, &dronePath); err != nil {
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

// GetAssignedOrderForDrone returns the order assigned to the given drone id (if any).
func (r *OrderRepository) GetAssignedOrderForDrone(ctx context.Context, droneID int64) (*models.Order, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var o models.Order
	var status string
	var pickupLat, pickupLng sql.NullFloat64
	var dronePath sql.NullString
	err := r.db.QueryRowContext(ctx, `
SELECT o.id, o.origin_lat, o.origin_lng, o.dest_lat, o.dest_lng, o.status, o.placement_date, o.submitted_by, o.pickup_lat, o.pickup_lng, o.drone_path
FROM drones d
JOIN orders o ON o.id = d.assigned_job
WHERE d.id = ?`, droneID).Scan(&o.ID, &o.OriginLat, &o.OriginLng, &o.DestLat, &o.DestLng, &status, &o.PlacementAt, &o.SubmittedBy, &pickupLat, &pickupLng, &dronePath)
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

// scanOrderRows is a helper to scan rows into Order objects.
func (r *OrderRepository) scanOrderRows(rows *sql.Rows) ([]models.Order, error) {
	var out []models.Order
	for rows.Next() {
		var o models.Order
		var status string
		var pickupLat, pickupLng sql.NullFloat64
		var dronePath sql.NullString
		if err := rows.Scan(&o.ID, &o.OriginLat, &o.OriginLng, &o.DestLat, &o.DestLng, &status, &o.PlacementAt, &o.SubmittedBy, &pickupLat, &pickupLng, &dronePath); err != nil {
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
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
