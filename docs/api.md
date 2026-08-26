# API

KOsync has four groups of endpoints.

## 1. The KOReader protocol, under `/koreader`

These exist for devices. They authenticate with the two headers KOReader sends and never accept a
PocketBase token.

| Method | Route | Description |
| --- | --- | --- |
| GET | `/koreader/users/auth` | Verify the device credentials. `200` with `{"authorized":"OK"}` or `401`. |
| POST | `/koreader/users/create` | Always `402`. Accounts are created in the web interface. |
| PUT | `/koreader/syncs/progress` | Store a progress push. |
| GET | `/koreader/syncs/progress/{document}` | Read the stored progress of one document. |

Authentication headers:

| Header | Value |
| --- | --- |
| `x-auth-user` | the KOReader username |
| `x-auth-key` | the MD5 hex digest of the KOReader password |

A push looks like this:

```http
PUT /koreader/syncs/progress
x-auth-user: alice-kobo
x-auth-key: 5f4dcc3b5aa765d61d8327deb882cf99
Content-Type: application/json

{
  "document": "043f11771ef9d191364ac0ba08198d36",
  "progress": "/body/DocFragment[3]/body/div/p[12]/text().0",
  "percentage": 0.42,
  "device": "Kobo Clara",
  "device_id": "BDD3C5BCA1624FE996EB00FC7948468E"
}
```

and is answered with `{"document": "...", "timestamp": 1772366400}`.

A pull answers with the same fields plus `timestamp`, in **Unix seconds**. Legacy KOsync returned its
internal 1/10000 second unit here, which no KOReader build expects; a client written against that
quirk needs adjusting.

A percentage outside 0..1 is clamped rather than refused, so a rounding artefact on the device does
not cost the reader their push.

### Document metadata

KOReader has a setting called **"Send document metadata"**, off by default, which adds a `metadata`
object to every push:

```json
{
  "document": "043f11771ef9d191364ac0ba08198d36",
  "percentage": 0.42,
  "metadata": {
    "filename": "Metro 2033.epub",
    "title": "Metro 2033",
    "authors": "Dmitry Glukhovsky"
  }
}
```

The official sync server ignores it; KOsync uses it, and it is worth turning on. It is what gives a
name to the documents that never match an uploaded book — the ones the documents page exists for,
which otherwise have nothing to be called but a 32 character hash.

Three things happen with it, and the difference between them matters:

- **`filename` is recorded on every push.** It describes the file as it is on the device now, so a
  rename there shows here.
- **`title` is only ever filled in, never replaced.** The title is the one thing on a document a
  person can edit, and a device that keeps sending the publisher's title must not undo a rename on
  the next sync.
- **The filename is hashed the way KOReader hashes one**, into `filename_hash`, and a book stored
  under that hash is matched to the document. This is an exact comparison against an indexed column,
  not a guess at a title. It also runs when the name first arrives rather than only when the document
  is created, so turning the setting on links what is already there.

Nothing is ever cleared: an absent field means the device did not say, not that the answer is
nothing.

### Timezones, or the lack of one

The protocol carries no clock. There is no timestamp in the body, and the only headers are `accept`
and the two authentication ones — and an HTTP `Date` would be GMT anyway. The timestamp KOsync stores
is `time.Now().UTC()` at the moment the push lands.

That is why the reader's timezone is a property of the *account*, taken from the browser at
registration. See [analytics.md](analytics.md) for what it changes.

## 2. The PocketBase collection API, under `/api/collections`

