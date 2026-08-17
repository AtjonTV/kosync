//
// File:        internal/schema/schema.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

// Package schema holds the collection and field names of the KOsync data model.
//
// Everything that reads or writes records goes through these constants so a
// renamed collection breaks the build instead of failing at runtime.
package schema

// Collection names.
const (
	// CollectionUsers is the built-in PocketBase auth collection used for WebUI accounts.
	CollectionUsers = "users"

	// CollectionKoreaderAccounts holds the per-device credentials KOReader authenticates with.
	CollectionKoreaderAccounts = "koreader_accounts"

	// CollectionDocuments holds the current reading progress of a document.
	CollectionDocuments = "documents"

	// CollectionDocumentHistory holds every superseded state of a document.
	CollectionDocumentHistory = "document_history"

	// CollectionDocumentAliases holds the document hashes that used to be
	// documents of their own and were merged into another one. A push arriving
	// under a retired hash lands on the document it was merged into, which is
	// what keeps a merge from undoing itself the next time the device syncs.
	CollectionDocumentAliases = "document_aliases"

	// CollectionReadingDays holds the precomputed daily reading statistics.
	CollectionReadingDays = "reading_days"

	// CollectionReadingMonths holds the monthly rollup of aged out daily statistics.
	CollectionReadingMonths = "reading_months"

	// CollectionAnalyticsQueue holds pending (owner, date) statistics recomputations.
	CollectionAnalyticsQueue = "analytics_queue"

	// CollectionBooks holds uploaded EPUBs with the metadata read out of them.
	CollectionBooks = "books"

	// CollectionDevices holds one row per device that has ever pushed progress,
	// so the thing can be given a name a person recognises.
	CollectionDevices = "devices"

	// CollectionAchievements holds what an account has earned. A row appears
	// when a threshold is first crossed and is never removed: an achievement
	// records that something happened, not that it is still true.
	CollectionAchievements = "achievements"

	// CollectionPageReads holds what a device's own statistics database says
	// about individual page turns: which page of which document, when, and for
	// how long. It is measurement rather than inference — the sync protocol
	// carries no clock, so everything else here has to reason about reading
	// times from the moments pushes happened to arrive.
	CollectionPageReads = "page_reads"

	// CollectionBookCollections holds the shelves an account puts together by
	// hand: a name, and the books that belong on it.
	//
	// It is not called "collections" because PocketBase calls its own tables
	// that, and a KOsync collection named after the thing it is stored in would
	// make every sentence about either of them ambiguous.
	CollectionBookCollections = "book_collections"

	// CollectionReadingBookDays holds the daily reading statistics of a single
	// book. It is a separate collection rather than a grouping of reading_days
	// because a day's reading time cannot be split across books without losing
	// the time spent switching between them.
	CollectionReadingBookDays = "reading_book_days"
)

// Shared field names.
//
// FieldCreated and FieldUpdated are the autodate fields every KOsync collection
// carries; PocketBase names them and maintains them.
const (
	FieldOwner      = "owner"
	FieldDate       = "date"
	FieldLastReadAt = "last_read_at"
	FieldCreated    = "created"
	FieldUpdated    = "updated"
)

// users field names.
//
// FieldTimezone is an IANA name such as "Europe/Vienna". It is what a reading
// day is reckoned in: every timestamp here is UTC, because the sync protocol
// carries no clock, so this is the only thing that says when the reader's day
// began.
const FieldTimezone = "timezone"

// FieldAchievementMail is whether the account wants to be told by mail when it
// earns something. It is positive rather than a mute switch so that an account
// created outside the browser is quiet until it asks not to be: mail nobody
// asked for is the kind of default worth getting the wrong way round.
const FieldAchievementMail = "achievement_mail"

