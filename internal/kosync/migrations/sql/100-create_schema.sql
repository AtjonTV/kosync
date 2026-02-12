CREATE TABLE IF NOT EXISTS schema_versions (
   version INTEGER PRIMARY KEY,
   installed_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
     id TEXT PRIMARY KEY,
     username TEXT UNIQUE,
     password TEXT,
     created_at INTEGER NOT NULL,
     updated_at INTEGER,
     deleted_at INTEGER
);

CREATE TABLE IF NOT EXISTS documents (
     id TEXT NOT NULL,
     owner_id TEXT NOT NULL,

     title TEXT,
     current_location TEXT,
     progress FLOAT,
     last_read_on_device TEXT,
     last_read_on_device_id TEXT,
     last_read_at INTEGER,

     created_at INTEGER NOT NULL,
     updated_at INTEGER,
     deleted_at INTEGER,
     PRIMARY KEY (id, owner_id)
);

CREATE TABLE IF NOT EXISTS document_history (
    id TEXT NOT NULL,
    owner_id TEXT NOT NULL,
    last_read_at INTEGER NOT NULL,

    title TEXT,
    current_location TEXT,
    progress FLOAT,
    last_read_on_device TEXT,
    last_read_on_device_id TEXT,

    created_at INTEGER NOT NULL,
    updated_at INTEGER,
    deleted_at INTEGER,
    PRIMARY KEY (id, owner_id, last_read_at)
);