This is what the web interface uses for everything that is plain reading and writing of records:
listing documents, renaming one, deleting a history entry, reading statistics, registering, signing
in, resetting a password, and subscribing to live updates. It is documented by
[PocketBase](https://pocketbase.io/docs/api-records/); the collections and their rules are described
in [database.md](database.md).

Live updates use PocketBase realtime, which applies the same rules as the REST API:

```js
pb.collection('documents').subscribe('*', handler)
pb.collection('reading_days').subscribe('*', handler)
```

### Uploading a book

Books are created the same way, as a multipart record create. Only the file and the owner are sent:
the server reads the EPUB as it arrives and fills in the title, authors, language, identifiers,
description, series, subjects, cover, word count and both KOReader document hashes. The derived
numbers and hashes are refused on update and ignored in favour of the file on create; the metadata
read out of the file — the title, the authors, the language, the description, the series and its
index, the subjects — can be corrected afterwards, because what a publisher wrote is not always what
the shelf should say.

`description` arrives as plain text with a blank line between paragraphs. The publisher's markup is
stripped on the way in, so a client can put the value straight into a text node; nothing here needs
an HTML sanitiser.

```js
const form = new FormData()
form.append('owner', pb.authStore.record.id)
form.append('file', epubFile)

await pb.collection('books').create(form)
```

Both the book and its cover are **protected files**: PocketBase serves them only to a request that
carries a file token, and checks the collection's own view rule against it, so a stored EPUB is no
more public than the record it belongs to. The token is obtained from `POST /api/files/token` and
goes in the address, because an `<img>` cannot send an `Authorization` header:

```js
const token = await pb.files.getToken()

pb.files.getURL(book, book.cover, { thumb: '200x300', token })
// /api/files/books/{id}/{cover}?thumb=200x300&token=...
```

The token lasts half an hour. It is deliberately longer than PocketBase's own three minutes: the
address of a protected file carries the token, so each renewal changes the address of every cover on
a page and the browser fetches all of them again. Thumbnails are still generated on first request.

The page count is derived too, and in three ways: `measured_pages` is the device's own count, with
`measured_source` saying whether the device stated it in the statistics it synced (`device`) or it was
recovered from the progress it pushed (`progress`), and `page_count` is the fallback from the word
count. All are read only — the measurement in particular refuses to be set by hand, because a number
nobody measured would then sit in front of every statistic reckoned in pages. See
[analytics.md](analytics.md).

A third hash, `hash_catalog`, is derived from the title rather than from the file: it is the KOReader
filename hash of the name the OPDS catalog serves the book under. It follows a rename, and like the
other two it is refused on update.

### Book statistics

`reading_book_days` is `reading_days` keyed by book as well as by day, and is read and subscribed to
the same way:

```js
const days = await pb.collection('reading_book_days').getFullList({
  filter: pb.filter('book = {:book}', { book: bookId }),
  sort: 'date',
})
```

A realtime subscription is filtered by the collection's list rule, which is per owner rather than per
book, so a client watching one book has to discard rows for the others itself.

### Devices

`devices` maps the `device_id` that appears on documents and on measured page counts to a name. Rows
are created by the server as pushes arrive, so the collection is read-mostly: `create` and `delete` are
refused, and of the fields only `name` may be changed.

```js
await pb.collection('devices').update(id, { name: 'Boox Go 7' })
```

### Collections

`book_collections` is a shelf somebody put together by hand: a name, an optional description, and a
list of books in the order they were arranged in. It is created, renamed, filled and deleted entirely
through the collection API — there is no route of its own — and the order of the `books` list is the
shelf, so whatever reads it back has to keep that order rather than sort by anything.

Books go on and come off one at a time, with PocketBase's own list modifiers, so that two open tabs
cannot overwrite each other's shelf:

```js
await pb.collection('book_collections').update(id, { 'books+': bookId })
await pb.collection('book_collections').update(id, { 'books-': bookId })
```

Rearranging is the one change that has to send the whole list, because there is no modifier for
"third, not fifth":

```js
await pb.collection('book_collections').update(id, { books: [second, first, third] })
```

Two refusals come from the server rather than from a rule, and both answer `400`:

- **`A collection can only hold books from your own library.`** — every id in `books`, after the
  modifiers have been applied, has to resolve to a book of the shelf's owner. A book somebody else
  uploaded is refused, and so is one that has been deleted meanwhile, which is the case that arrives
  on its own when a second tab is doing the deleting.
- **`A collection cannot change owner.`** — sending the record back whole, owner included and
  unchanged, is fine; changing it is not.

A duplicate name is refused by the index on `(owner, name)`, as a validation error on `name`.

## 3. The OPDS catalog, under `/opds`

The library as a catalog a reading device can browse and download from. OPDS 2.0, which is the
Readium manifest model in JSON, against the KOReader v2026.07 baseline. It is on unless `ENABLE_OPDS`
says otherwise.

The prefix is `/opds` rather than `/koreader/opds`: `/koreader` exists to isolate that reader's own
header protocol, and OPDS is a standard other readers speak.

| Method | Route | Description |
| --- | --- | --- |
| GET | `/opds` | the catalog: the shelves, the navigation feeds, and the search template |
| GET | `/opds/reading` | books started and not finished, most recently read first |
| GET | `/opds/recent` | the library, newest upload first |
| GET | `/opds/books` | the whole library, by title |
| GET | `/opds/collections` | the shelves this account put together by hand |
| GET | `/opds/authors` | every author, with a count |
| GET | `/opds/series` | every series, with a count |
| GET | `/opds/languages` | every language, with a count |
| GET | `/opds/by?facet=…&value=…` | the books under one of those entries |
| GET | `/opds/search?query=…` | books whose title or author matches |
| GET | `/opds/books/{id}/download/{name}` | the EPUB itself |
| GET | `/opds/books/{id}/cover` | the full cover image |
| GET | `/opds/books/{id}/thumbnail` | the cover at 200x300, generated on first request |

### Navigation feeds

A shelf answers with books; a navigation feed answers with the ways the library divides up, each
entry carrying its own count and pointing at `/opds/by`. Four of them ship, and the front page offers
only the ones with something in them — a facet that would open on nothing costs a page turn to find
that out:

```json
{ "metadata": { "title": "By author", "numberOfItems": 108 },
  "navigation": [
    { "rel": "subsection", "title": "Lee Child (29)",
      "href": "https://host/opds/by?facet=authors&value=Lee+Child",
      "type": "application/opds+json" } ] }
```

**Collections come first**, because they are the only division somebody decided on; the other three
are the library described back to itself. A collection is a shelf its owner built by hand, in the
order they put it in, and it is served in that order — see
[database.md](database.md) for the record behind it. Its `value` is the collection's id rather than
its name, so a bookmark survives a rename. An empty collection is left out of the feed.

**Authors are folded before they are counted.** Publisher metadata writes a name either way round and
punctuates initials as it pleases, so `Child, Lee` and `Lee Child` are one shelf, and the URL carries
a readable spelling rather than the fold key. **Languages are folded** the same way — `de`, `de-DE`
and `DE` are one entry, shown as a name — and a file that declines to name one is shelved under
"Unknown". **A series is served in reading order**, by its index and only then by title.

The value travels in the query string rather than in the path: author names carry slashes, dots,
ampersands and apostrophes, and a path segment has to survive a reader, a proxy and a router without
any of them normalising it away.

**Authentication is HTTP Basic** against `koreader_accounts` — the same credential the device syncs
with, and nothing new to create. Basic delivers the plain password, which the server hashes with MD5
before verifying against the stored bcrypt digest, so the existing verified-credential cache serves
both. A request without one is answered `401` with an `application/opds-authentication+json` body
naming the scheme, so a conformant reader can put up the right prompt instead of guessing.

Feeds are `application/opds+json`. A list of books is paginated with `?page=` (one based), and says
where it is:

```json
{
  "metadata": { "title": "All books", "numberOfItems": 10, "itemsPerPage": 50, "currentPage": 1 },
  "links": [ { "rel": "next", "href": "https://host/opds/books?page=2", "type": "application/opds+json" } ],
  "publications": [ {
    "metadata": {
      "@type": "http://schema.org/Book",
      "identifier": "urn:isbn:9783423426091",
      "title": "Zeit des Sturms",
      "author": [ { "name": "Andrzej Sapkowski" } ],
      "language": "de",
      "numberOfPages": 700,
      "description": "Finished, last opened on Boox Go 7 on 21 February 2026.\n\n700 pages, measured from your own reading.\n109,288 words.\nISBN 9783423426091"
    },
    "links": [ {
      "rel": "http://opds-spec.org/acquisition/open-access",
      "href": "https://host/opds/books/fe0p6d412uhhld6/download/Zeit%20des%20Sturms.epub",
      "type": "application/epub+zip"
    } ],
    "images": [ { "href": "https://host/opds/books/fe0p6d412uhhld6/cover" } ]
  } ]
}
```

Three things worth knowing:

**Acquisition does not go through PocketBase's file URLs.** `/api/files/...` wants a short lived token
as a query parameter, obtained from an endpoint no OPDS client has heard of. The catalog streams from
its own routes instead, behind the same Basic authentication as the feed that pointed at them.

**Every publication carries a `description`,** because KOReader greys out its "book information"
button for one that does not (`opdsbrowser.lua`: `enabled = type(item.content) == "string"`, filled
from `entry.metadata.description`). Most EPUBs carry no description of their own — none of the
reference books do — so rather than leave the button dead, the catalog writes what this server
actually knows: how far the reading has got, on which device and when, the page count and whether it
was measured or estimated, the word count, and the ISBN.

**The name in the acquisition URL is derived from the title**, not from the file as it was uploaded,
and it is sent again as the `Content-Disposition`. `hash_catalog` is the hash of that name, so a
reader holding the file under it is recognised from its very first push, with no upload and no manual
linking.

That last one comes with a caveat worth stating plainly. KOReader names a downloaded file
`Author - Title.epub` of its own accord and only asks the server what to call it when the catalog was
added with **"use server filenames"** ticked (`root_catalog_raw_names`, which switches it to a `HEAD`
and the `Content-Disposition`). So `hash_catalog` matches when that box is ticked *and* the device
identifies documents by filename. In every other case the binary hash does the work — which is the
default matching method, and which matches regardless, because the file the reader downloaded is
byte for byte the file the server holds.

## 4. The KOsync API, under `/api/kosync`

The few operations the generated collection API cannot express. All of them require an account
session (`Authorization: <token>`).

| Method | Route | Description |
| --- | --- | --- |
| POST | `/api/kosync/koreader-accounts` | Create a device credential. Body: `{"username":…,"password":…,"label":…}`. The server hashes the password with MD5 before storing it, so the browser never has to. |
| POST | `/api/kosync/koreader-accounts/{id}/password` | Replace the password of one of your credentials. Body: `{"password":…}`. |
| POST | `/api/kosync/documents/{id}/restore/{historyId}` | Put a document back into an earlier state. The state being replaced is archived first, so the restore itself can be undone. |
| POST | `/api/kosync/documents/merge` | Fold several documents into one. Body: `{"into":…,"from":[…]}`. |
| GET | `/api/kosync/achievements` | Every achievement rule with your standing in it. |
| GET | `/api/kosync/storage` | How much room your library takes, and how much it may take. |

## 5. The statistics sync target, under `/webdav`

A WebDAV collection holding one file, `statistics.sqlite3`, per account — the sync target KOReader's
statistics plugin uploads to. Authentication is HTTP Basic with a **KOReader device credential**, the
same one the sync protocol and the catalog take, so a device needs nothing new.

`PROPFIND`, `GET`, `HEAD`, `PUT`, `DELETE` and `OPTIONS` work on that one name; `MKCOL`, `MOVE`,
`COPY` and any other name are refused. An upload has to be a real SQLite database carrying KOReader's
`book` and `page_stat_data` tables, or it is not kept. See [config.md](config.md) for the reasoning
and the device setup.

Anything that belongs to somebody else answers `404`, the same as something that does not exist.

### Achievements

```json
{
  "achievements": [
    {
      "rule": "lap-warmer",
      "name": "Lap Warmer",
      "summary": "Your longest run of days without missing one.",
      "unit": "days",
      "icon": "ach-streak",
      "fur": "cream",
      "tiers": [7, 30, 100],
      "value": 12,
      "tier": 1,
      "next": 30,
      "earned": [{ "tier": 1, "value": 7, "at": "2026-08-01 10:00:00.000Z" }]
    }
  ]
}
```

The rules are served rather than duplicated in the browser. They are code — "how many nights did you
read past midnight" is a timezone conversion, not a column — and a copy of their names and thresholds
in the interface would be a second place for the two to disagree from. `icon` and `fur` name a drawing
in the web interface's sprite, so a new achievement needs no change there beyond a cat.

Unearned rules are included, with `tier: 0`. A badge nobody has yet is the one worth showing: it is
the only thing that says what there is to aim at.

There are eight of them:

| Rule | Counts | Tiers |
| --- | --- | --- |
| `first-pounce` | books read to the end, ever — the history remembers a finish a re-read has undone | 1 / 10 / 50 |
| `page-turner` | pages read, from `reading_days` and the months they were folded into | 1000 / 10000 / 100000 |
| `shelf-inspector` | books uploaded to the library | 10 / 50 / 200 |
| `night-prowler` | nights still reading after midnight, in your zone, named after the day they began | 1 / 25 / 100 |
| `lap-warmer` | the longest run of days without missing one | 7 / 30 / 100 |
| `sunbeam-sitter` | mornings reading between 05:00 and 08:00, in your zone | 1 / 25 / 100 |
| `the-long-sit` | the most pages on any single day | 100 / 250 / 500 |
| `nine-lives` | books finished and then begun again | 1 / 5 / 20 |

Subscribe to the `achievements` collection for the moment one is awarded, then read this again — the
record says which tier was earned, while the card also shows how far the next one is.

### Library storage

```json
{ "books": 42, "used": 391143424, "quota": 1073741824 }
```

Bytes, and only the EPUBs: covers are generated and not counted. `quota` is `0` when the operator has
set no limit, which is not the same as a full library and has to be told apart from one. Half of this
answer is a server setting rather than data, which is why it is an endpoint and not a sum over the
`books` collection — and it means the bar in the interface and the message refusing an upload can
never disagree about what full is.

### Merging documents

KOReader identifies a book by its contents, so the same title read from two
different copies of the file is two documents here, with the reading split
between them. Merging joins them:

```js
await pb.send(KosyncApi.mergeDocuments, {
  method: 'POST',
  body: { into: keptDocumentId, from: [otherDocumentId] },
})
```

`into` survives and keeps its own hash. It takes on the most recent position
among all of them, and a book or a title only where it had none — merging never
relabels the document the caller chose to keep.

Everything the merge replaces is written to `document_history` first. The
documents that are folded in are deleted outright, so this is the only thing
that keeps the reading they hold, and it is what makes an unwanted merge
recoverable: the state is one restore away.

**The retired hashes keep working.** Each one becomes a row in
`document_aliases` pointing at the survivor, and both `PUT` and `GET` on
`/koreader/syncs/progress` resolve through it. Without that the device that
reported a folded hash would push it again and get a fresh document back,
undoing the merge on the next sync. With it, the two devices sync with each
other. A pull is still answered with the hash it asked about, not with the one
the document is stored under.

Deleting an alias is the way back out: it is the one operation the collection
allows, and once the hash means nothing again the next push from that device
makes a document of its own.

```js
await pb.collection('document_aliases').delete(aliasId)
```
