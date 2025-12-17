package repository

import (
	"context"

	"droneDeliveryManagement/models"
)

// UserRepository defines operations on User entities.
type UserRepositoryI interface {
	Create(ctx context.Context, username string) (*models.User, error)
	GetByUsername(ctx context.Context, username string) (*models.User, error)
	GetByID(ctx context.Context, id int64) (*models.User, error)
	List(ctx context.Context, limit, offset int) ([]*models.User, error)
}

// OrderRepository defines operations on Order entities.
type OrderRepositoryI interface {
	Create(ctx context.Context, o *models.Order) (*models.Order, error)
	GetByID(ctx context.Context, id int64) (*models.Order, error)
	GetByUserID(ctx context.Context, userID int64) (*models.Order, error)
	Update(ctx context.Context, o *models.Order) error
	UpdateStatus(ctx context.Context, id int64, status models.OrderStatus) error
	UpdateAssignedDrone(ctx context.Context, id int64, droneID *int64) error
	UpdatePickupLocation(ctx context.Context, id int64, lat, lng float64) error
	AddDroneToPath(ctx context.Context, orderID int64, droneID int64) error
	FindNextAvailableForReservation(ctx context.Context, droneID int64) (*models.Order, error)
	FindByAssignedDrone(ctx context.Context, droneID int64) (*models.Order, error)
}

// DroneRepository defines operations on Drone entities.
type DroneRepositoryI interface {
	Create(ctx context.Context, d *models.Drone) (*models.Drone, error)
	GetByID(ctx context.Context, id int64) (*models.Drone, error)
	GetBySerial(ctx context.Context, serial string) (*models.Drone, error)
	GetByName(ctx context.Context, name string) (*models.Drone, error)
	Update(ctx context.Context, d *models.Drone) error
	UpdateStatus(ctx context.Context, id int64, status models.DroneStatus) error
	UpdateLocation(ctx context.Context, id int64, lat, lng float64) error
	AssignJob(ctx context.Context, droneID, orderID int64) error
	ClearAssignedJob(ctx context.Context, droneID int64) error
	List(ctx context.Context, limit, offset int) ([]*models.Drone, error)
}
