# Database

Since version `2026.05.0` KOSync uses a SQLite database for data and an environment file for configuration.

The `kosync.db` has the following Schema:

```sql
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
```

The `kosync.env` can have the following values:
```
LISTEN_ADDRESS=:8080
ENABLE_DEBUG_LOG=true
DISABLE_REGISTRATION=false
ENABLE_WEBUI=false

```
