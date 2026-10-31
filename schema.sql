CREATE TABLE IF NOT EXISTS galleries (
    gallery_id INTEGER PRIMARY KEY,
    title TEXT,
    author TEXT,
    lang TEXT,
    dir TEXT,
    first_seen TEXT NOT NULL,
    last_seen TEXT
);

CREATE TABLE IF NOT EXISTS files (
    path TEXT PRIMARY KEY,
    gallery_id INTEGER REFERENCES galleries(gallery_id),
    sha256 TEXT NOT NULL,
    size INTEGER NOT NULL,
    ext TEXT,
    magic TEXT,
    scan_status TEXT NOT NULL CHECK (scan_status IN ('pass', 'hit', 'not_evaluated', 'external_removal', 'scan_error')),
    scan_source TEXT NOT NULL,
    scan_time TEXT NOT NULL,
    missing_at_verify INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS pageorder (
    gallery_id INTEGER NOT NULL REFERENCES galleries(gallery_id),
    page_index INTEGER NOT NULL,
    hash TEXT NOT NULL,
    PRIMARY KEY (gallery_id, page_index)
);

CREATE TABLE IF NOT EXISTS scan_cache (
    sha256 TEXT PRIMARY KEY,
    verdict TEXT NOT NULL,
    engine TEXT NOT NULL,
    threat_name TEXT,
    detail TEXT,
    cached_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS findings (
    id INTEGER PRIMARY KEY,
    path TEXT NOT NULL,
    sha256 TEXT NOT NULL,
    rule TEXT NOT NULL,
    detail TEXT,
    found_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS bookmark_queries (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    query TEXT NOT NULL,
    created_at TEXT NOT NULL,
    last_run_at TEXT,
    notify INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS read_state (
    gallery_id INTEGER PRIMARY KEY REFERENCES galleries(gallery_id),
    last_page INTEGER NOT NULL DEFAULT 0,
    fully_read INTEGER NOT NULL DEFAULT 0,
    opened_at TEXT,
    favorite INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_files_sha256 ON files(sha256);
CREATE INDEX IF NOT EXISTS idx_files_status ON files(scan_status);
CREATE INDEX IF NOT EXISTS idx_findings_sha256 ON findings(sha256);
