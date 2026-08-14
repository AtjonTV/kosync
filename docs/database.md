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

### `books`

An uploaded EPUB and everything read out of it. Only the file itself is supplied: the title, authors,
language, identifiers, cover, word count and both document hashes are filled in by the server as the
upload arrives, and the derived ones cannot be edited afterwards. The title and authors can, since
correcting a publisher's metadata is the owner's business.

`hash_binary` and `hash_filename` are the two ways KOReader identifies a document. They are stored as
separate indexed columns so a progress push can be matched to a book by either. `content_hash` is a
SHA-256 of the whole file and is unique per owner, so uploading the same file twice is refused rather
than duplicated; two owners uploading the same book each keep their own copy.

`page_count` is a fallback, derived from the word count at `BOOKS_WORDS_PER_PAGE`. An EPUB is
reflowable and has no pages of its own, so a count measured from a reader's own progress is better
wherever one can be had — see [analytics.md](analytics.md). `measured_pages` holds that measurement
when there is one, with `measured_device` naming where it came from and `measured_through` recording
how far into the reading it looked, so a book nobody has read since is not measured again.

### `reading_book_days`

The daily statistics of a single book, keyed by owner, day and book. Read only through the API, like
the other analytics collections, and computed by the same worker in the same pass. Deleting a book
takes its rows with it; the reading itself lives in `documents` and stays. See
[analytics.md](analytics.md) for why the reading time here does not add up to the day totals and the
pages do.

A `documents` row carries a `book` relation, set by the server when the document's hash matches an
uploaded book. It is empty until such a book exists, and it is cleared rather than cascaded when the
book is deleted: removing a file must not remove the reading done in it.

## Backups

PocketBase takes care of them, in the superuser interface under **Settings → Backups**, or through
its API. They can be stored locally or on S3, and can be scheduled.
