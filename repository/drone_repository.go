package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"droneDeliveryManagement/models"
)

type DroneRepository struct {
	db *sql.DB
}

func NewDroneRepository(db *sql.DB) *DroneRepository {
	return &DroneRepository{db: db}
}

// Create inserts a new drone. Status defaults to 'fixed' if empty.
func (r *DroneRepository) Create(ctx context.Context, d *models.Drone) (*models.Drone, error) {
	if d == nil {
		return nil, errors.New("drone is nil")
	}
	if d.Status == "" {
		d.Status = models.DroneStatusFixed
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var assigned any
	if d.AssignedJob == nil {
		assigned = nil
	} else {
		assigned = *d.AssignedJob
	}

	res, err := r.db.ExecContext(ctx, `INSERT INTO drones (serial_number, lat, lng, speed_mph, assigned_job, status, name) VALUES (?,?,?,?,?,?,?)`,
		d.SerialNumber, d.Lat, d.Lng, d.SpeedMPH, assigned, string(d.Status), d.Name)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	d.ID = id
	return d, nil
}

func (r *DroneRepository) GetByID(ctx context.Context, id int64) (*models.Drone, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var d models.Drone
	var status string
	var assigned sql.NullInt64
	err := r.db.QueryRowContext(ctx, `SELECT id, serial_number, lat, lng, speed_mph, assigned_job, status, name FROM drones WHERE id = ?`, id).
		Scan(&d.ID, &d.SerialNumber, &d.Lat, &d.Lng, &d.SpeedMPH, &assigned, &status, &d.Name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if assigned.Valid {
		v := assigned.Int64
		d.AssignedJob = &v
	}
	d.Status = models.DroneStatus(status)
	return &d, nil
}

func (r *DroneRepository) GetBySerial(ctx context.Context, serial string) (*models.Drone, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var d models.Drone
	var status string
	var assigned sql.NullInt64
	err := r.db.QueryRowContext(ctx, `SELECT id, serial_number, lat, lng, speed_mph, assigned_job, status, name FROM drones WHERE serial_number = ?`, serial).
		Scan(&d.ID, &d.SerialNumber, &d.Lat, &d.Lng, &d.SpeedMPH, &assigned, &status, &d.Name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if assigned.Valid {
		v := assigned.Int64
		d.AssignedJob = &v
	}
	d.Status = models.DroneStatus(status)
	return &d, nil
}

// GetByName fetches a drone by its name.
func (r *DroneRepository) GetByName(ctx context.Context, name string) (*models.Drone, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var d models.Drone
	var status string
	var assigned sql.NullInt64
	err := r.db.QueryRowContext(ctx, `SELECT id, serial_number, lat, lng, speed_mph, assigned_job, status, name FROM drones WHERE name = ?`, name).
		Scan(&d.ID, &d.SerialNumber, &d.Lat, &d.Lng, &d.SpeedMPH, &assigned, &status, &d.Name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if assigned.Valid {
		v := assigned.Int64
		d.AssignedJob = &v
	}
	d.Status = models.DroneStatus(status)
	return &d, nil
}

func (r *DroneRepository) GetByOrderID(ctx context.Context, orderID int64) (*models.Drone, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var d models.Drone
	var status string
	var assigned sql.NullInt64
	err := r.db.QueryRowContext(ctx, `SELECT id, serial_number, lat, lng, speed_mph, assigned_job, status, name FROM drones WHERE assigned_job = ?`, orderID).
		Scan(&d.ID, &d.SerialNumber, &d.Lat, &d.Lng, &d.SpeedMPH, &assigned, &status, &d.Name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if assigned.Valid {
		v := assigned.Int64
		d.AssignedJob = &v
	}
	d.Status = models.DroneStatus(status)
	return &d, nil
}

func (r *DroneRepository) UpdateLocationAndSpeed(ctx context.Context, id int64, lat, lng, speed float64) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := r.db.ExecContext(ctx, `UPDATE drones SET lat = ?, lng = ?, speed_mph = ? WHERE id = ?`, lat, lng, speed, id)
	return err
}

func (r *DroneRepository) UpdateStatus(ctx context.Context, id int64, status models.DroneStatus) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := r.db.ExecContext(ctx, `UPDATE drones SET status = ? WHERE id = ?`, string(status), id)
	return err
}

func (r *DroneRepository) AssignJob(ctx context.Context, id int64, orderID int64) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := r.db.ExecContext(ctx, `UPDATE drones SET assigned_job = ? WHERE id = ?`, orderID, id)
	return err
}

func (r *DroneRepository) UnassignJob(ctx context.Context, id int64) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := r.db.ExecContext(ctx, `UPDATE drones SET assigned_job = NULL WHERE id = ?`, id)
	return err
}

func (r *DroneRepository) Delete(ctx context.Context, id int64) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := r.db.ExecContext(ctx, `DELETE FROM drones WHERE id = ?`, id)
	return err
}

// ListDronesAdminParams contains filters and pagination for admin GetDrones.
type ListDronesAdminParams struct {
	Status               *models.DroneStatus
	AssignedOnly         *bool
	UnassignedOnly       *bool
	NameOrSerialContains *string
	PageSize             int
	AfterID              int64
}

// ListAdmin returns drones matching filters ordered by id asc with keyset pagination by id.
func (r *DroneRepository) ListAdmin(ctx context.Context, p ListDronesAdminParams) ([]models.Drone, error) {
	if p.PageSize <= 0 {
		p.PageSize = 20
	}
	if p.PageSize > 100 {
		p.PageSize = 100
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	where := make([]string, 0, 4)
	args := make([]any, 0, 6)

	if p.Status != nil {
		where = append(where, "status = ?")
		args = append(args, string(*p.Status))
	}
	if p.AssignedOnly != nil && *p.AssignedOnly {
		where = append(where, "assigned_job IS NOT NULL")
	}
	if p.UnassignedOnly != nil && *p.UnassignedOnly {
		where = append(where, "assigned_job IS NULL")
	}
	if p.NameOrSerialContains != nil && strings.TrimSpace(*p.NameOrSerialContains) != "" {
		like := "%" + strings.TrimSpace(*p.NameOrSerialContains) + "%"
		where = append(where, "(name LIKE ? OR serial_number LIKE ?)")
		args = append(args, like, like)
	}
	if p.AfterID > 0 {
		where = append(where, "id > ?")
		args = append(args, p.AfterID)
	}

	query := "SELECT id, serial_number, lat, lng, speed_mph, assigned_job, status, name FROM drones"
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY id ASC LIMIT ?"
	args = append(args, p.PageSize)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Drone
	for rows.Next() {
		var d models.Drone
		var status string
		var assigned sql.NullInt64
		if err := rows.Scan(&d.ID, &d.SerialNumber, &d.Lat, &d.Lng, &d.SpeedMPH, &assigned, &status, &d.Name); err != nil {
			return nil, err
		}
		if assigned.Valid {
			v := assigned.Int64
			d.AssignedJob = &v
		}
		d.Status = models.DroneStatus(status)
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
