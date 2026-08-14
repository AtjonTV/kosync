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

A pull answers with the same fields plus `timestamp`, in **Unix seconds**. KOsync 1 returned its
internal 1/10000 second unit here, which no KOReader build expects; a client written against that
quirk needs adjusting.

A percentage outside 0..1 is clamped rather than refused, so a rounding artefact on the device does
not cost the reader their push.

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
the server reads the EPUB as it arrives and fills in the title, authors, language, identifiers, cover,
word count and both KOReader document hashes. Sending any of those is refused on update, and ignored
in favour of the file on create.

```js
const form = new FormData()
form.append('owner', pb.authStore.record.id)
form.append('file', epubFile)

await pb.collection('books').create(form)
```

Covers are served as PocketBase files, with thumbnails generated on request:

```
/api/files/books/{id}/{cover}?thumb=200x300
```

The page count is derived too, and in two ways: `measured_pages` is what the device's own progress
reports imply, and `page_count` is the fallback from the word count. Both are read only — the
measurement in particular refuses to be set by hand, because a number nobody measured would then sit
in front of every statistic reckoned in pages. See [analytics.md](analytics.md).

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

## 3. The OPDS catalog, under `/opds`

The library as a catalog a reading device can browse and download from. OPDS 2.0, which is the
Readium manifest model in JSON, against the KOReader v2026.07 baseline. It is on unless `ENABLE_OPDS`
says otherwise.

The prefix is `/opds` rather than `/koreader/opds`: `/koreader` exists to isolate that reader's own
header protocol, and OPDS is a standard other readers speak.

| Method | Route | Description |
| --- | --- | --- |
| GET | `/opds` | the catalog: the three shelves, and the search template |
| GET | `/opds/reading` | books started and not finished, most recently read first |
| GET | `/opds/recent` | the library, newest upload first |
| GET | `/opds/books` | the whole library, by title |
| GET | `/opds/search?query=…` | books whose title or author matches |
| GET | `/opds/books/{id}/download/{name}` | the EPUB itself |
| GET | `/opds/books/{id}/cover` | the full cover image |
| GET | `/opds/books/{id}/thumbnail` | the cover at 200x300, generated on first request |

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

Anything that belongs to somebody else answers `404`, the same as something that does not exist.
