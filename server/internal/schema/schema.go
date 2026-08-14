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

// OwnerRule restricts a collection to the records owned by the authenticated
// WebUI user. It is deliberately also used for realtime subscriptions, which
// PocketBase filters with the very same list rule.
const OwnerRule = "@request.auth.collectionName = '" + CollectionUsers + "' && " + FieldOwner + " = @request.auth.id"
