<!--
File:        docs/plans/epub-preview-reader.md
Project:     https://git.obth.eu/atjontv/kosync
Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->

# EPUB preview reader — implementation plan

Status: proposal, no code written yet.
Target: a read-only look inside a book from the web interface, so "what is this one about?" is
answered without opening it on a device, and without that answer becoming reading history.

---

## 1. The problem this solves

A book on the shelf shows its title, its authors, its series and its cover. It does not show what is
inside it. `dc:description` is absent from most of the reference library, so the only way to find
out is to open the book — and every way of doing that today is wrong for the question:

| Way in | What it costs |
| --- | --- |
| Open it on the reader | KOReader syncs progress, so a two-minute skim becomes reading history |
| Download and open in a desktop reader | a download, an application launch, and a file to clean up |
| Nothing else exists | — |

The web interface already holds the file and already parses EPUBs on the server. Showing the text is
a smaller step than any of the above.

## 2. Scope

**In scope**

- Reading the text of a book that is already in the library, chapter by chapter, in the browser.
- Moving between chapters, and a list of them to jump around with.
- Working on an e-ink tablet browser (the operator reads on a Boox), which means large tap targets,
  no animation, no layout that depends on a hover state, and as little script per page turn as the
  design allows.

**Explicitly out of scope**

- Reading statistics of any kind. The preview touches no `documents`, no `reading_days`, no
  achievements. This falls out of the design rather than needing a guard: the endpoints read a book
  and write nothing.
- A remembered position. Closing the preview forgets where it was, by design. Nothing is written to
  the database and nothing to `localStorage`.
- Font, margin and theme controls, bookmarks, annotations, search inside the book, text selection
  features. A preview is not a reading application.
- Formats other than EPUB. The library only holds EPUB.

**Adjacent, and deliberately not part of this plan:** `dc:description` is parsed by no code today,
though `internal/epub` sees it. Storing it on the book and showing it on the book page would answer
the same question for the books that carry one, and is a much smaller change. It is worth doing on
its own; it does not replace this, because most books have no description.

## 3. The approach, and the one not taken

### 3.1 Chosen: the server extracts, the browser displays

The server opens the stored EPUB, pulls out one spine document, strips it down to an allow-list of
markup, and returns that as JSON. The browser drops the result into a sandboxed `<iframe>`.

Why this way:

- `internal/epub` already opens the archive, resolves the package document and walks the spine. The
  reader needs one more method, not a new parser.
- Nothing new arrives in the browser bundle. The web interface has eleven runtime dependencies and
  a licence gate on both sides of the build; keeping it at eleven is worth some server-side work.
- A page turn is one small JSON request. The alternative downloads the whole book — 128 MB is the
  upload limit — into a tablet browser before showing the first word.
- The server decides what markup is allowed to exist, in one place, with tests.

### 3.2 Not chosen: a JavaScript EPUB reader in the browser

`epub.js` or `foliate-js` would give real pagination, CSS fidelity and a page-turn animation.
Rejected because:

- It moves the whole file to the client, which is exactly the slowness the operator is trying to
  avoid, and does it on the least capable device in the house.
- It renders the book's own CSS and, in the general case, its own scripts. Getting that safe means
  the same sandboxing work as below, plus a dependency whose licence has to clear `bun audit` and
  the project's allow-list.
- Fidelity is not the goal. The goal is a paragraph or two of prose.

Worth revisiting only if the preview turns out to be used as a reader, which the operator has said
it is not for.

## 4. Server

### 4.1 Getting at the archive without loading it

`internal/books/covers.go` reads a stored book fully into memory (`openStoredBook`, capped at
`maxCoverBytes`). That is right for a nightly pass over books that mostly have nothing to find. It
is wrong for an interactive endpoint, where it would mean up to 128 MB per page turn.

`filesystem.System.GetReader` returns a `*blob.Reader`, which is an `io.ReadSeekCloser` that knows
its own `Size()`. `zip.NewReader` wants an `io.ReaderAt`. The gap is a small adapter — seek, then
read, under a mutex — after which the zip central directory and one entry are all that is read.

- New: `internal/books/storage.go` (or a well-named file in the same package) holding the adapter
  and an `openBook(app, record) (*epub.Reader, func() error, error)` helper that returns the reader
  and its closer.
- A seek on `blob.Reader` re-creates the underlying driver reader on the next read. On the local
  filesystem that is a re-open; on S3 it is a range request. A chapter costs a handful of these,
  which is acceptable and still far cheaper than reading the whole file.
