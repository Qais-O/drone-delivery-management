package models

// OrderStatus represents the current progress of an order.
type OrderStatus string

const (
	OrderStatusPlaced    OrderStatus = "placed"
	OrderStatusDelivered OrderStatus = "delivered"
	OrderStatusEnRoute   OrderStatus = "en route"
	OrderStatusFailed    OrderStatus = "failed"
	OrderStatusToPickUp  OrderStatus = "to pick up"
	OrderStatusWithdrawn OrderStatus = "withdrawn"
)

// Order represents a delivery order with a one-to-one relation to User via SubmittedBy.
type Order struct {
	ID          int64       `db:"id" json:"id"`
	OriginLat   float64     `db:"origin_lat" json:"origin_lat"`
	OriginLng   float64     `db:"origin_lng" json:"origin_lng"`
	DestLat     float64     `db:"dest_lat" json:"dest_lat"`
	DestLng     float64     `db:"dest_lng" json:"dest_lng"`
	SubmittedBy int64       `db:"submitted_by" json:"submitted_by"`
	Status      OrderStatus `db:"status" json:"status"`
	PlacementAt string      `db:"placement_date" json:"placement_date"`
	// Pickup location is used when an in-flight order needs handoff (drone broken).
	// They are nullable in DB; use pointers to distinguish null vs zero.
	PickupLat *float64 `db:"pickup_lat" json:"pickup_lat,omitempty"`
	PickupLng *float64 `db:"pickup_lng" json:"pickup_lng,omitempty"`
	// DronePath is a comma-delimited string of drone IDs that have handled this order.
	// Used to prevent the same drone from being assigned to the same order twice.
	DronePath string `db:"drone_path" json:"drone_path,omitempty"`
}
