CREATE TABLE IF NOT EXISTS orders (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  origin_lat REAL NOT NULL,
  origin_lng REAL NOT NULL,
  dest_lat REAL NOT NULL,
  dest_lng REAL NOT NULL,
  status TEXT NOT NULL DEFAULT 'placed' CHECK (status IN ('placed','delivered','en route','failed','to pick up','withdrawn')),
  placement_date DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP),
  submitted_by INTEGER NOT NULL,
  pickup_lat REAL NULL,
  pickup_lng REAL NULL,
  drone_path TEXT NULL,
  FOREIGN KEY(submitted_by) REFERENCES users(id) ON DELETE CASCADE
);