- `covers.go` can move onto the same helper afterwards. Not required, and not in this plan's first
  phase.

### 4.2 What `internal/epub` gains

Three additions, each with the package's existing caps applied:

```go
// Spine lists the documents the book is read in, in order.
func (r *Reader) Spine() []Document

// Document is one entry of the spine.
type Document struct {
    Index int    // position in the spine
    Path  string // archive path, for resolving what the document references
    Title string // <title>, or the first heading, or ""
}

// Read returns one spine document as it is stored, capped at maxDocumentBytes.
func (r *Reader) Read(index int) ([]byte, error)

// Resource returns one file the archive holds, for the images a document draws.
func (r *Reader) Resource(name string) ([]byte, string, error)
```

`maxSpineItems` (10000) and `maxDocumentBytes` (32 MiB) already exist and apply unchanged.
`maxSpineBytes` does not: nothing here reads the whole spine at once.

Chapter titles in the first phase come from the document itself — its `<title>`, falling back to the
first `<h1>`–`<h3>`, falling back to "Chapter *n*". This is honest and needs no new parsing.

**Phase two, optional:** the real table of contents, from the EPUB 3 navigation document
(`properties="nav"`) or the EPUB 2 NCX (`<spine toc="…">`, media type `application/x-dtbncx+xml`).
It gives better titles and a proper hierarchy. It is a genuine chunk of parsing and is not needed to
answer "what is this about", so it is separated out and can be skipped.

### 4.3 Reducing a chapter to something safe to show

New package: `internal/preview`. One exported function, everything else internal to it:

```go
type Resolver func(href string) (data string, ok bool)

// Clean turns one spine document into the markup the web interface may render.
func Clean(document []byte, resolve Resolver) (html string, truncated bool)
```

It parses with `golang.org/x/net/html` — already a direct dependency, used by `internal/webdav` —
walks the tree, and **re-serialises only what is on the allow-list**. Anything not named is dropped;
this is a rebuild, not a scrub, so unknown constructs fail closed.

| Kept | Dropped |
| --- | --- |
| `p h1 h2 h3 h4 h5 h6 em strong i b u s small sub sup br hr blockquote q cite` | `script style link meta iframe object embed form input audio video canvas` |
| `ul ol li dl dt dd div span section article header footer figure figcaption` | every `on*` attribute |
| `table thead tbody tfoot tr td th` (`colspan`, `rowspan`) | `style`, `class`, `id` — the book's own CSS is not loaded, so they say nothing |
| `img` with a resolved `src`, plus `alt`, `width`, `height` | `img` with an unresolvable or remote `src` |
| `a` with `href`, rewritten (§4.4) | `a` with a remote or unresolvable `href` — kept as plain text |

Two rules matter more than the table:

1. **No URL leaves the archive.** Every `src` and `href` is resolved against the archive; anything
   with a scheme, a host, or a `//` prefix is dropped. A tracking pixel in a book must not be able
   to tell a publisher that this library holds this file and read it on this evening.
2. **Output is capped.** A chapter is truncated at 512 KiB of cleaned markup, at an element
   boundary, and the endpoint says `"truncated": true` so the interface can show a line about it.
   Some books put themselves in one enormous XHTML file.

### 4.4 Images and internal links

**Images** are inlined as `data:` URIs by the `resolve` callback, from the archive, with three caps:
per image 2 MiB, per chapter 8 MiB, and only `image/jpeg`, `image/png`, `image/gif`, `image/webp`
and `image/svg+xml` — the last of these re-cleaned through the same allow-list, because SVG is
markup and can carry script. An image over the cap is dropped and its `alt` text kept.

Inlining rather than a second endpoint, because the preview endpoint is authenticated and an
`<img>` inside a sandboxed frame sends no `Authorization` header — the same problem the library's
covers solve with a file token in the address. A resource endpoint would need its own version of
that, which is more surface than a preview needs. If real books turn out to be too image-heavy for
inlining, a token-carrying resource endpoint is the escape hatch, and only §4.3's `resolve` callback
changes.

**Internal links** — a footnote, a chapter cross-reference — are rewritten to
`#kosync-preview:<spineIndex>` and handled by the interface, so following one moves the preview
rather than navigating the application. A link the archive cannot resolve becomes plain text.

### 4.5 The endpoints

