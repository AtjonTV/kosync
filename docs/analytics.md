# Reading statistics

The dashboard shows three numbers per day: how many progress updates arrived, how much further into
their books the reader got, and roughly how long they read. This is how those are produced.

## Why they are precomputed

Legacy KOsync calculated them on every request, with a recursive query over the entire progress
history. That query gets slower every time somebody reads a page, and it ran again for every
dashboard load and every live update.

Here the numbers are computed once, in the background, and stored in the `reading_days` collection.
The web interface reads and subscribes to that collection like any other, so a recomputed day appears
in an open dashboard on its own.

## The pipeline

1. A progress push, a change in the web interface or an import writes to `documents` or
   `document_history`.
2. A hook puts `(owner, day)` into `analytics_queue`. The queue is unique on that pair, so two
   hundred pushes on one day are one item.
3. A background worker drains the queue every `ANALYTICS_WORKER_INTERVAL_SECONDS`, recomputes each
   day from the raw records and stores the result.
4. A day that turns out to have no reading at all is removed rather than stored as a row of zeroes.

If an enqueue is ever lost, a weekly job re-queues the last `ANALYTICS_RECONCILE_DAYS` days of
everybody who read recently, so a stale row heals by itself.

## Which day a reading belongs to

Every timestamp KOsync holds is UTC, and it has to be: the KOReader sync protocol carries no clock at
all. The body of a progress push is the document, the position and the device, the headers are
authentication, and there is nothing in either to say what time the device thinks it is. The
timestamp on a document is the moment the push reached the server.

A reading day, though, has to be the reader's day. So an account carries a `timezone` — an IANA name
such as `Europe/Vienna` — and every day boundary is reckoned in it. An account that has never set one
uses UTC, which is what the timestamps already are, so nothing about it is shifted.

The conversion is a half-open range of UTC instants rather than an offset applied to the stored text,
and that is not a detail. An offset would be wrong twice a year: the last Sunday in March is 23 hours
long in Vienna and the last in October is 25, and only a range says so. It also happens to be the
faster question, because `last_read_at` is indexed and a range reads the index while a substring
cannot.

**Changing the timezone recomputes everything.** Moving the boundaries makes every stored day wrong at
once, so the change queues every day the account has ever read — plus the days the old boundaries
produced, which nothing else would ever revisit. Nothing is lost, because it is all recomputed from
`documents` and `document_history`, which are untouched. But some numbers move: an evening session
that used to count as the next day moves back, which can join two streaks into one or split a day's
pages across two.

The retention cutoff is the one place this is approximate. It compares a UTC-derived date against
stored local dates for every account at once, so it can be a day out at the boundary — which for a
window measured in hundreds of days is not worth a per-account pass.

## What the numbers mean

**`update_count`** — the number of distinct moments a progress was recorded that day.

**`progress_increase`** — for each document, how much further the reader got compared to the furthest
point they had reached before that day, in percentage points, summed over all documents. Starting a
book over does not subtract from the day; it contributes zero.

**`reading_time`** — the sum of the gaps between consecutive progress updates, counting only gaps
shorter than `ANALYTICS_SESSION_GAP_SECONDS` (five minutes by default). This is an estimate, and it
is worth being explicit about why: KOReader reports positions, never durations. Reading with sync set
to every two pages produces a good estimate; reading with sync switched off produces almost none.

Unless the device has said otherwise — see **Measured days** below.

**`documents_touched`** — how many documents saw a progress update that day.

**`pages_read`** — the sum of the pages read in each book that day. Progress in a document with no
uploaded book contributes nothing: an EPUB that is not on the server has no length, and a guess would
be worse than a gap. See **Pages** below.

## Measured days

Everything above is an inference. The sync protocol carries no clock, so a day is worked out from
when pushes *arrived* and a duration from the gaps between them. Two things follow, and both are
wrong rather than imprecise:

- A device that reads offline and reconnects later lands all of that reading on the day it
  reconnected, with no reading time at all — a single push has no gap to measure.
- The days it actually read on hold no rows whatsoever.

KOReader keeps its own database of page turns, and it can sync that database to this server (see
[config.md](config.md)). What it holds is not another opinion about the same numbers: it is the only
record of when the reading happened, with a duration per page that was counted rather than deduced.

On the reference instance, over eight months:

| | inferred from pushes | measured by the device |
| --- | --- | --- |
| Reading time | 70.4 h | **130.9 h** |
| Days with reading | 91 | **107** |

So where a day has measurements, they win:

