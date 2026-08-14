# API

KOsync has three groups of endpoints.

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

## 3. The KOsync API, under `/api/kosync`

The few operations the generated collection API cannot express. All of them require an account
session (`Authorization: <token>`).

| Method | Route | Description |
| --- | --- | --- |
| POST | `/api/kosync/koreader-accounts` | Create a device credential. Body: `{"username":…,"password":…,"label":…}`. The server hashes the password with MD5 before storing it, so the browser never has to. |
| POST | `/api/kosync/koreader-accounts/{id}/password` | Replace the password of one of your credentials. Body: `{"password":…}`. |
| POST | `/api/kosync/documents/{id}/restore/{historyId}` | Put a document back into an earlier state. The state being replaced is archived first, so the restore itself can be undone. |

Anything that belongs to somebody else answers `404`, the same as something that does not exist.