Under `/api/kosync`, which already binds `apis.RequireAuth(schema.CollectionUsers)`. Ownership is
checked per request against `schema.FieldOwner`, the way the rest of the package does it — the
collection rule does not apply to a custom route.

```
GET /api/kosync/books/{id}/preview
GET /api/kosync/books/{id}/preview/{index}
```

The first returns the shape of the book:

```json
{
  "title": "Zeit des Sturms",
  "chapters": [{ "index": 0, "title": "Cover" }, { "index": 1, "title": "Kapitel 1" }]
}
```

The second returns one chapter:

```json
{ "index": 1, "title": "Kapitel 1", "html": "<p>…</p>", "truncated": false }
```

Both answer `404` for a book that is not the caller's, exactly as for one that does not exist, so
the endpoint does not confirm that an id belongs to somebody. Both send `Cache-Control: private,
max-age=…` and an `ETag` from the book id and the chapter index, so a page turn back is free.

Where the code goes:

| File | Holds |
| --- | --- |
| `internal/preview/preview.go` + `_test.go` | `Clean`, the allow-list, the caps |
| `internal/epub/spine.go` + `_test.go` | `Spine`, `Read`, `Resource` (or added to `epub.go`) |
| `internal/books/storage.go` + `_test.go` | the `ReaderAt` adapter and `openBook` |
| `internal/kosyncapi/preview.go` + `_test.go` | the two routes, ownership, caching |

### 4.6 What this costs the server

Per page turn: one storage open, a zip central directory read, one entry decompressed (≤ 32 MiB),
one HTML parse and one re-serialisation of at most 512 KiB. For an ordinary chapter this is single
-digit milliseconds and a few hundred kilobytes of allocation. No caching is proposed for the first
version; if it is ever wanted, an LRU of open books keyed by record id is the obvious shape, and the
`ETag` already spares the repeat requests that matter.

## 5. Web interface

### 5.1 Where it lives

A route rather than a dialog: `/library/:id/preview`, name `preview`, `requiresAuth`. The reader
wants the whole window, the Boox needs a hardware back button that works, and a dialog gives
neither.

- `webui/src/views/BookPreviewView.vue` — the page.
- A "Preview" button on `BookView.vue`, next to Download, icon `pi pi-eye`.
- Optionally the same on the library card. Worth doing; not required for the feature to be useful.

### 5.2 The page

```
┌──────────────────────────────────────────────┐
│ ‹ Back    Zeit des Sturms · Kapitel 1    ☰   │   fixed header
├──────────────────────────────────────────────┤
│                                              │
│   the chapter, scrolling inside this pane    │   sandboxed iframe
│                                              │
├──────────────────────────────────────────────┤
│   ‹ Previous            Next ›               │   fixed footer
└──────────────────────────────────────────────┘
```

- `☰` opens a PrimeVue `Drawer` with the chapter list; picking one loads it.
- Previous/Next move by one spine entry and scroll the pane back to the top.
- Chapters already fetched are kept in a `Map` in the component for the life of the page, so paging
  back is instant. Nothing is written anywhere; leaving the route drops it.
- No store. This is one page's state and no other page wants it — unlike `books` or `bookStats`,
  which are shared. `KosyncApi` in `src/pb.ts` gains the two endpoint builders.
- Keyboard: left/right arrows page. Cheap, and it makes the desktop case pleasant.

### 5.3 Rendering the chapter

The cleaned markup goes into an `<iframe>` via `srcdoc`, with

```html
<iframe sandbox referrerpolicy="no-referrer" …>
```

`sandbox` with no tokens at all: no scripts, no same-origin, no forms, no top-level navigation. This
is the actual security boundary (§6), and it costs one thing — with no scripts, the frame cannot
report its own height, so the pane is a fixed-height scrolling region rather than growing to fit.
For a full-window reader that is what is wanted anyway.

The `srcdoc` carries a small stylesheet the interface writes itself: readable measure, generous line
height, `img { max-width: 100% }`, and the two colour schemes wired to `prefers-color-scheme` so
dark mode follows the rest of the interface. The book's own CSS is never loaded.

## 6. Security

Untrusted markup from a file an operator uploaded is being rendered inside an authenticated
application. Two independent defences, in this order:

1. **The sandboxed frame.** No `allow-scripts` and no `allow-same-origin` means script cannot run
   and, if it somehow did, it would run in an opaque origin with no access to the session token, the
   API, or the parent document. This holds even if the sanitiser has a bug.