- `reading_time` becomes the sum of the durations,
- `pages_read` becomes the number of distinct pages opened,
- `documents_touched` grows to include documents that were never pushed,
- and a day with measurements is **kept even when no push ever landed on it**.

Everything the pushes alone can say — `update_count`, `progress_increase` — is still theirs, because
the statistics database has nothing to say about progress through a document.

The measurements are stored as events, in `page_reads`, and aggregated at recomputation time in the
account's own timezone. That is what lets a zone change move them the same way it moves everything
else, and it is why an import queues the days it touched rather than writing totals directly.

One consequence worth stating: on a measured day, `pages_read` is the day's own count and is no longer
the sum of its book rows. Reading in a file that was never uploaded has pages the device counted and
no book to attribute them to, so the day is larger than its books. The alternative would be to throw
those pages away for tidiness.

## Per book

The same measures are also computed per book, into `reading_book_days`, keyed by `(owner, date,
book)`. That is what the book page in the web interface is made of: how long a book took, which days
went into it, how many pages each of those days was.

They are computed independently of the day rows rather than by grouping them, and the two do not
agree — deliberately:

- **Reading time does not add up.** A day's reading time is the sum of the gaps between consecutive
  pushes. A gap that spans a switch from one book to another falls inside neither book's window, so
  the book rows sum to *less* than the day. On the reference data the difference is around two
  minutes on a day with three books. The day total is the authoritative one; the residual is
  switching, and forcing the books to add up to it would mean inventing an owner for it.
- **Pages do add up.** Every page is read in exactly one book, so the day's `pages_read` is the sum
  of its book rows and nothing falls between them.

Only documents matched to an uploaded book appear here at all. Measured reading is folded into these
rows the same way it is into the day: the reading time and pages of a book the device measured replace
the inferred ones, and a book read entirely offline gets a row it would otherwise never have had.

## Pages

An EPUB is reflowable: it has no pages, and KOReader's own count changes with the font and the
screen. KOsync does not ask for one. It takes the device's, in one of two ways, and records in
`measured_source` which of them a number came from.

**The count the device states.** A synced statistics database carries `total_pages` on every page
turn: the count the book stood at when that page was read. KOsync takes the pagination most of a
document's forty most recent turns were recorded in. Forty because a device's count moves with the
font, so only the recent end describes the book as it is now — and because three stray turns from
reopening a finished book must not outvote a week of reading it. This is a fact, not an inference, and
it has no upper limit. It is stored without a device: which reader wrote the file is not in the file.

**The count recovered from the pushes.** KOReader syncs every N pages, so the progress values a device
pushes move in near-exact multiples of one page. Recovering that unit gives the page count directly —
`1 / page_fraction`. On the reference books this reproduced the counts the device reports, 700 and
563, exactly. It is taken per file and per device, from the recent end of the series first so changing
the font is followed rather than ignored, and the series with the most pushes behind it wins. This one
is stored with the `device_id` it came from — see the `devices` collection in
[database.md](database.md) for how that becomes a name worth reading.

Where both are available the stated count wins and the estimator leaves the book alone, because the
estimate is a reconstruction of exactly the number the other one states outright. The estimate is also
the one that can fail, and when it cannot be had it says so instead of guessing:

- **It needs pushes.** A book read before the server existed has nothing to measure.
- **It stops at roughly 1600 pages.** Progress is reported to four decimals, and a page in a very long
  omnibus is narrower than that grid. Three of the five reference books fall outside for one of these
  two reasons.

A book with neither falls back to `page_count`, which is the word count divided by
`BOOKS_WORDS_PER_PAGE` (155 by default). That is a real fallback and not a second opinion: on the
reference books the density ranged from 154 to 207 words per page **on the same device**, so the
fallback can be a third out. The web interface labels which of the three a number came from for
exactly that reason.

One consequence worth stating: a book uploaded after its reading was imported has no stated count
until the next statistics sync, because the counts are read out of the file as it arrives. The next
sync a device makes fills it in.

## Retention

Daily rows are kept for `ANALYTICS_RETENTION_DAYS` (90 by default). A day that ages out is either
folded into its month in `reading_months` and then deleted (`ANALYTICS_RETENTION_MODE=aggregate`, the
default) or simply deleted (`=delete`).

Folding happens exactly once per daily row, because the row is deleted in the same pass, so running
the job twice cannot double count.

The per-book rows follow the mode rather than the window: `delete` removes them with everything else,
`aggregate` keeps them. They are not the per-day detail retention exists to bound — they are the
record of how long a book took, and a monthly total cannot hold that. There is one row per book per
day, which for a reader is the same order of magnitude as the day rows themselves.
