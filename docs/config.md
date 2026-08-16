# Configuration

KOsync is configured in two places, and it is worth knowing which is which.

**PocketBase** owns the listen address, the data directory, the mail server, the backups, the rate
limits and the token lifetimes. Those are command line flags and settings in the superuser interface
at `/_/`:

```bash
kosync serve --http=0.0.0.0:8080 --dir=/pb_data
```

**KOsync** owns everything below. These are environment variables, optionally read from a
`kosync.env` file next to the executable. A value already present in the environment wins over the
file. See [`server/kosync.env.example`](../server/kosync.env.example) for a copy with comments.

| Variable | Default | Meaning |
| --- | --- | --- |
| `ENABLE_WEBUI` | `false` | serve the embedded web interface at `/` |
| `DISABLE_REGISTRATION` | `false` | refuse new accounts in the web interface |
| `ANALYTICS_RETENTION_DAYS` | `90` | how long a day of statistics is kept in detail |
| `ANALYTICS_RETENTION_MODE` | `aggregate` | `aggregate` folds an aged out day into its month, `delete` drops it |
| `ANALYTICS_WORKER_INTERVAL_SECONDS` | `5` | how often queued statistics are recomputed |
| `ANALYTICS_WORKER_BATCH_SIZE` | `50` | how many days one pass recomputes |
| `ANALYTICS_SESSION_GAP_SECONDS` | `300` | the longest pause that still counts as reading |
| `ANALYTICS_RECONCILE_DAYS` | `7` | how many recent days the weekly reconciliation recomputes |
| `KOREADER_AUTH_CACHE_TTL_SECONDS` | `300` | lifetime of a verified device credential in memory, `0` disables |
| `KOREADER_AUTH_CACHE_ENTRIES` | `1024` | how many credentials are cached at most |
| `BOOKS_WORDS_PER_PAGE` | `155` | fallback reading density for books whose page count cannot be measured |
| `BOOKS_QUOTA_MEGABYTES` | `0` | how much room one account's books may take together, `0` means no limit |
| `ENABLE_OPDS` | `true` | serve the library as an OPDS catalog at `/opds` |
| `OPDS_PAGE_SIZE` | `50` | how many books one page of a catalog feed holds |
| `ENABLE_WEBDAV` | `true` | serve a WebDAV target at `/webdav` for KOReader's statistics database |
| `WEBDAV_MAX_MEGABYTES` | `64` | the largest statistics database that will be accepted |
| `ENABLE_ACHIEVEMENT_MAIL` | `true` | let the server tell an account what it has earned |
| `ENABLE_SUMMARY_MAIL` | `true` | let the server send the weekly and monthly reading summaries |

An invalid value falls back to its default instead of stopping the server.

## About the credential cache

Device credentials are stored bcrypt hashed, and KOReader is usually set to push every two pages.
Verifying bcrypt on every push is expensive, so a successful verification is remembered for
`KOREADER_AUTH_CACHE_TTL_SECONDS`.

Two things this does not do: it never caches a failed attempt, so guessing a password still costs the
full verification every time; and it never keeps a rotated, disabled or deleted credential alive,
because those changes drop the cached entry immediately.

## About the catalog

`ENABLE_OPDS` is on by default. The catalog shows one account its own books and asks for the same
device credential that account already syncs with, so it exposes nothing the sync API does not; turn
it off if you would rather not have the endpoint there at all.

The links inside a feed are absolute, and they are built from the address the request arrived on so
that a reader gets back the name it reached the server by. Behind a reverse proxy that means
`X-Forwarded-Proto` and `X-Forwarded-Host` have to be set, or the acquisition links will point at
whatever the proxy dialled — usually an address the device cannot reach.

## About the library quota

`BOOKS_QUOTA_MEGABYTES` is `0` by default, which means an account may upload as much as the disk
holds. That is the right default for the instance this was written for — one reader, their own
machine — and a limit is a decision about somebody else's library that only you can make.

With a limit set, an upload that would not fit is refused with a message saying what it needed and
what was free, and the library page shows a bar. Only the EPUB counts; the extracted cover is
generated and small. Lowering the limit below what an account already holds takes nothing away: the
books stay, no more can be added.

