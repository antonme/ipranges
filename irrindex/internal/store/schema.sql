PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS sources (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  url TEXT NOT NULL DEFAULT '',
  fetched_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_modified TEXT,
  sha256 TEXT NOT NULL DEFAULT '',
  size_bytes INTEGER NOT NULL DEFAULT 0,
  ok INTEGER NOT NULL DEFAULT 1,
  error TEXT
);

CREATE TABLE IF NOT EXISTS objects (
  id INTEGER PRIMARY KEY,
  source_id INTEGER NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
  object_type TEXT NOT NULL,
  family INTEGER NOT NULL,
  prefix TEXT,
  start_ip TEXT,
  end_ip TEXT,
  peer_count INTEGER
);

CREATE TABLE IF NOT EXISTS attrs (
  object_id INTEGER NOT NULL REFERENCES objects(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  value TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS objects_source_family_prefix_idx ON objects(source_id, family, prefix);
CREATE INDEX IF NOT EXISTS attrs_name_value_object_idx ON attrs(name, value, object_id);
