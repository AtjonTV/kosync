# Reading statistics

The dashboard shows three numbers per day: how many progress updates arrived, how much further into
their books the reader got, and roughly how long they read. This is how those are produced.

## Why they are precomputed

KOsync 1 calculated them on every request, with a recursive query over the entire progress history.
That query gets slower every time somebody reads a page, and it ran again for every dashboard load
and every live update.

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

## What the numbers mean

Everything is computed in **UTC**.

**`update_count`** — the number of distinct moments a progress was recorded that day.

**`progress_increase`** — for each document, how much further the reader got compared to the furthest
point they had reached before that day, in percentage points, summed over all documents. Starting a
book over does not subtract from the day; it contributes zero.

**`reading_time`** — the sum of the gaps between consecutive progress updates, counting only gaps
shorter than `ANALYTICS_SESSION_GAP_SECONDS` (five minutes by default). This is an estimate, and it
is worth being explicit about why: KOReader reports positions, never durations. Reading with sync set
to every two pages produces a good estimate; reading with sync switched off produces almost none.

**`documents_touched`** — how many documents saw a progress update that day.

**`pages_read`** — always zero for now. It becomes meaningful once a document can be linked to an
uploaded book with a known page count.

## Retention

Daily rows are kept for `ANALYTICS_RETENTION_DAYS` (90 by default). A day that ages out is either
folded into its month in `reading_months` and then deleted (`ANALYTICS_RETENTION_MODE=aggregate`, the
default) or simply deleted (`=delete`).

Folding happens exactly once per daily row, because the row is deleted in the same pass, so running
the job twice cannot double count.
