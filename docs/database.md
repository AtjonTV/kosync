# Database

KOsync stores everything in the PocketBase data directory (`--dir`, `/pb_data` in the container):
the SQLite database, the uploads and the backups.

The schema is defined in Go, in [`server/internal/migrations`](../server/internal/migrations), and is
applied automatically before the server starts serving. Editing collections by hand in the superuser
interface works for experimenting, but the next deployment applies the migrations again, so schema
changes belong in a migration.

## Collections

### `users`

The PocketBase auth collection, used for the web interface. Email is required, because account
recovery goes through it. Rules: everybody may register (unless `DISABLE_REGISTRATION` is set),
everybody may only see and change themselves.

### `koreader_accounts`

The credentials a device signs in with.

| Field | Type | Notes |
| --- | --- | --- |
| `username` | text | unique across the server, this is the KOReader username |
| `password` | password | the MD5 digest KOReader sends, hashed with bcrypt by PocketBase |
| `owner` | relation → `users` | deleting an account deletes its credentials |
| `label` | text | free text, shown in the web interface |
| `disabled` | bool | revoke a device without losing its history |
| `last_used` | date | when a device last authenticated |

Authentication through the PocketBase auth API is switched off for this collection, so a device
credential can never become an API session. Creating one and changing its password go through
[`/api/kosync`](api.md); the collection API refuses both.

### `documents`

The current reading position, one record per owner and document hash.

| Field | Type | Notes |
| --- | --- | --- |
| `owner` | relation → `users` | |
| `document` | text | the hash KOReader computes for the file |
| `title` | text | editable in the web interface |
| `current_location` | text | the xpointer or page fragment KOReader sent |
| `progress` | number | 0..1 |
| `last_device`, `last_device_id` | text | |
| `last_read_at` | date | |
| `source_account` | relation → `koreader_accounts` | which credential pushed last; deleting it keeps the document |

Unique on `(owner, document)`: the same book synced by two people are two records.

### `document_history`

Every position a document has left behind, written by the server whenever the current state is
replaced. Owners can read and delete their entries; nobody can write or edit one through the API.

### `reading_days`

One precomputed row per owner and day. See [analytics.md](analytics.md).

### `reading_months`

The monthly totals that aged out days are folded into.

### `analytics_queue`

Internal bookkeeping: the days waiting to be recomputed. Invisible to accounts.

## Backups

PocketBase takes care of them, in the superuser interface under **Settings → Backups**, or through
its API. They can be stored locally or on S3, and can be scheduled.
