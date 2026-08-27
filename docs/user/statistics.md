# The reading record

Everything on this page is counted from what your devices actually reported. Nothing is estimated
and nothing is imported from anywhere else, which is the reason the numbers are worth having and
also the reason they start at zero on the day you set the server up.

## The dashboard

Three numbers across the top. **Total Documents** is how many books your devices have ever synced,
which is not how many you uploaded — see [reading.md](reading.md) for the difference. **Average
Progress** is how far through you are, averaged over all of them; it is a shelf-wide number, so a
pile of books opened once holds it down, which is arguably the point of it. **Recent Read Time**
is hours and minutes in the window the chart below is showing, and that window is the last 14 days
until you change it.

## The chart

**Reading Statistics (Last N Days)**, over 7, 14, 30 or 60 days, with three lines on it. *Updates*
counts how many times your devices reported in, which is a rough shape of how much you read.
*Progress Increase (%)* is how much further through your books you got that day. *Reading Time
(min)* is minutes, where the reading was long enough to be measured as time.

Days you did not read are drawn as zero rather than skipped, so a gap looks like a gap. Changing
the range changes **Recent Read Time** with it, since the two read the same days.

Whoever runs the server decides how far back the daily record is kept; three months is the
default, and older days are either folded into totals or dropped. Achievements you have already
earned are not affected by this (see below).

## Days are counted in your timezone

Set **Account → Reading days are counted in** to where you live.

This is what decides which day a reading session belongs to, and everything built on days rests on
it: the chart, the streak, "best day", the late-night and early-morning counts. Your devices never
say what time they think it is, so this setting is the only thing that tells KOsync when your day
started. Left wrong, a session at eleven at night can land on tomorrow, and a streak can break on
a day you did read.

Changing it recomputes every day you have ever read. Nothing is lost, but numbers move: an evening
that used to count as the next day moves back, which can join two streaks into one or split a
day's pages across two.

## Per book

A book's own page has what the reading came to for that book alone: **Time spent**, **Pages read**,
**Days read**, **Best day**, and a chart of the days you read it on. It appears once a device has
reported something for that book — see [library.md](library.md).

## Achievements

Eight of them, three tiers each — bronze, silver, gold. They are cats.

| Achievement | Counts | Bronze | Silver | Gold |
| --- | --- | --- | --- | --- |
| First Pounce | Books you have read to the end | 1 | 10 | 50 |
| Page Turner | Pages read, counted from your own reading | 1 000 | 10 000 | 100 000 |
| Shelf Inspector | Books uploaded to your library | 10 | 50 | 200 |
| Night Prowler | Nights you were still reading after midnight | 1 | 25 | 100 |
| Lap Warmer | Your longest run of days without missing one | 7 | 30 | 100 |
| Sunbeam Sitter | Mornings you were reading before eight | 1 | 25 | 100 |
| The Long Sit | The most pages you have read in one day | 100 | 250 | 500 |
| Nine Lives | Books you finished and then began again | 1 | 5 | 20 |

Each row shows where you are against the next tier — `1,240 of 10,000 pages` — or, once there is
no next one, `every tier earned`.

Some fine print, which is mostly there to stop these looking broken.

"After midnight" ends at five in the morning, and "before eight" starts there. Reading at three is
still last night; reading at six is an early start. The two bands meet and do not overlap, so one
session is never counted as both.

Finished means having reached the end once, not being at the end now — a book you have started
again still counts as finished, which is what makes Nine Lives possible at all. Begun again means
back near the front, inside the first tenth, since re-opening a finished book leaves you on the
last page and that is not the same thing as reading it.

An achievement, once earned, is never taken away. It was true when it was measured, so if the
daily record ages out from under a streak, the streak you earned stays earned.

They are recalculated as your reading arrives, so a tier can appear while you are looking at the
page.

## Mail

Two kinds, both to your account's email address and both only once you have confirmed that
address. An unconfirmed address is one nobody has proved they can read, so the server does not
write to it at all.

The first arrives when you earn something. One message per batch, so earning three at once is one
mail and not three: *You earned Night Prowler, bronze*, or *You earned 3 new achievements*.

The second is a summary of your reading, off unless you ask for it. **Account → Send me a summary
of my reading** offers Never, Every week and Every month.

> Pages, hours, the books you were in and anything you earned. It arrives in the morning after the
> week or month has ended, and a period you did not read in is not mailed at all.

The subject says the headline: *1,204 pages last week*, or *4,880 pages in July*.

A server with no mail set up sends nothing at all, and summaries can be switched off for the whole
server, in which case the setting is still there and simply has no effect. Achievement notices are
not tied to the summary setting: turning summaries off leaves them on.
