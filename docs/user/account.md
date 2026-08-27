# Your account

**Account** in the menu. Everything on this page is there, in the order it appears.

There are two kinds of credential in KOsync and it is worth getting them straight before anything
else. Your sign-in, an email address and a password, is for this web interface and nothing else;
no device ever uses it. A KOReader credential, a username and a password, is what a reader uses.
Your devices keep using theirs whatever you do to your sign-in, and the other way round.

They are separate on purpose. KOReader can only protect what it stores with MD5, which is not
something to hand an account password to, and a credential that lives on a device you carry around
should be one you can revoke without changing how you sign in.

## Sign in details

### Email address

Type the new one and **Send confirmation**. A link goes to the *new* address and nothing changes
until you open it, which is the only way to be sure the address works.

This needs the server to be able to send mail. If it cannot, whoever runs it can change the address
for you in the admin interface.

Confirm the address once it is set. Until you do, the server sends you nothing at all — no
achievement notices, no summaries, and no password reset on the day you need one. If yours came
over from an old KOsync import, it may be a generated address that cannot receive mail; the page
says so, and it is worth fixing before you need it.

### Password

Current password, new password, **Change password**.

Changing it signs out every other session and leaves you signed in here. Your devices are not
affected — different credentials.

Forgotten it? **Forgot password** on the sign-in page sends a link to your address, which is the
one case where a confirmed address is the difference between an inconvenience and an account you
cannot get back into.

### Timezone

Set **Reading days are counted in** to where you live.

Your devices never say what time they think it is, so this is the only thing that tells KOsync when
your day started, and everything counted by days rests on it. Changing it recomputes every day you
have ever read: nothing is lost, but numbers move. See [statistics.md](statistics.md).

### Summary mail

**Send me a summary of my reading** offers Never, Every week and Every month. It is off unless you
ask for it, and described in [statistics.md](statistics.md).

## Devices

Every reader that has synced appears here on its own, with when it was last seen.

The name comes from KOReader, which usually calls a device something short rather than something
recognisable. Rename it to whatever you call the thing; that name is then used wherever a device is
named — on a document, in the merge dialog, everywhere — and the page still shows what the device
calls itself underneath.

Renaming is cosmetic and is not a security control. To stop a device syncing, disable the
credential it uses.

## KOReader credentials

The username and password you type into a reader. **Add credential**, with a username, a password,
and optionally the name of the device it is for.

The password is shown once, right after you create it, and never again — the server keeps only a
hash of it. Write it down before you dismiss the message. Lose it and you change it rather than
recover it.

Each row shows the username, the device you named it for, when it was **Last used**, and whether
it is **Active** or **Disabled**. There are four things you can do to one.

**Rename** changes the device it is named for, which is cosmetic and only there so you can tell
the rows apart. **Change password** issues a new one, shown once the same way; the device stops
syncing until you type it in. **Disable** keeps the credential and refuses every attempt to use
it, which is the one to reach for when a device goes missing — instant, reversible, and it costs
your other devices nothing. **Delete** is for good, though the reading that arrived through it is
not: that lives on the documents and stays.

Nothing here touches your books, your documents or your statistics.

## Registration and getting in

Accounts are made in the web interface and never on a device. KOReader has a register button of its
own; it will not work against this server, which is deliberate — an account here is an account with
an email address behind it.

Some servers have registration switched off, and say so plainly when you try. Then an account is
something to ask whoever runs the server for.
