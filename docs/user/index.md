# KOsync for readers

KOsync remembers where you are in a book, on every device you read on. Close a book on the phone
on the train, open it on the e-reader in the evening, and it opens on the page you stopped at.

That is the whole idea. Everything else on this site exists because the server has to hold your
books and your reading anyway, and once it holds them it can just as well hand them back to you.

## What you get

Your reader tells the server where you got to every couple of pages, and another device asks the
server before it opens the book. Neither of them has to be online at the same time as the other,
and neither has to be the one you read on last.

Then there is the library. Upload your EPUBs here and they are yours to pull down onto any device,
over the same connection that syncs your reading. The server reads the title, the authors, the
series, the cover and the publisher's blurb out of each file, so the shelf looks like a shelf
rather than a list of file names. See [library.md](library.md).

And there is the record of it all: pages, hours, the days you read on, how far you got in each
book, and eight achievements that are cats. None of it is guessed at — it is what your reader
actually reported. See [statistics.md](statistics.md).

## What it is not

KOsync is not a reading app. [KOReader](https://koreader.rocks) is the reader — on a Kobo, a
Kindle, a Boox, an Android phone, or a desktop — and KOsync is what sits behind it. You do your
reading in KOReader and come here to see what came of it, to add books, or to change a setting.

The one exception is the **preview**, a book shown in the browser so you can answer "what is this
one about" without opening the file on a reader. It is deliberately not a second reader: it keeps
no position and it counts as nothing.

## The three addresses

Your reader is pointed at this server three times, for three different things. All three take the
same credential, and only the first is required.

| What | Address | For |
| --- | --- | --- |
| Sync server | `https://your-host/koreader` | where you are in each book |
| Catalog | `https://your-host/opds` | downloading books onto the device |
| Statistics | `https://your-host/webdav/` | KOReader's own page-turn record |

The exact host is whatever you type into the browser to get here. The dashboard shows all three
filled in, so you do not have to assemble them by hand.

## Where to start

[getting-started.md](getting-started.md) takes you from nothing to a device that syncs, in nine
steps. The rest of these pages describe one part each:

- [library.md](library.md) — uploading books, finding them again, collections, the preview
- [reading.md](reading.md) — how your progress arrives, and what to do when it arrives twice
- [statistics.md](statistics.md) — the numbers, the chart, the achievements, the summary mail
- [account.md](account.md) — your sign-in, your devices, and the credentials they use
