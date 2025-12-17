package models

// User represents an end user in the system.
// It maps to the `users` table in SQLite.
type User struct {
	ID       int64  `db:"id" json:"id"`
	Username string `db:"username" json:"username"`
	Role     string `db:"role" json:"role"`
}