// FieldSummaryMail is how often the account wants a summary of its own reading,
// and FieldSummarySent is the last period one was sent for.
//
// The cadence is a choice rather than a switch because the two answers are
// genuinely different: a week is a report on a habit, a month is a report on a
// book. Unset means neither, which is what an account that has never been asked
// should get.
//
// FieldSummarySent is what makes the sending idempotent. A server that was
// switched off over the weekend has still not sent last week's summary when it
// comes back, and the only way to know that is to have written down which
// period the last one covered.
const (
	FieldSummaryMail = "summary_mail"
	FieldSummarySent = "summary_sent"
)

// The cadences FieldSummaryMail may hold. An empty value means the same as
// SummaryOff: a field nobody has set cannot be a promise to send anything.
const (
	SummaryOff     = "off"
	SummaryWeekly  = "weekly"
	SummaryMonthly = "monthly"
)

// koreader_accounts field names.
const (
	FieldUsername = "username"
	FieldPassword = "password"
	FieldLabel    = "label"
	FieldDisabled = "disabled"
	FieldLastUsed = "last_used"
)

// documents, document_history and document_aliases field names.
//
// An alias row is only FieldOwner, FieldDocument and FieldDocumentRef: the
// retired hash, and the document it now resolves to.
const (
	FieldDocument        = "document"
	FieldDocumentRef     = "document_ref"
	FieldTitle           = "title"
	FieldCurrentLocation = "current_location"
	FieldProgress        = "progress"
	FieldLastDevice      = "last_device"
	FieldLastDeviceId    = "last_device_id"
	FieldSourceAccount   = "source_account"

	// FieldBook links a document to the uploaded EPUB it is progress through,
	// empty until a matching book exists.
	FieldBook = "book"

	// FieldFilename and FieldDocumentAuthors are what the device says the file
	// is, sent only when KOReader's "send document metadata" setting is on. They
	// describe a document that may never be matched to a book, which is the case
	// they exist for: an unmatched document has no other name than its hash.
	//
	// FieldFilenameHash is the KOReader filename hash of FieldFilename, stored so
	// that a book can be matched to it by an indexed comparison rather than by
	// hashing every row.
	FieldFilename        = "filename"
	FieldFilenameHash    = "filename_hash"
	FieldDocumentAuthors = "authors"
)

// devices field names.
//
// FieldDeviceId is KOReader's own identifier and is what everything groups by,
// because it survives a rename. FieldReportedName is the name the device last
// sent, and FieldName is the one its owner chose — seeded from the reported one
// and never overwritten afterwards, so a rename is not undone by the next push.
const (
	FieldDeviceId     = "device_id"
	FieldReportedName = "reported_name"
	FieldName         = "name"
	FieldLastSeen     = "last_seen"
)

// achievements field names.
//
// FieldValue is what the measure stood at when the tier was crossed, kept
// because it is the only record of it: the measure is recomputed from live data
// and will have moved on by the time anybody looks.
const (
	FieldRule     = "rule"
	FieldTier     = "tier"
	FieldValue    = "value"
	FieldEarnedAt = "earned_at"
)

// page_reads field names.
//
// FieldDocument is shared with the documents collection and means the same
// thing: KOReader's own hash of the file. A statistics database calls it md5,
// and it is the same string, which is what makes this exact rather than a guess.
//
// FieldStartedAt is when the page was turned to and FieldDuration is how long it
// stayed open, both from the device, both in the device's own reckoning of the
// clock — which is the only reckoning there is of when reading actually
// happened.
const (
	FieldPage      = "page"
	FieldStartedAt = "started_at"
	FieldDuration  = "duration"
)

// reading_days and reading_months field names.
const (
	FieldUpdateCount      = "update_count"
	FieldProgressIncrease = "progress_increase"
	FieldReadingTime      = "reading_time"
	FieldDocumentsTouched = "documents_touched"
	FieldPagesRead        = "pages_read"
	FieldComputedAt       = "computed_at"
	FieldMonth            = "month"
	FieldDaysActive       = "days_active"
)

