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

`achievement_mail` is whether the account wants to be told by mail about what it earns. It is
positive rather than a mute switch, because a boolean is false when it has never been set and for
unsolicited mail the safe end of that is silence — so an account created outside the browser is quiet
until somebody ticks the box. See [config.md](config.md).

`timezone` holds an IANA name such as `Europe/Vienna`, and is what the statistics days are reckoned
in. The browser supplies it at registration, because nothing else can: the KOReader protocol carries
no clock. It defaults to `UTC`, and changing it requeues every day the account has ever read — see
[analytics.md](analytics.md).

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
| `filename`, `authors` | text | what the device says the file is, when it is set to send metadata |
| `filename_hash` | text | the KOReader filename hash of `filename`, indexed so a book can be matched to it |

Unique on `(owner, document)`: the same book synced by two people are two records.

### `document_history`

Every position a document has left behind, written by the server whenever the current state is
replaced. Owners can read and delete their entries; nobody can write or edit one through the API.

### `document_aliases`

The document hashes that used to be documents of their own and were merged into another one.

| Field | Type | Notes |
| --- | --- | --- |
| `owner` | relation → `users` | |
| `document` | text | the retired hash |
| `document_ref` | relation → `documents` | what it resolves to now; cascades |

Unique on `(owner, document)`, the same as `documents`: one owner, one meaning per hash. A device
carries on sending the hash it has always sent, and this is what lets that push land on the document
the reading was folded into instead of quietly rebuilding the one that was merged away.

Written by the merge, never by a client, so there is no create rule and no update rule. Deleting one
is allowed and is the way back: drop the alias and the next push from that device makes its own
document again.

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

`hash_catalog` is a third indexed column and the odd one out: it is a filename hash again, but of the
name the OPDS catalog serves the book under rather than the name it was uploaded with. Those are
different strings — the uploader's name is whatever their file happened to be called, and the served
name is derived from the title — so a reader that downloaded from the catalog and identifies
documents by name needs its own column to be found by. It follows a rename, which does leave a device
that downloaded the book earlier holding the old name; the binary hash still covers that device.

`page_count` is a fallback, derived from the word count at `BOOKS_WORDS_PER_PAGE`. An EPUB is
reflowable and has no pages of its own, so a count measured from a reader's own progress is better
wherever one can be had — see [analytics.md](analytics.md). `measured_pages` holds that measurement
when there is one, with `measured_device` naming where it came from and `measured_through` recording
how far into the reading it looked, so a book nobody has read since is not measured again.

A `documents` row carries a `book` relation, set by the server when the document's hash matches an
uploaded book. It is empty until such a book exists, and it is cleared rather than cascaded when the
book is deleted: removing a file must not remove the reading done in it.

### `devices`

One row per device that has pushed progress, per owner.

| Field | Type | Notes |
| --- | --- | --- |
| `owner` | relation → `users` | |
| `device_id` | text | KOReader's own identifier; everything else groups by this, because it survives a rename |
| `reported_name` | text | what the device last called itself, refreshed on every push |
| `name` | text | what its owner calls it; seeded from the reported name and never overwritten afterwards |
| `last_seen` | date | only ever moves forwards, so imported history cannot make a device look retired |

Unique on `(owner, device_id)`: the same physical device used from two accounts is two rows, because
each owner names it for themselves. Rows are created by the server as pushes arrive — a device exists
because it synced, so `create` and `delete` are superuser only and only `name` may be edited.

This is deliberately not `koreader_accounts.label`. A credential and a device are not the same thing:
one credential can be used from several devices, and the statistics group by device.

### `achievements`

What an account has been recognised for.

| Field | Type | Notes |
| --- | --- | --- |
| `owner` | relation → `users` | |
| `rule` | text | the rule's slug, such as `lap-warmer` |
| `tier` | number | 1, 2 or 3 |
| `value` | number | what the measure stood at when the tier was crossed |
| `earned_at` | date | when it was noticed, which is not quite when it was reached |

Unique on `(owner, rule, tier)`. Read only through the API: an achievement that could be granted from
the browser would not be worth having, and one that could be deleted would make the rule below a lie.

**Nothing is ever revoked.** Every measure is recomputed from live data, and live data moves
backwards — history gets deleted, a re-read puts progress back to the start, and the retention window
eventually removes the daily rows a streak was counted from. An achievement records that something
happened, and it having happened does not stop being true.

The rules themselves are not stored. They are code, in
[`server/internal/achievements`](../server/internal/achievements), because each one is a question
only code can ask, and the web interface reads them from `/api/kosync/achievements` rather than
keeping a copy. There are eight; [api.md](api.md) lists them with their thresholds.

### `reading_book_days`

The daily statistics of a single book, keyed by owner, day and book. Read only through the API, like
the other analytics collections, and computed by the same worker in the same pass. Deleting a book
takes its rows with it; the reading itself lives in `documents` and stays. See
[analytics.md](analytics.md) for why the reading time here does not add up to the day totals and the
pages do.


## Backups

PocketBase takes care of them, in the superuser interface under **Settings → Backups**, or through
its API. They can be stored locally or on S3, and can be scheduled.
