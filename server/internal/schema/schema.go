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

	// CollectionReadingDays holds the precomputed daily reading statistics.
	CollectionReadingDays = "reading_days"

	// CollectionReadingMonths holds the monthly rollup of aged out daily statistics.
	CollectionReadingMonths = "reading_months"

	// CollectionAnalyticsQueue holds pending (owner, date) statistics recomputations.
	CollectionAnalyticsQueue = "analytics_queue"

	// CollectionBooks holds uploaded EPUBs with the metadata read out of them.
	CollectionBooks = "books"
)

// Shared field names.
const (
	FieldOwner      = "owner"
	FieldDate       = "date"
	FieldLastReadAt = "last_read_at"
)

// koreader_accounts field names.
const (
	FieldUsername = "username"
	FieldPassword = "password"
	FieldLabel    = "label"
	FieldDisabled = "disabled"
	FieldLastUsed = "last_used"
)

// documents and document_history field names.
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
	FieldHashBinary   = "hash_binary"
	FieldHashFilename = "hash_filename"
)

// OwnerRule restricts a collection to the records owned by the authenticated
// WebUI user. It is deliberately also used for realtime subscriptions, which
// PocketBase filters with the very same list rule.
const OwnerRule = "@request.auth.collectionName = '" + CollectionUsers + "' && " + FieldOwner + " = @request.auth.id"
