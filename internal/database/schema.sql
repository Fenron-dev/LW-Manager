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
    detected_type TEXT,
    storage_location TEXT,
    note TEXT,
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
    width INTEGER NOT NULL DEFAULT 0,
    height INTEGER NOT NULL DEFAULT 0,
    mime_type TEXT,
    content_hash TEXT,
    thumbnail TEXT,
    metadata TEXT,
    text_content TEXT,
    created_at TEXT,
    modified_at TEXT,
    scanned_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(drive_id, path)
);

CREATE INDEX IF NOT EXISTS idx_files_drive ON files(drive_id);
CREATE INDEX IF NOT EXISTS idx_files_extension ON files(extension);
CREATE INDEX IF NOT EXISTS idx_files_hash ON files(content_hash);

CREATE TABLE IF NOT EXISTS duplicate_preferences (
    content_hash TEXT PRIMARY KEY,
    drive_id INTEGER NOT NULL REFERENCES drives(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_duplicate_preferences_drive ON duplicate_preferences(drive_id);

CREATE TABLE IF NOT EXISTS scan_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    drive_id INTEGER NOT NULL REFERENCES drives(id) ON DELETE CASCADE,
    captured_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    file_count INTEGER NOT NULL DEFAULT 0,
    total_bytes INTEGER NOT NULL DEFAULT 0,
    protected INTEGER NOT NULL DEFAULT 0,
    note TEXT
);

CREATE TABLE IF NOT EXISTS archived_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    snapshot_id INTEGER NOT NULL REFERENCES scan_snapshots(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    filename TEXT NOT NULL,
    extension TEXT,
    size INTEGER NOT NULL DEFAULT 0,
    mime_type TEXT,
    modified_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_snapshots_drive ON scan_snapshots(drive_id);
CREATE INDEX IF NOT EXISTS idx_archived_snapshot ON archived_files(snapshot_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_archived_snapshot_path ON archived_files(snapshot_id,path);
CREATE INDEX IF NOT EXISTS idx_archived_name ON archived_files(filename);

CREATE TABLE IF NOT EXISTS storage_locations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL COLLATE NOCASE,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL COLLATE NOCASE,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS drive_tags (
    drive_id INTEGER NOT NULL REFERENCES drives(id) ON DELETE CASCADE,
    tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (drive_id, tag_id)
);

CREATE TABLE IF NOT EXISTS snapshot_tags (
    snapshot_id INTEGER NOT NULL REFERENCES scan_snapshots(id) ON DELETE CASCADE,
    tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (snapshot_id, tag_id)
);

CREATE TABLE IF NOT EXISTS file_tags (
    drive_id INTEGER NOT NULL REFERENCES drives(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    source TEXT NOT NULL DEFAULT 'manual',
    PRIMARY KEY (drive_id, path, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_drive_tags_tag ON drive_tags(tag_id);
CREATE INDEX IF NOT EXISTS idx_snapshot_tags_tag ON snapshot_tags(tag_id);
CREATE INDEX IF NOT EXISTS idx_file_tags_tag ON file_tags(tag_id);
CREATE INDEX IF NOT EXISTS idx_file_tags_file ON file_tags(drive_id,path);

CREATE TABLE IF NOT EXISTS file_ai_analyses (
    drive_id INTEGER NOT NULL REFERENCES drives(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    source_size INTEGER NOT NULL,
    source_modified TEXT NOT NULL,
    summary TEXT NOT NULL,
    tags TEXT NOT NULL,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    input_bytes INTEGER NOT NULL DEFAULT 0,
    input_truncated INTEGER NOT NULL DEFAULT 0,
	image_bytes INTEGER NOT NULL DEFAULT 0,
	vision INTEGER NOT NULL DEFAULT 0,
    analyzed_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (drive_id, path)
);

CREATE INDEX IF NOT EXISTS idx_file_ai_drive ON file_ai_analyses(drive_id);
