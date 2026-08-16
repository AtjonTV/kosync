# Migrating from legacy KOsync

KOsync 26.08.0 can import a legacy KOsync database. Devices keep working afterwards without being touched,
because their credentials are carried over unchanged.

## Before you start

Stop the old server, and take a copy of its `kosync.db`. The import only reads the file, but a copy
costs nothing.

## The import

```bash
kosync import-legacy --file ./kosync.db --dry-run
kosync import-legacy --file ./kosync.db
```

Always do the dry run first. It goes through the whole import, reports exactly what would happen and
then rolls everything back.

| Flag | Default | Meaning |
| --- | --- | --- |
| `--file` | `./kosync.db` | the legacy database |
| `--owner-email` | none | attach everything to this existing account instead of creating one per legacy user |
| `--include-deleted` | `false` | also import the rows the old server had marked as deleted |
| `--dry-run` | `false` | report without writing |

Running the import twice is safe. A legacy user whose credential already exists, and a document that
was already imported, are skipped instead of duplicated.

## What happens to your users

A legacy user was one thing: a username and an MD5 password, used by both the device and the web
interface. Here those are two different things, so each legacy user becomes:

- a **KOReader credential** with the same username and the same stored password, so the device keeps
  syncing, and
- an **account** for the web interface that owns it.

Since legacy users had no email address, the import creates one account per user with an address at
`invalid.local` and a generated password, and prints them **once**:

```
LEGACY USER  EMAIL                  PASSWORD
alice        alice@invalid.local    hZ3k9QmT2rXv8Lp4
```

Hand those out. Each person should sign in and open **Account → Sign in details**, where they can:

- **set their own password**, which takes effect immediately and signs out every other session, and
- **change the address** to a real one, so account recovery works. A confirmation link goes to the
  new address and nothing changes until it is opened, which needs the server to be able to send
  mail. On an instance without mail, a superuser can change the address at `/_/` instead.

Neither touches the KOReader credential, so no device has to be reconfigured.

## Single user instances

If the old server was only ever used by you, register an account in the web interface first and then:

```bash
kosync import-legacy --file ./kosync.db --owner-email you@example.com
```

Everything is attached to that account and nothing is generated.

## What changes for a device

Only the server address. KOReader is pointed at `https://your-host/koreader` instead of
`https://your-host`; the username and password stay the same.

## What is not imported

- Deleted rows, unless `--include-deleted` is given.
- The legacy `created_at` and `updated_at` bookkeeping. The reading timestamps are converted (the old
  format counted 1/10000 of a second, PocketBase stores milliseconds).
- Statistics. They are recomputed from the imported progress at the end of the import, which is why
  the command finishes with a line about how many days it computed.

## Why some statistics differ afterwards

The numbers are computed the same way as before, and on a database of real data 82 of the 83
comparable days come out identical to the digit. The days that do differ, differ for one of three
reasons, all of them expected:

1. **Days that involved a deleted document.** The old statistics query counted the history of a
   deleted document but not its final state, which is an inconsistency in that query. The import
   leaves deleted documents out entirely, so such a day now shows less. `--include-deleted` brings
   them back as ordinary documents.
2. **Restarted books.** Starting a book over used to subtract from the day's progress, sometimes
   pushing it far negative. Now a document can only contribute zero or more.
3. **A second of reading time, occasionally.** The old format could tell two updates apart down to a
   ten thousandth of a second, PocketBase stores milliseconds. Two updates less than a millisecond
   apart therefore collapse into one, which can move a day's estimated reading time by a second.
