CREATE TABLE IF NOT EXISTS drones (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  serial_number TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL DEFAULT '',
  lat REAL NOT NULL,
  lng REAL NOT NULL,
  speed_mph REAL NOT NULL,
  assigned_job INTEGER UNIQUE,
  status TEXT NOT NULL DEFAULT 'fixed' CHECK (status IN ('fixed','broken')),
  drone_path TEXT NULL,
  FOREIGN KEY(assigned_job) REFERENCES orders(id) ON DELETE SET NULL
);
