CREATE TABLE document_history_old AS SELECT * FROM document_history;

DROP TABLE document_history;
CREATE TABLE document_history (
  document_id TEXT NOT NULL,
  owner_id TEXT NOT NULL,

  title TEXT,
  current_location TEXT,
  progress FLOAT,
  last_read_on_device TEXT,
  last_read_on_device_id TEXT,
  last_read_at INTEGER,

  created_at INTEGER,
  updated_at INTEGER,
  deleted_at INTEGER
);

INSERT INTO document_history (
      document_id,
      owner_id,
      title,
      current_location,
      progress,
      last_read_on_device,
      last_read_on_device_id,
      last_read_at,
      created_at,
      updated_at,
      deleted_at
)
SELECT id as document_id,
   owner_id,
   title,
   current_location,
   progress,
   last_read_on_device,
   last_read_on_device_id,
   last_read_at,
   created_at,
   updated_at,
   deleted_at
FROM document_history_old;

DROP TABLE document_history_old;