// books field names.
const (
	FieldFile        = "file"
	FieldCover       = "cover"
	FieldAuthors     = "authors"
	FieldLanguage    = "language"
	FieldIdentifiers = "identifiers"
	FieldPageCount   = "page_count"
	FieldWordCount   = "word_count"
	FieldContentHash = "content_hash"

	// FieldSeries and FieldSeriesIndex are the series a book belongs to and
	// where in it this volume sits. Two columns rather than one string like
	// "A Song of Ice and Fire #2", because a series shelf has to sort by the
	// number and group by the name, and neither is possible once they are
	// spelled into one value.
	//
	// FieldSubjects is what the file says the book is about, as a JSON array.
	// It is stored as the publisher wrote it: the values are of very mixed
	// quality — on the reference library 143 of 202 distinct subjects belong to
	// exactly one book — so what they are good for is search and seeding a
	// hand-made collection, not a shelf of their own.
	FieldSeries      = "series"
	FieldSeriesIndex = "series_index"
	FieldSubjects    = "subjects"

	// FieldFileSize is how many bytes the uploaded EPUB takes, stored because
	// nothing else records it: PocketBase keeps the name of a file on the
	// record and its size only on the filesystem, and asking the filesystem
	// once per book is not a question a quota can afford to ask on every
	// upload. The extracted cover is not counted; it is generated, small, and
	// nobody chose to store it.
	FieldFileSize = "file_size"

	// FieldHashBinary and FieldHashFilename are the two document hashes
	// KOReader identifies a book by. They are separate indexed columns rather
	// than one JSON blob because matching a progress push means looking a book
	// up by either of them.
	//
	// FieldHashCatalog is the filename hash again, of the name the OPDS catalog
	// serves the book under rather than the name it was uploaded with. A reader
	// set to the filename method hashes the name on its own disk, and that is
	// the name the catalog put there — which is a different string from the one
	// the uploader happened to use.
	FieldHashBinary   = "hash_binary"
	FieldHashFilename = "hash_filename"
	FieldHashCatalog  = "hash_catalog"

	// FieldMeasuredPages and FieldMeasuredDevice hold the page count recovered
	// from the progress a device pushed, and which device it came from. Zero
	// pages means no measurement was possible and FieldPageCount is all there is.
	//
	// FieldMeasuredThrough is how far into the reading that measurement looked —
	// the newest push it saw, not the moment it ran. Reading timestamps come from
	// the device and are always in the past, so a wall clock there would mean no
	// book is ever measured a second time.
	FieldMeasuredPages   = "measured_pages"
	FieldMeasuredDevice  = "measured_device"
	FieldMeasuredThrough = "measured_through"
)

// book_collections field names.
//
// The shelf's own name is FieldName, shared with the devices collection because
// it means the same thing there: what its owner chose to call the thing.
//
// FieldBooks is what is on the shelf, in the order it was put there. A relation
// field keeps the order of its ids, and that order is half the point of a
// hand-made shelf: a reading list is a sequence, not a set.
const (
	FieldDescription = "description"
	FieldBooks       = "books"
)

// analytics_queue field names.
//
// FieldAttempts counts how often a queued day has been tried and failed, and
// FieldRetryAfter is the moment it may be tried again. A day that cannot be
// recomputed is not an error to give up on — the data behind it is still there
// and the next attempt may well succeed — but it must not be retried on every
// tick either, because the queue is drained oldest first and a row that always
// fails would otherwise be the only row ever looked at.
const (
	FieldAttempts   = "attempts"
	FieldRetryAfter = "retry_after"
)

// OwnerRule restricts a collection to the records owned by the authenticated
// WebUI user. It is deliberately also used for realtime subscriptions, which
// PocketBase filters with the very same list rule.
const OwnerRule = "@request.auth.collectionName = '" + CollectionUsers + "' && " + FieldOwner + " = @request.auth.id"
