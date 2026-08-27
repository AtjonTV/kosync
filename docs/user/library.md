# Your library

The library is the copy of your books that lives on the server. It is a backup, it is where your
devices download from, and it is how KOsync knows that the file two readers are reporting progress
for is one book.

None of it is required — a device syncs its position whether or not the book is here — but almost
everything else on the site is built out of it. A book that is here has a cover, a blurb, a page
count and a preview; a book that is not is a hash and a title.

## Adding books

**Add EPUB**, top right of the library. Pick one file or a dozen; they are uploaded one at a time,
and if one fails the others still go up and the failure says which file it was.

Upload the very file you read on the device. The match is made on the file's contents, so another
copy of the same title — a different conversion, a different shop, the same book re-downloaded —
is a different book as far as the server is concerned. See [reading.md](reading.md) for what to do
when that has already happened.

Only EPUBs. KOsync reads inside the file, and that is a format it can read.

### What is read out of the file

On upload the server opens the archive and takes the title, the authors, the series and its
number, the language and the publisher's blurb out of the book's own metadata; the cover from
wherever the file says its cover is; and a word count from the text itself.

Nothing is fetched from the internet, and nothing is guessed. A book whose metadata is thin looks
thin here, which is usually a sign the file itself is thin.

A book that arrived without a cover is not a lost cause: the server retries the extraction nightly
for books that have none, so a file that needed a slower path gets its cover in the small hours
rather than never.

**Pages** is a number with two sources. Until a device has read the book, it is what the word count
implies; once a device has reported its own page count, that measured number takes over. This is
why a page total can change after you start reading, and why it can differ from what somebody
else's reader says: pages are a property of the device's font size, not of the book.

The title is the one thing you can change by hand — the trash-can's neighbour on each cover, or
**Change the title** — for the files that come out of a shop named `book_final_2 (1)`.

## Finding a book again

Above the shelf there are three controls, and the library remembers where you left them.

**Search** matches title, author and series as you type, and while it is on, the count beside it
says how much of the library is showing. **Sort by** offers Title, Recently added, Recently read
and Progress. **Group by** offers Nothing, Author, Series and Language — grouped by series, a
shelf reads like a shelf, with the trilogy standing together in its own order rather than
scattered alphabetically.

## The book page

Click a cover. The page has the metadata on the left (Pages, Words, Language), the blurb under
**About this book** where the file had one, and, once you have read some of it, what the reading
came to: **Time spent**, **Pages read**, **Days read**, **Best day**, and a chart of the days you
read it on.

Three buttons under the cover. **Download** hands you the EPUB back, exactly the bytes you
uploaded. **Preview** opens the book in the browser, which is the section below. **Add to
collection** files it on a shelf of your own; the collections it is already on are shown as chips,
each with a **Take off** button.

## The preview

The preview shows the book itself in the browser: a chapter list down the side, **Previous** and
**Next** through the spine, and the text in between.

It is deliberately not a second reader. It keeps no position, it counts as nothing, and it leaves
no trace that you looked — no progress, no statistics, no "last read". It answers "what is this one
about", which is a question asked once.

Two things about it will look like faults and are not. The first is that links do nothing: a
footnote marker or a cross reference comes back as its own words rather than as something to
click, because a link inside a book points at another file in the archive, and a browser showing
one chapter does not have that.

The second is the chapter names, which are the book's own and not ours. They come out of its table
of contents, so they are whatever the publisher wrote, including the ones that are just `1`. Where
a chapter sits inside a part, the part's name is shown with it, which is what makes a trilogy in
one file readable — it numbers its chapters from one three times over.

A very long chapter is cut off at the point where rendering it stops being reasonable, and says so.

## Collections

A collection is a shelf you arrange by hand. **Collections** in the menu, then **New collection**,
with a name and a description.

Add books to it from the collection's own page or from a book's **Add to collection**, then move
them up and down with the arrows. That order is not decoration: a collection is served over OPDS in
exactly the order you put it in, so "the series, in order" or "next up" is a shelf your reader can
browse. See the OPDS section of [getting-started.md](getting-started.md).

Renaming or deleting a collection touches nothing but the collection. The books stay in the
library.

## Storage

Where the server sets a quota, the library page has a **Storage** bar above the shelf —
`1.2 GB of 5.0 GB`. It turns amber at 80% and red at 95%, so the first you hear of a full library
is not an upload being refused.

Whoever runs the server decides the quota, and may set none at all. Then there is no bar, because
there is nothing to be near the end of.

## Deleting a book

The trash can on the cover, or on the book's page. It asks first, and what it asks is the whole
story:

> Delete "…"? The file is removed from the server. Reading progress pushed by your devices is kept.

That is the part worth understanding. The library and the reading record are two different things
(see [reading.md](reading.md)): deleting a book takes away the file, the cover and the blurb, and
leaves the document your devices have been reporting progress for exactly where it was. Your
statistics do not change, your streak does not break, and your reader carries on syncing that book.

Upload the same file again and the two find each other again.
