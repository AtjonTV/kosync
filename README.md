# KOsync v2

KOsync is a progress sync server for [KOReader](https://koreader.rocks), written in Go on top of
[PocketBase](https://pocketbase.io).

This is the rewrite of [KOsync 1](https://git.obth.eu/atjontv/kosync). It keeps the KOReader sync
protocol and the web interface, and replaces the hand written database layer, the JWT handling and the
custom WebSocket protocol with what PocketBase already does well.

## What it does

- **Syncs reading progress** with KOReader, using the same protocol as the official sync server.
- **Shows a dashboard**: documents, reading progress, per document history, and reading statistics.
- **Holds your books.** Upload an EPUB and it is matched to the reading you have already done, with
  the cover, the metadata and a page count measured from your own device's progress.
- **Serves the library as an OPDS catalog**, so a device can browse and download from it. A book
  downloaded that way is recognised the moment you start reading it.
- **Keeps a history** of every position a document went through, and can put a document back into an
  earlier one.
- **Merges split reading.** Two devices reading two copies of the same book report two documents;
  fold them into one and they sync with each other from then on.
- **Hands out achievements**, drawn as comic-book cats, for pages read, books finished, reading
  streaks, the books you went back to, and the nights and mornings you read at either end of.
- **Tells you when you earn one**, by mail, if you ask it to and the server can send any.
- **Writes to you about your reading**, weekly or monthly if you pick a cadence: pages, hours, the
  books you were in and anything you earned. A week you did not read in is not mailed.
- **Takes your reading statistics off the device by itself.** KOReader can sync its own page-turn
  database to a WebDAV target, and this server is one — with the credential the device already has.
- **Deploys as a single executable** with the web interface compiled in. No Redis, no Nginx, no Node.

## How the accounts fit together

There are two kinds of credentials, and the difference matters:

| | Account | KOReader credential |
| --- | --- | --- |
| Used for | the web interface | a device |
| Created in | the web interface | the web interface, under Account |
| Password | bcrypt, with recovery by mail | whatever the device sends, hashed with MD5 first |
| How many | one per person | as many as you like, one per device or one for all |

KOReader can only protect its password with MD5, so those credentials are kept apart from the account
password and can be revoked one at a time. A device credential can never be used to sign in to the API.

Registration happens in the web interface only, because a device credential has to belong to an
account and KOReader has no way to ask for one.

## Setting up a device

1. Register in the web interface.
2. Open **Account → KOReader credentials** and add one. The password is shown once.
3. In KOReader, set **Custom sync server** to `https://your-host/koreader`.
4. Log in with the credential from step 2.
5. Enable **automatically keep documents in sync**, set **periodically sync every # pages** to 2 and
   **Document matching method** to "Binary".
6. Optionally, add `https://your-host/opds` under **Search → OPDS catalog**, with the same credential,
   to browse and download your library from the device.

## Running it

```bash
kosync serve --http=0.0.0.0:8080 --dir=/pb_data
```

On first start the server prints a link for creating the first superuser. The superuser interface
lives at `/_/` and is PocketBase's own; the reading interface is at `/`.

See [docs/build.md](docs/build.md) for building, [docs/config.md](docs/config.md) for the settings,
and [docs/migration.md](docs/migration.md) for moving data over from KOsync 1.

## Documentation

- [Building](docs/build.md)
- [Configuration](docs/config.md)
- [Database and collections](docs/database.md)
- [API](docs/api.md)
- [Reading statistics](docs/analytics.md)
- [Migrating from KOsync 1](docs/migration.md)
- [The rewrite plan](docs/rewrite-plan.md)

## License

EUPL-1.2 or later. Contributions are governed by the [OBTH Machine Policy](MACHINE_POLICY.md).
