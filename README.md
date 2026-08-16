# KOsync

KOsync is a progress sync server for [KOReader](https://koreader.rocks) written in Go.

This is **26.08.0**, the current version. It is a rewrite, built on
[PocketBase](https://pocketbase.io), and it replaces the 1.x series, which is now the
[legacy KOsync](https://git.obth.eu/atjontv/kosync/-/tree/legacy-main). A legacy database can be imported and the
devices syncing against it keep working — see [docs/migration.md](docs/migration.md).

## Why?

The [official KOReader progress sync server](https://github.com/koreader/koreader-sync-server) is
written in Lua using OpenResty.  
For deployment it needs Nginx with OpenResty as well as Redis as database.

KOsync wants to be simpler by not having any dependencies besides the OS itself.  
(If you need TLS, a reverse proxy is also required, I recommend [Caddy](https://caddyserver.com))

In addition to requiring Nginx, OpenResty and Redis, the official server is not very maintained.  
The last feature adding commits was around 2016.

It also wants to be worth having beyond the sync itself. A reading position is only the smallest
thing a device knows: KOsync keeps the books, the history behind that position, and the record of
when the reading actually happened.

## KOsync vs [KOReader Sync Server](https://github.com/koreader/koreader-sync-server)

You may choose KOsync over [KORSS](https://github.com/koreader/koreader-sync-server) due to the
following differences:

- Actively maintained and open for feature requests
- Syncs reading progress with the same protocol, so no device has to be reconfigured beyond its
  address
- A [web interface](#webui) with your documents, their history, your statistics and your library
- **Holds your books.** Upload an EPUB and it is matched to the reading you have already done, with
  the cover, the metadata and a page count measured from your own device's progress
- **Serves the library as an OPDS catalog**, browsable by author, series, language or a collection
  you put together yourself. A book downloaded that way is recognised the moment you start reading
  it
- **Takes your statistics off the device by itself.** KOReader can sync its own page-turn database
  to a WebDAV target, and this server is one — with the credential the device already has. What
  arrives is real measurement, including the days you read with the WiFi off
- **Keeps a history** of every position a document went through, and can put a document back into an
  earlier one
- **Merges split reading.** Two devices reading two copies of the same book report two documents;
  fold them into one and they sync with each other from then on
- **Hands out achievements**, drawn as comic-book cats, and will write to you weekly or monthly
  about your own reading if you ask it to
- Written in Go and deploys as a single executable
- Simple SQLite database and ENV configuration instead of Redis

Additional differences that should be known:

- KOsync is licensed under `EUPL-1.2 or later` compared to KORSS, which is `AGPL-3.0 or later`
- Simple deployment via Docker
- Requires a Reverse Proxy for TLS

## KOsync 26.08.0 vs legacy KOsync

The protocol is the same one and the reading is carried over, so this is an upgrade rather than a
different server. What changed:

- **The device address gains a prefix.** KOReader is pointed at `https://your-host/koreader` instead
  of `https://your-host`; the username and password stay the same
- **An account and a device credential are two different things** now, because KOReader can only
  protect a password with MD5 — see [Accounts and devices](#accounts-and-devices)
- **Registration happens in the web interface**, not from the device, since a credential has to
  belong to an account
- The hand written database layer, the JWT handling and the custom WebSocket protocol are gone;
  PocketBase does all three, and realtime is an ordinary subscription
- Statistics are precomputed rather than derived by a recursive query on every request
- The library, the OPDS catalog, the collections, the achievements, the mail and the statistics
  sync are all new; legacy KOsync synced a position and showed it to you

### Simplicity

**Simple Code**  
KOsync is written in Go, with the web interface compiled into the same binary.  
All you need to run KOsync is bundled into a single executable. No Redis, no Nginx, no Node.

See [docs/build.md](docs/build.md) for build and deployment instructions.

**Simple Datastore**  
KOsync stores all user data in a PocketBase-managed SQLite database, next to the uploads and the
backups, while deployment settings are stored in an environment file.

```bash
kosync serve --http=0.0.0.0:8080 --dir=/pb_data
```

On first start the server prints a link for creating the first superuser. The superuser interface
lives at `/_/` and is PocketBase's own; the reading interface is at `/`.

### Accounts and devices

There are two kinds of credentials, and the difference matters:

| | Account | KOReader credential |
| --- | --- | --- |
| Used for | the web interface | a device |
| Created in | the web interface | the web interface, under Account |
| Password | bcrypt, with recovery by mail | whatever the device sends, hashed with MD5 first |
| How many | one per person | as many as you like, one per device or one for all |

A device credential can never be used to sign in to the API, and can be revoked one at a time
without touching the account or the reading it pushed.

### Setting up a device

1. Register in the web interface.
2. Open **Account → KOReader credentials** and add one. The password is shown once.
3. In KOReader, set **Custom sync server** to `https://your-host/koreader`.
4. Log in with the credential from step 2.
5. Enable **automatically keep documents in sync**, set **periodically sync every # pages** to 2 and
   **Document matching method** to "Binary".
6. Optionally, add `https://your-host/opds` under **Search → OPDS catalog**, with the same
   credential, to browse and download your library from the device.
7. Optionally, add `https://your-host/webdav/` as a **WebDAV** cloud storage entry, again with the
   same credential, and point the statistics plugin's cloud sync at it.

The same steps, with the addresses of your own server filled in, are on the front page of the web
interface until you have signed in.

### Configuration

See [docs/config.md](docs/config.md)

### Database

See [docs/database.md](docs/database.md)

### Backups

PocketBase takes them, locally or to S3, on a schedule or on demand: **Settings → Backups** in the
superuser interface.

See [docs/database.md](docs/database.md#backups)

### API Specification

See [docs/api.md](docs/api.md) for the KOReader protocol, the collection API, the OPDS catalog and
the WebDAV target.  
See [docs/analytics.md](docs/analytics.md) for how the reading statistics are worked out.

### WebUI

Vue 3 and PrimeVue, built with Bun and embedded into the server binary, so there is nothing to serve
separately: the dashboard, the library, the collections, the statistics and the account settings are
all at `/`.

### Migrating from legacy KOsync

See [docs/migration.md](docs/migration.md)

### The rewrite

See [docs/rewrite-plan.md](docs/rewrite-plan.md) for what was built, in what order, and why.

## AI Usage

Most of this version (26.08.0 and newer) was built and changed using AI Tools.  
The primary tool used is Anthropic Claude Opus 5.

Each Git commit with AI contribution is marked with `AI-Agent` and `AI-Model`.  
Commits without are human made.

## License

KOsync is licensed under the [European Union Public License v1.2 or later](/LICENSE.txt)

Contributions are governed by the [OBTH Machine Policy](MACHINE_POLICY.md).
