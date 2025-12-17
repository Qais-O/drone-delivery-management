package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"droneDeliveryManagement/models"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create inserts a new user with the given username.
// Returns the created User with its generated ID. Role defaults to 'end user'.
func (r *UserRepository) Create(ctx context.Context, username string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	res, err := r.db.ExecContext(ctx, `INSERT INTO users (username) VALUES (?)`, username)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &models.User{ID: id, Username: username, Role: "end user"}, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*models.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var u models.User
	err := r.db.QueryRowContext(ctx, `SELECT id, username, role FROM users WHERE id = ?`, id).Scan(&u.ID, &u.Username, &u.Role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var u models.User
	err := r.db.QueryRowContext(ctx, `SELECT id, username, role FROM users WHERE username = ?`, username).Scan(&u.ID, &u.Username, &u.Role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) List(ctx context.Context, limit, offset int) ([]models.User, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, `SELECT id, username, role FROM users ORDER BY id LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *UserRepository) Delete(ctx context.Context, id int64) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	_, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	return err
}

// UpdateRoleByUsername sets the role for the given username.
// Intended for administrative flows and tests.
func (r *UserRepository) UpdateRoleByUsername(ctx context.Context, username, role string) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := r.db.ExecContext(ctx, `UPDATE users SET role = ? WHERE username = ?`, role, username)
	return err
}
