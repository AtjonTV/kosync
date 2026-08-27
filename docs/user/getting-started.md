# Getting started

From nothing to a device that syncs. The same nine steps are on the dashboard when you are not
signed in, so you can follow them with the addresses already filled in.

## 1. Create an account

Register in the web interface, on the front page. Registration happens here and never on the
device — KOReader has a "register" button of its own, and it will not work against this server.

Some servers have registration switched off. If yours says so, ask whoever runs it for an account.

You will be asked to confirm your address. Do it: an unconfirmed address is one nobody has proved
they can read, so the server will not send anything to it — no achievement notices, no summaries,
and no password reset when you need one.

## 2. Add a KOReader credential

Open **Account → KOReader credentials** and add one. Give it the name of the device it is for
("Kobo Clara"), and note down the username and password: **the password is shown once and never
again**. If you lose it, change it there and put the new one into the device.

This is deliberately not your sign-in. A device credential is stored in plain sight on the reader
and travels with it; your account password does not. A credential can be renamed, disabled or
deleted at any time without touching your account, which is what you want when a device is lost or
sold. See [account.md](account.md) for the rest of it.

You can use one credential on every device or one per device. One per device is worth the small
extra effort: a reader that is lost, sold or handed on can then be cut off on its own, by disabling
that one credential, without every other device needing a new password.

## 3. Point KOReader at the server

In KOReader, open the sync settings and set **Custom sync server** to:

```
https://your-host/koreader
```

The `/koreader` on the end is not optional. KOReader appends its own paths to whatever it is
given, so the address has to end there and not at the site root.

## 4. Log in on the device

Log in with the username and password from step 2 — not with your account's email address.

## 5. Turn on automatic syncing

Enable **automatically keep documents in sync**. Without it, nothing is sent until you ask for it
by hand, which is a thing to forget exactly once per book.

## 6. Sync every 2 pages

Set **periodically sync every # pages** to 2. This is how often the device tells the server where
you are, and it is the difference between picking up the other device on the right page and
picking it up a chapter behind. The requests are tiny.

## 7. Set the matching method to "Binary"

Set **Document matching method** to "Binary". This decides how a device tells the server which
book it is talking about. Binary identifies the file by its contents, so it goes on matching after
you rename the file, and two devices holding the same file agree whatever they each call it. The
alternative hashes the file name, which agrees only as long as every device spells it the same way.

The server understands both, but a device set to one does not match a device set to the other —
so pick one and use it everywhere. See [reading.md](reading.md) for what to do when they have
already disagreed.

## 8. Repeat on every device

Steps 3 to 7, on each reader you use, with the same credential or a new one.

The menu paths move between KOReader releases, so go by the names above rather than by where they
sat on your device last time.

## 9. Read books

That is the setup. Open something and turn a few pages; the dashboard should show the document
within a minute or so.

---

## Optional: get your books from the server

Your library is also an **OPDS catalog**, which is the standard way a reader downloads books. In
KOReader, open **Search → OPDS catalog**, add a catalog with the address

```
https://your-host/opds
```

and the same credential from step 2.

A book downloaded from there is recognised the moment you start reading it, so its progress and
its statistics arrive without you uploading anything or matching anything up by hand. This is the
easiest way to keep a device stocked, and it is the reason the library is worth filling.

The catalog opens on what you are reading now and what was added last, then lets you browse by
author, by series, by language, or by any collection you have arranged yourself. A collection is
served in the order you put it in, which is the point of making one.

## Optional: bring your reading history with you

KOReader keeps its own record of every page you turn, in a database on the device, and it can sync
that file to a WebDAV target. This server is one.

In KOReader, add a cloud storage entry of type **WebDAV** with the address

```
https://your-host/webdav/
```

and the same credential again, then point the statistics plugin's own cloud sync at that entry.
The trailing slash is how KOReader's dialog expects a folder to be written.

Once a device has synced it, the pages and hours it recorded show up in your statistics here —
including everything from before you had an account on this server. Each account gets its own copy
of the file and can see no other.
