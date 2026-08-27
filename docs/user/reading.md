# Your reading

This page is about the part of KOsync that does the actual job: the position your devices report,
where it is kept, and what to do on the day two devices disagree about which book they are in.

## How a position arrives

Your reader sends the server two things every couple of pages: which book, and where in it. The
server writes them down and stamps them with the time they arrived. That is the whole protocol.

Nothing is pushed the other way. When you open a book, KOReader asks the server where you were and
offers to jump; the server never reaches into a device. So a device that has been off for a week
is not "behind" — it asks on the next open and catches up in one step.

The time is the server's, not the device's, so a reader with a wrong clock does not put your
reading on the wrong day. What does move it is a device that syncs long after the fact: an
e-reader that spent the weekend in a bag and connects on Monday reports Monday's reading, because
Monday is when the server heard about it.

## Books and documents

Two lists on this site look like the same list and are not. **Library** is the books you uploaded:
files, covers, blurbs, page counts. **Documents** is what your devices report — one entry per file
a reader has ever synced, with the position, the device that last touched it, and when.

A document exists whether or not you uploaded anything, because the sync works on its own.

The two are joined by a hash — a short string a reader computes from the file. Upload a book whose
hash matches a document and they become one thing: the document's progress starts showing on the
book's cover, and the book's reading gets a chart. The **Documents** page shows the hash of each
entry, and marks the ones nothing in your library matches: *No uploaded EPUB matches this
document*.

An unmatched document is not a problem. It syncs perfectly. It is just a row that says a hash
instead of a title.

## Why the same book can appear twice

Because the hash is computed from the file, and two files are two files.

You will end up with two documents for one book when the copy on one device is not byte-for-byte
the copy on the other: a re-download, a different shop, a conversion, or the same title fetched
twice. Then the reading is split — two rows, two positions, and per-book statistics that only ever
see half of it.

Switching **Document matching method** between "Binary" and the filename method does it too. The
server understands both, but they are different strings, so a device set to one does not recognise
what a device set to the other reported. Pick one and use it on every device.

The prevention is to put the same file on every device — which is what the OPDS catalog is for,
since a book downloaded from your own library is by definition the file you uploaded.

## Merging

The cure is **Merge**, on the row of the document you want to keep.

Pick the document to merge into, tick the ones to fold in, and the server joins them. The most
recent position wins, because you are somewhere in one book and it is wherever you last were. The
positions that lose are written to the history first, so an unwanted merge is one restore away.
And the hashes of the folded documents become aliases for the survivor, so the device that
reported one goes on syncing against the joined document — without that, the next push would
recreate exactly what you just merged away.

The kept document keeps its own title and its own book; it takes those on only where it had none.
Merging does not touch your library, and your statistics are recounted for the days involved
rather than added up twice.

## History

Every position that has ever been replaced is kept. Open **History** on a document and you get the
list: reading progress, the title the device called it at the time, which device, and when.

That means the history is complete, not a sample — every push archives the state it replaced.

**Restore** puts an old position back and keeps the current one in the history, so restoring is
itself undoable. Use it after a stray tap sends you to the end of a book, or after a merge you did
not mean. **Delete** on a history row removes that one entry and changes nothing about where you
are.

## Deleting a document

The trash can on a document removes the entry and its history. The device that was reporting it
will simply create it again the next time you open that book — deleting a document is forgetting a
position, not telling a reader to stop.

To take the file off the server instead, delete the **book** in the library. That is the other
direction, and it leaves the document alone; see [library.md](library.md).

## Reading on several devices

Nothing needs configuring for this beyond doing steps 3 to 7 of
[getting-started.md](getting-started.md) on each reader. A few things are worth knowing anyway.

Neither device has to be online at the same time as the other; the server is the meeting point,
not a connection between them. And whichever speaks last wins, since every push replaces what was
there. So if you read on two devices while one of them is offline, the offline one overwrites the
position when it finally connects, even though you read it earlier — the position it replaced is
in the history, one **Restore** away.

Page counts are a property of the device rather than the book. A phone in a large font and an
e-reader in a small one are two different totals for one title. What gets synced is a place in the
text and how far through the book it is, not a page number, so it survives the difference; the
counts you see here are the ones a device measured and reported.

The device named on a document row is the last one to report it, which is how you find out that
the book you thought you were reading on the Kobo has been syncing from the phone all along. Each
reader appears under the name KOReader gives it, which is usually short rather than recognisable;
rename them in **Account → Devices** and the new name is used everywhere. See
[account.md](account.md).
