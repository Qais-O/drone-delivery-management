package models

// DroneStatus represents the health status of a drone.
type DroneStatus string

const (
	DroneStatusFixed  DroneStatus = "fixed"
	DroneStatusBroken DroneStatus = "broken"
)

// Drone represents a delivery drone.
// assigned_job has a one-to-one relation to Order (nullable when unassigned).
type Drone struct {
	ID           int64       `db:"id" json:"id"`
	Name         string      `db:"name" json:"name"`
	SerialNumber string      `db:"serial_number" json:"serial_number"`
	Lat          float64     `db:"lat" json:"lat"`
	Lng          float64     `db:"lng" json:"lng"`
	SpeedMPH     float64     `db:"speed_mph" json:"speed_mph"`
	AssignedJob  *int64      `db:"assigned_job" json:"assigned_job"`
	Status       DroneStatus `db:"status" json:"status"`
}
