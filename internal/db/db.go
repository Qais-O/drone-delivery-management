package db

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	stdfs "io/fs"
	"regexp"
	"sort"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// Open opens (or creates) a local SQLite database file and applies pending migrations.
// It uses versioned .sql files under internal/db/migrations following the pattern:
//
//	0001_name.up.sql / 0001_name.down.sql
//
// Only new migrations are applied. Use RollbackLast to revert the last applied migration.
func Open(path string) (*sql.DB, error) {
	if path == "" {
		path = "app.db"
	}
	d, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	if err := d.Ping(); err != nil {
		_ = d.Close()
		return nil, err
	}
	// Pragmas for robustness
	// journal_mode may not be supported in some contexts (e.g., in-memory). Ignore errors.
	_, _ = d.Exec(`PRAGMA journal_mode=WAL`)
	if _, err := d.Exec(`PRAGMA busy_timeout=5000`); err != nil {
		_ = d.Close()
		return nil, err
	}
	if _, err := d.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		_ = d.Close()
		return nil, err
	}
	if err := applyMigrations(d); err != nil {
		_ = d.Close()
		return nil, err
	}
	return d, nil
}

// RollbackLast rolls back the most recently applied migration, if its down script exists.
func RollbackLast(d *sql.DB) error {
	if d == nil {
		return errors.New("nil db")
	}
	if err := ensureMigrationsTable(d); err != nil {
		return err
	}
	var version int
	err := d.QueryRow(`SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1`).Scan(&version)
	if err == sql.ErrNoRows {
		return nil // nothing to rollback
	} else if err != nil {
		return err
	}
	migs, err := loadMigrations()
	if err != nil {
		return err
	}
	m, ok := migs[version]
	if !ok || m.downFile == "" {
		return fmt.Errorf("no down migration found for version %d", version)
	}
	sqlText, err := migrationsFS.ReadFile(m.downFile)
	if err != nil {
		return err
	}
	text := string(sqlText)
	if strings.HasPrefix(strings.TrimSpace(text), "-- NO_TX") {
		// Execute as-is without wrapping in a transaction
		if _, err := d.Exec(text); err != nil {
			return err
		}
		if _, err := d.Exec(`DELETE FROM schema_migrations WHERE version = ?`, version); err != nil {
			return err
		}
		return nil
	}
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(text); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.Exec(`DELETE FROM schema_migrations WHERE version = ?`, version); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

//go:embed migrations/*.sql
var migrationsFS embed.FS

type migration struct {
	version  int
	name     string
	upFile   string // path inside embedded FS
	downFile string // path inside embedded FS
}

var migFileRe = regexp.MustCompile(`^([0-9]{4})_(.+)\.(up|down)\.sql$`)

func loadMigrations() (map[int]migration, error) {
	entries := map[int]migration{}
	list, err := stdfs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		// if directory missing, just return empty set
		return entries, nil
	}
	for _, de := range list {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		m := migFileRe.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		verStr, migName, kind := m[1], m[2], m[3]
		var ver int
		if _, err := fmt.Sscanf(verStr, "%04d", &ver); err != nil {
			continue
		}
		item := entries[ver]
		item.version = ver
		item.name = migName
		p := "migrations/" + name
		if kind == "up" {
			item.upFile = p
		} else {
			item.downFile = p
		}
		entries[ver] = item
	}
	return entries, nil
}

func ensureMigrationsTable(d *sql.DB) error {
	_, err := d.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
        version INTEGER PRIMARY KEY,
        applied_at TEXT NOT NULL DEFAULT (CURRENT_TIMESTAMP)
    )`)
	return err
}

func appliedVersions(d *sql.DB) (map[int]bool, error) {
	if err := ensureMigrationsTable(d); err != nil {
		return nil, err
	}
	rows, err := d.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	got := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		got[v] = true
	}
	return got, rows.Err()
}

func applyMigrations(d *sql.DB) error {
	migs, err := loadMigrations()
	if err != nil {
		return err
	}
	if len(migs) == 0 {
		// nothing to do
		return nil
	}
	applied, err := appliedVersions(d)
	if err != nil {
		return err
	}
	// order versions
	versions := make([]int, 0, len(migs))
	for v := range migs {
		versions = append(versions, v)
	}
	sort.Ints(versions)
	for _, v := range versions {
		if applied[v] {
			continue
		}
		m := migs[v]
		if strings.TrimSpace(m.upFile) == "" {
			return fmt.Errorf("missing up migration for version %04d", v)
		}
		sqlText, err := migrationsFS.ReadFile(m.upFile)
		if err != nil {
			return err
		}
		text := string(sqlText)
		if strings.HasPrefix(strings.TrimSpace(text), "-- NO_TX") {
			// Execute as-is without wrapping in a transaction
			if _, err := d.Exec(text); err != nil {
				return fmt.Errorf("migration %04d failed: %w", v, err)
			}
			if _, err := d.Exec(`INSERT INTO schema_migrations(version) VALUES(?)`, v); err != nil {
				return err
			}
			continue
		}
		tx, err := d.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(text); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %04d failed: %w", v, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version) VALUES(?)`, v); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
