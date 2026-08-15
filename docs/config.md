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
| `ENABLE_OPDS` | `true` | serve the library as an OPDS catalog at `/opds` |
| `OPDS_PAGE_SIZE` | `50` | how many books one page of a catalog feed holds |
| `ENABLE_ACHIEVEMENT_MAIL` | `true` | let the server tell an account what it has earned |

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

## About the mail

There is only one message KOsync writes itself: the note that an account has earned an achievement.
Everything else that arrives from this server — verification, password reset, the confirmation of an
address change — is PocketBase's, from its own templates.

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
