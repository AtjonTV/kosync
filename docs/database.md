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

`summary_mail` is how often the account wants a report on its own reading: `off`, `weekly` or
`monthly`. Unset means off, and unlike `achievement_mail` it is not backfilled for accounts that
predate it — nobody asked them. `summary_sent` is the last period a summary went out for, written as
`2026-W33` or `2026-07`; it is what makes the hourly job idempotent, and it is refused in an update
request from anybody but a superuser, because moving it back would ask for the same summary again.

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
language, identifiers, series, subjects, cover, word count and both document hashes are filled in by
the server as the upload arrives, and the derived ones cannot be edited afterwards. The metadata read
out of the file can be, since correcting what a publisher wrote is the owner's business.

`series` and `series_index` are the series a book belongs to and where in it this volume sits. They
are two columns rather than one string like `A Song of Ice and Fire #2`, because the catalog's series
feed has to group by the name and sort by the number, and neither is possible once the two are
spelled into one value. The index is a number, so a novella published as 1.5 keeps its place. Both
are only filled where the file says so; a book with no series is simply not on one, and the feed
leaves it out rather than inventing a shelf of one.

`subjects` is what the file says the book is about, as a JSON array, stored as the publisher wrote
it. Nothing reads it yet, deliberately: the values are of very mixed quality — on the reference
library 143 of 202 distinct subjects belong to exactly one book — so a navigation feed of them would
mostly be a list of single titles, and a hand-made collection is the better answer to the same
question. It is stored because the file says it and throwing it away would mean re-reading every
EPUB to get it back.

`hash_binary` and `hash_filename` are the two ways KOReader identifies a document. They are stored as
separate indexed columns so a progress push can be matched to a book by either. `content_hash` is a
SHA-256 of the whole file and is unique per owner, so uploading the same file twice is refused rather
than duplicated; two owners uploading the same book each keep their own copy. The refusal is checked
before the insert as well as by the index, so the answer names the book that is already there instead
of the index's own "Failed to create record.".

`hash_catalog` is a third indexed column and the odd one out: it is a filename hash again, but of the
name the OPDS catalog serves the book under rather than the name it was uploaded with. Those are
different strings — the uploader's name is whatever their file happened to be called, and the served
name is derived from the title — so a reader that downloaded from the catalog and identifies
documents by name needs its own column to be found by. It follows a rename, which does leave a device
that downloaded the book earlier holding the old name; the binary hash still covers that device.

`file_size` is how many bytes the uploaded file takes. It is stored rather than read from the
filesystem because the per-account quota needs a sum over the whole library on every upload, and a
stat call per book would get slower with every book added. The extracted cover is not counted: it is
generated, it is small, and nobody chose to store it. Books uploaded before the column existed were
measured once by the migration that added it.

`page_count` is a fallback, derived from the word count at `BOOKS_WORDS_PER_PAGE`. An EPUB is
reflowable and has no pages of its own, so a reader's own count is better wherever one can be had —
see [analytics.md](analytics.md). `measured_pages` holds it when there is one, and `measured_source`
says which of the two it is: `device` for the count a synced statistics database states outright,
`progress` for the count recovered from the size of the steps a device's progress moved in. A stated
count wins over an estimated one, and an empty source on a book that has a measurement is one taken
before the column existed, which can only have been the estimator.

`measured_device` names where an estimated count came from; a stated one leaves it empty, because
which device wrote a statistics database is not in the database. `measured_through` records how far
into the reading the measurement looked, so a book nobody has read since is not measured again.

A `documents` row carries a `book` relation, set by the server when the document's hash matches an
uploaded book. It is empty until such a book exists, and it is cleared rather than cascaded when the
book is deleted: removing a file must not remove the reading done in it.

### `book_collections`

A shelf somebody put together by hand. The name is unfortunate and unavoidable: PocketBase calls its
own tables collections too, and this is the KOsync kind.