2. **The allow-list rebuild** (§4.3). Defence in depth, and the thing that keeps the page tidy and
   quiet: no external requests, no book CSS fighting the interface.

Also considered and decided:

- **`v-html` into the page instead of a frame** — rejected. It makes the sanitiser the only thing
  between a book and the session, and hand-written sanitisers are a bad bet.
- **A vetted sanitiser (`bluemonday`, BSD-3-Clause, allow-listed)** — a reasonable substitute for
  §4.3 if the maintainer would rather not own that code. It is only a sensible trade because the
  frame, not the sanitiser, is the boundary; if the frame were dropped, this becomes mandatory.
- **A `Content-Security-Policy`** — the server sets none today, for the interface or anything else.
  Adding one is a good idea and a separate change; this feature does not depend on it.
- **Zip bombs** — bounded by `maxDocumentBytes` per entry and the per-chapter output cap. Nothing
  reads the whole spine.
- **Access control** is the same story the stored files now tell: `file` and `cover` are protected
  fields, so `/api/files/…` checks the collection's view rule against a file token. The preview
  endpoint reaches the same conclusion by a shorter route — a session, and an ownership check in
  the handler.

## 7. Tests

Following CONTRIBUTING §2: a `_test.go` beside every new source file, standard library only, real
migrated `testutil.NewApp(t)`, no mocked database.

**Go**

- `internal/preview`: script tags, `on*` attributes, `javascript:` and remote `src`/`href`,
  `<style>` and `<link>`, an SVG carrying script, unbalanced markup, an entity-encoded payload,
  the truncation cap, and that ordinary prose survives unchanged.
- `internal/epub`: spine order, a spine entry pointing at a missing file, title extraction and both
  fallbacks, `Resource` for an image and for something outside the archive.
- `internal/books`: the `ReaderAt` adapter against a stored file, including a read that crosses a
  seek.
- `internal/kosyncapi`: both routes for a book that is the caller's, another user's book (404), an
  unknown id (404), a chapter index out of range, no session (401), and a stored file that is not a
  readable EPUB (a clean error, not a panic).

**Vitest** (`webui/src/tests/components/BookPreviewView.test.ts`)

- Loads the chapter list, renders the first chapter, moves with Next and Previous, jumps from the
  drawer, keeps a fetched chapter instead of refetching it, and shows an error state when the
  endpoint fails. `pb` mocked as everywhere else.

## 8. Documentation and changelog

- `docs/api.md` §4 gains both endpoints, with the request and response shapes and the note that they
  neither write nor count as reading.
- `CHANGELOG.md`, Unreleased → Added: one entry written for the operator — what it is for, that it
  records nothing, and that it forgets where you were.
- No new environment variable, so nothing for `kosync.env.example` or `docs/config.md`. If the caps
  in §4.3 and §4.4 should be operator-settable, that changes and both files gain entries; the
  recommendation is that they should not be.
- No schema change, so nothing for `docs/database.md`.

## 9. Order of work

Each step builds and tests on its own.

| # | Step | Touches |
| --- | --- | --- |
| 1 | `ReaderAt` adapter and `openBook` | `internal/books` |
| 2 | `Spine`, `Read`, `Resource` | `internal/epub` |
| 3 | `internal/preview`: allow-list, caps, image inlining, link rewriting | new package |
| 4 | The two endpoints, ownership, caching | `internal/kosyncapi`, `docs/api.md` |
| 5 | `BookPreviewView.vue`, route, `KosyncApi` entries, the button on `BookView` | `webui` |
| 6 | Changelog | `CHANGELOG.md` |
| 7 | *Optional:* nav/NCX table of contents | `internal/epub` |
| 8 | *Optional:* the Preview button on the library card | `webui` |

Steps 1–6 are the feature. 7 and 8 are improvements to it.

## 10. Decisions the operator should make before step 1

1. **Own sanitiser or `bluemonday`?** The plan assumes the former, on the grounds that the frame is
   the boundary and `golang.org/x/net/html` is already a dependency.
2. **Images inlined, or a signed resource endpoint?** The plan assumes inlined with caps, which is
   simpler and enough for prose. Heavily illustrated books would argue the other way.
3. **Table of contents in the first version, or spine order with derived titles?** The plan assumes
   the latter, and keeps the real one as step 7.
4. **Preview from the library card as well as the book page?** The plan assumes book page first.