Superusers are not counted against it, so an operator can always put a file into an instance they
administer.

## About the statistics sync

KOReader keeps its own database of every page turn — which book, which page, when, and for how long —
and can sync it to a cloud target. The targets it offers are Dropbox, FTP and WebDAV, and of those
only WebDAV is something this server can be, so `/webdav` is one.

The point is that nothing has to be carried by hand. A device that already pushes progress has a
credential, and that same credential is what the WebDAV target takes; there is no browser to log into
on an e-ink screen and no file to move between machines.

Setting it up on the device, roughly: add a WebDAV cloud storage entry pointing at
`https://your-server/webdav/`, with the username and password of a KOReader credential from
**Account → KOReader credentials**; then point the statistics plugin's cloud sync at it. The exact
menu path moves between KOReader releases, so go by the names rather than by the order.

What the endpoint will accept is deliberately almost nothing:

- one directory per account, which nothing can name but the account that owns it;
- one file name in it, `statistics.sqlite3`, and no other;
- capped at `WEBDAV_MAX_MEGABYTES`;
- and kept only if it really is a SQLite database with KOReader's statistics tables in it.

An upload is written beside the stored copy and only replaces it once all of that passes, so a sync
that is cut off half way leaves the previous copy intact. What is stored is returned byte for byte,
because the plugin downloads it and merges its own history into it — a server that rewrote the file
would be handing back something the device never wrote.

Everything the endpoint turns down is logged, which is how to find out what a KOReader release wants
that this does not offer.

Note that the file is only *received* today. Reading the reading times out of it is separate work.

## About the mail

There are three messages KOsync writes itself: the note that an account has earned an achievement,
and the weekly and monthly summaries of its own reading. Everything else that arrives from this
server — verification, password reset, the confirmation of an address change — is PocketBase's, from
its own templates.

All of it needs the SMTP settings in the superuser interface at `/_/` under **Settings → Mail**.
Without them PocketBase falls back to the local `sendmail`, which a container does not have, and
sending fails with a line in the log.

The achievement notice asks twice before it goes out. `ENABLE_ACHIEVEMENT_MAIL` is the operator's
switch, for a server that should send nothing of its own; the account's own switch is under
**Account** in the web interface, and it is off until somebody turns it on. An account created by a
script or in the superuser interface therefore stays quiet, while one registered in the browser has
already asked. Nothing is sent to an unconfirmed address, which also keeps the placeholder addresses
the legacy import creates from being written to forever.

One message goes out per statistics batch rather than one per badge. A first evaluation of an
account that has been reading for years earns a dozen tiers at once, and a dozen mails about it would
be a bug rather than a celebration.

The summaries work the same way and start from further off: no account has a cadence until it picks
one under **Account**, and the migration that added the setting deliberately does not backfill it.
`ENABLE_SUMMARY_MAIL` is the operator's half. A summary covers the week or month that has *finished*
and arrives at eight in the morning in the account's own timezone, so the job that sends them runs
hourly and on most runs finds nobody to write to. A period with no reading in it is not mailed —
there is nothing to report, and being told so weekly would be worse than silence.

Which period an account was last written to about is stored on the account, so a server that was
switched off over the weekend still sends Monday's summary when it comes back, once.

## Settings that are gone

If you are coming from KOsync 1:

| KOsync 1 | Now |
| --- | --- |
| `DATABASE_FILE` | `kosync serve --dir` (PocketBase manages the file) |
| `LISTEN_ADDRESS` | `kosync serve --http` |
| `LOG_TO_FILE`, `LOG_FILE`, `DEBUG_LOG` | PocketBase logs, viewable in the superuser interface |
| `DISABLE_WEBSOCKET_API` | gone, realtime is a PocketBase subscription |
| `PRINT_CRYPTO_KEYS`, `CRYPTO_KEYS_SEED`, `JWT_DURATION` | PocketBase issues and manages the tokens |
| `ENABLE_TRUSTED_PROXIES`, `TRUSTED_PROXIES`, `PROXY_IP_VALIDATION` | superuser interface, "Application" settings |
| `DISABLE_REGISTRATION` | unchanged, but it now refers to the web interface; KOReader can never register |