| Field | Type | Notes |
| --- | --- | --- |
| `owner` | relation → `users` | deleting an account deletes its shelves |
| `name` | text | up to 100 characters, unique per owner |
| `description` | text | up to 1000 characters, optional |
| `books` | relation → `books`, list | the shelf itself, in the order it was built |

This is the one collection whose contents are entirely somebody's opinion. Everything else here is
read out of a file or reported by a device and so is written by the server; a shelf is made, renamed,
filled and thrown away by its owner, which is why all five rules are the owner rule and none of them
is superuser only.

The order of `books` is the shelf. A relation list keeps the order it was given, and that order is
the one thing about a collection no query could work out — *Westeros, in order* is not alphabetical,
not by upload date, and not by series index either once a spin-off is on it. So the catalog and the
web interface both re-sort the rows they read back into the stored order rather than letting SQL
choose.

Unique on `(owner, name)`, through `idx_book_collections_owner_name`: two shelves of one name are two
answers to the same question, and the index turns that into a validation error on the name the
browser can say something useful about.

The `books` relation deliberately does **not** cascade. PocketBase removes a deleted book's id from
every list that named it and only deletes the record holding the list when the list empties, so a
cascade here would mean that deleting the last book of a reading list deletes the reading list.
Somebody who clears out a shelf still has the shelf.

The ceiling of 2000 books exists because a relation field has to be told a maximum to be a list at
all — PocketBase reads a maximum of one as a single value. It is set far past any library this
serves.

What the rules cannot say is that the books on a shelf have to be the owner's own: a rule is a filter
over the record being written, and the books are a list of ids pointing somewhere else. Without that
sentence an account could put any book id at all on a shelf and read the titles back through the
relation's expansion, which is a way of asking what somebody else uploaded. So it is enforced by a
hook in [`server/internal/collections`](../server/internal/collections), which refuses a create or
update whose books are not all owned by the shelf's owner, and refuses a shelf changing owner. A book
that has been deleted meanwhile is refused the same way, which is the case that arrives on its own:
two browser tabs, one of them deleting.

Books are added and taken off one at a time, with PocketBase's own `books+` and `books-` list
modifiers, so that two open tabs cannot overwrite each other's shelf; the hook sees the merged list,
so the check still holds. Reordering is the one operation that has to send the whole list, because
there is no modifier for "third, not fifth".

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

### `page_reads`

What a device measured about its own reading: which page of which document, from when, for how long.
It arrives by importing the statistics database KOReader uploads to `/webdav` (see
[config.md](config.md)) and is the only record anywhere of *when* reading actually happened — the sync
protocol carries no clock, so everything else here infers a day from the moment a push arrived.

`document` is KOReader's `md5`, which is the same string this database calls a document hash and the
same one a book's `hash_binary` holds. That is what makes the matching exact rather than a guess at a
title. There is deliberately no relation to `documents` or `books`: a device measures reading in files
that were never pushed and never uploaded, and that reading is no less real for it. The books are
matched by a join when the days are computed, so a book uploaded next month makes last month's
measurements count towards it without anything having to be repaired.

Stored as events rather than as a daily summary for two reasons: a day depends on a timezone that can
be changed afterwards, and the events are what makes the import idempotent. The unique index on
`(owner, document, page, started_at)` is KOReader's own key, so re-importing a database that has grown
by a week inserts exactly that week.

Nothing writes these through the API — the rules allow reading your own and nothing else. A row
somebody typed in would be a measurement nothing measured.

### `reading_book_days`

The daily statistics of a single book, keyed by owner, day and book. Read only through the API, like
the other analytics collections, and computed by the same worker in the same pass. Deleting a book
takes its rows with it; the reading itself lives in `documents` and stays. See
[analytics.md](analytics.md) for why the reading time here does not add up to the day totals and the
pages do.


## Backups

PocketBase takes care of them, in the superuser interface under **Settings → Backups**, or through
its API. They can be stored locally or on S3, and can be scheduled.
