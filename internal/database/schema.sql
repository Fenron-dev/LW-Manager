PRAGMA foreign_keys = ON;
PRAGMA journal_mode = DELETE;
PRAGMA synchronous = NORMAL;
PRAGMA busy_timeout = 5000;

CREATE TABLE IF NOT EXISTS drives (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT UNIQUE NOT NULL,
    label TEXT,
    serial TEXT,
    vendor TEXT,
    model TEXT,
    total_size INTEGER,
    fs_type TEXT,
    vault_path TEXT,
    display_name TEXT,
    inventory_number TEXT,
    manufacturer TEXT,
    device_type TEXT,
    used_size INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    drive_id INTEGER NOT NULL REFERENCES drives(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    filename TEXT NOT NULL,
    extension TEXT,
    size INTEGER NOT NULL DEFAULT 0,
    mime_type TEXT,
    content_hash TEXT,
    thumbnail TEXT,
    metadata TEXT,
    created_at TEXT,
    modified_at TEXT,
    scanned_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(drive_id, path)
);

CREATE INDEX IF NOT EXISTS idx_files_drive ON files(drive_id);
CREATE INDEX IF NOT EXISTS idx_files_extension ON files(extension);
CREATE INDEX IF NOT EXISTS idx_files_hash ON files(content_hash);
