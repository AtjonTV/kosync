//
// File:        internal/books/books.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

// Package books turns an uploaded EPUB into a library record.
//
// Uploads go through the ordinary PocketBase collection API, so the rules,
// realtime events and file serving all come for free. What this package adds is
// a hook that reads the file as it arrives and fills in everything derived from
// it: the two hashes KOReader identifies the book by, the bibliographic
// metadata, the cover and the word count.
package books

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"path/filepath"
	"strings"
	"unicode"

	"git.obth.eu/atjontv/kosync/internal/config"
	"git.obth.eu/atjontv/kosync/internal/epub"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

// coverName is the stored name of the extracted cover. The extension is
// replaced with the one the archive used.
const coverName = "cover"

// Register wires the upload processing and the document matching into the app.
func Register(app core.App, conf *config.Config) {
	registerMatching(app)

	// Matching on arrival can fail, and nothing that arrives twice is guaranteed
	// to arrive a third time. This asks the question again every night.
	app.Cron().MustAdd(JobReconcile, scheduleReconcile, func() {
		linked, err := Reconcile(app)
		if err != nil {
			app.Logger().Error("failed to reconcile the unlinked documents", "error", err)

			return
		}
		if linked > 0 {
			app.Logger().Info("linked documents to books they had been missing", "documents", linked)
		}
	})

	app.OnRecordCreateRequest(schema.CollectionBooks).BindFunc(func(e *core.RecordRequestEvent) error {
		if err := describe(e.Record, conf); err != nil {
			return e.BadRequestError(sentence(err.Error()), nil)
		}

		if message, duplicate := alreadyHere(e.App, e.Record); duplicate {
			return e.BadRequestError(message, nil)
		}

		return e.Next()
	})

	// After the hook above, which is where the size of the incoming file is
	// learned: a quota cannot weigh a book nobody has measured yet.
	registerQuota(app, conf.QuotaBytes())

	// The catalog serves a book under a name derived from its title, so the hash
	// of that name has to be recomputed whenever the title changes. These are the
	// plain record hooks rather than the request ones so that a book created by
	// the importer or by a test gets the hash too.
	setCatalogHash := func(e *core.RecordEvent) error {
		e.Record.Set(schema.FieldHashCatalog, CatalogHash(e.Record))
		return e.Next()
	}
	app.OnRecordCreate(schema.CollectionBooks).BindFunc(setCatalogHash)
	app.OnRecordUpdate(schema.CollectionBooks).BindFunc(setCatalogHash)

	// The derived fields describe the file, so they cannot be edited away from
	// it. Title and authors are deliberately not in this list: correcting
	// publisher metadata is the owner's business.
	app.OnRecordUpdateRequest(schema.CollectionBooks).BindFunc(func(e *core.RecordRequestEvent) error {
		info, err := e.RequestInfo()
		if err != nil {
			return err
		}

		for _, field := range []string{
			schema.FieldFile,
			schema.FieldContentHash,
			schema.FieldHashBinary,
			schema.FieldHashFilename,
			schema.FieldWordCount,
			schema.FieldFileSize,
		} {
			if _, present := info.Body[field]; present {
				return e.BadRequestError(
					fmt.Sprintf("%q describes the uploaded file and cannot be changed; upload the book again instead.", field),
					nil,
				)
			}
		}

		// The catalog hash follows the title, and a hand-set value would claim
		// the book is served under a name it is not.
		if _, present := info.Body[schema.FieldHashCatalog]; present {
			return e.BadRequestError(
				fmt.Sprintf("%q follows the title and cannot be set by hand; rename the book instead.", schema.FieldHashCatalog),
				nil,
			)
		}

		// The measurement is taken from the progress the devices pushed. Letting
		// it be typed in would put a number nothing produced in front of every
		// page count the statistics are reckoned in.
		for _, field := range []string{
			schema.FieldMeasuredPages,
			schema.FieldMeasuredDevice,
			schema.FieldMeasuredThrough,
		} {
			if _, present := info.Body[field]; present {
				return e.BadRequestError(
					fmt.Sprintf("%q is measured from your reading and cannot be set by hand.", field),
					nil,
				)
			}
		}

		return e.Next()
	})
}

// alreadyHere reports whether the owner has uploaded this exact file before,
// with the sentence to say so.
//
// The unique index on (owner, content_hash) already refuses the second copy, but
// what it refuses it with is "Failed to create record." — true, unhelpful, and
// the same thing it would say about a dozen other problems. Somebody who has
// just dragged in a file wants to know which of the two it was, and that the
// answer is "nothing is wrong, you already have it".
//
// A lookup that fails is not treated as a duplicate. The index is still there
// behind this, so the worst case is the old message rather than a book let in
// twice.
func alreadyHere(app core.App, record *core.Record) (string, bool) {
	owner := record.GetString(schema.FieldOwner)
	hash := record.GetString(schema.FieldContentHash)
	if owner == "" || hash == "" {
		return "", false
	}

	existing, err := app.FindRecordsByFilter(
		schema.CollectionBooks,
		fmt.Sprintf("%s = {:owner} && %s = {:hash}", schema.FieldOwner, schema.FieldContentHash),
		"", 1, 0,
		dbx.Params{"owner": owner, "hash": hash},
	)
	if err != nil || len(existing) == 0 {
		return "", false
	}

	// The stored title rather than the incoming one: the same book can be
	// uploaded again under a different file name, and naming what is already
	// there is what lets somebody find it.
	title := strings.TrimSpace(existing[0].GetString(schema.FieldTitle))
	if title == "" {
		return "That book is already in your library.", true
	}

	return fmt.Sprintf("%q is already in your library.", title), true
}

// sentence turns an idiomatic Go error into something worth showing a person.
// Errors are lowercase and unpunctuated by convention; messages in a user
// interface are neither.
func sentence(message string) string {
	if message == "" {
		return message
	}

	runes := []rune(message)
	runes[0] = unicode.ToUpper(runes[0])
	result := string(runes)

	if !strings.HasSuffix(result, ".") {
		result += "."
	}

	return result
}

// describe reads the uploaded EPUB and fills in everything derived from it.
func describe(record *core.Record, conf *config.Config) error {
	uploads := record.GetUnsavedFiles(schema.FieldFile)
	if len(uploads) == 0 {
		// Either the file is missing, in which case the field's own validation
		// reports it, or this is a record being created server side.
		return nil
	}

	upload := uploads[0]

	reader, err := upload.Reader.Open()
	if err != nil {
		return fmt.Errorf("cannot read the uploaded file: %w", err)
	}
	defer reader.Close()

	binary, err := epub.PartialMD5(reader)
	if err != nil {
		return fmt.Errorf("cannot hash the uploaded file: %w", err)
	}

	content, err := contentHash(reader)
	if err != nil {
		return fmt.Errorf("cannot hash the uploaded file: %w", err)
	}

	book, err := epub.Open(readerAt{reader}, upload.Size)
	if err != nil {
		if errors.Is(err, epub.ErrNotEPUB) {
			return errors.New("that file is not an EPUB")
		}

		return fmt.Errorf("that EPUB could not be read: %w", err)
	}

	metadata := book.Metadata()
	words, err := book.WordCount()
	if err != nil {
		return fmt.Errorf("cannot read the EPUB: %w", err)
	}

	record.Set(schema.FieldContentHash, content)
	record.Set(schema.FieldFileSize, upload.Size)
	record.Set(schema.FieldHashBinary, binary)
	// The name the reader has on disk is the one it hashes, so this matches a
	// device that holds the very file that was uploaded. A copy served from the
	// catalog later will need the hash of the name it is served under.
	record.Set(schema.FieldHashFilename, epub.FilenameMD5(upload.OriginalName))
	record.Set(schema.FieldWordCount, words)
	record.Set(schema.FieldPageCount, notionalPages(words, conf.BooksWordsPerPage))

	// Publisher metadata only fills what the uploader has not set, so a title
	// corrected in the same request survives.
	setIfEmpty(record, schema.FieldTitle, fallbackTitle(metadata.Title, upload.OriginalName))
	setIfEmpty(record, schema.FieldLanguage, metadata.Language)
	setJSONIfEmpty(record, schema.FieldAuthors, metadata.Authors)
	setJSONIfEmpty(record, schema.FieldIdentifiers, metadata.Identifiers)

	// The series and its subjects are as correctable as the title is, and for
	// the same reason: publishers spell a series differently across its own
	// volumes, which is exactly the thing that would split one shelf into two.
	setIfEmpty(record, schema.FieldSeries, metadata.Series)
	setNumberIfZero(record, schema.FieldSeriesIndex, metadata.SeriesIndex)
	setJSONIfEmpty(record, schema.FieldSubjects, metadata.Subjects)

	if err := attachCover(record, book); err != nil {
		return err
	}

	return nil
}

// coverTypes are the image types a cover may be stored as, mapped to the
// extension the stored file gets. It must agree with the MimeTypes on the cover
// field, or the upload fails validation on the book's behalf.
var coverTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// attachCover extracts the cover image, if the book declares one.
//
// Every failure here is silent on purpose: a book with a missing, broken or
// exotic cover is still a book, and refusing the upload over its artwork would
// be absurd. The type is sniffed from the content rather than taken from the
// href, because an archive that names a PNG ".jpg" is common enough and would
// otherwise fail the field's own mime check and take the whole upload with it.
func attachCover(record *core.Record, book *epub.Reader) error {
	if len(record.GetUnsavedFiles(schema.FieldCover)) > 0 {
		return nil // the uploader supplied their own
	}

	_, data, err := book.Cover()
	if err != nil || len(data) == 0 {
		return nil //nolint:nilerr // deliberately non-fatal
	}

	extension, supported := coverTypes[detectImageType(data)]
	if !supported {
		return nil
	}

	file, err := filesystem.NewFileFromBytes(data, coverName+extension)
	if err != nil {
		return nil //nolint:nilerr // deliberately non-fatal
	}
	record.Set(schema.FieldCover, file)

	return nil
}

// detectImageType sniffs an image's type from its leading bytes.
func detectImageType(data []byte) string {
	detected := http.DetectContentType(data)
	if index := strings.IndexByte(detected, ';'); index >= 0 {
		detected = detected[:index]
	}

	return strings.TrimSpace(detected)
}

// contentHash is the SHA-256 of the whole file, used to recognise the same
// upload twice.
func contentHash(reader io.ReadSeeker) (string, error) {
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	digest := sha256.New()
	if _, err := io.Copy(digest, reader); err != nil {
		return "", err
	}

	return hex.EncodeToString(digest.Sum(nil)), nil
}

// notionalPages is the fallback page count: an EPUB has no pages of its own, so
// this is what the word count implies at the configured density. A measured
// count from the reader's own progress replaces it where one can be had.
func notionalPages(words, wordsPerPage int) int {
	if words <= 0 || wordsPerPage <= 0 {
		return 0
	}

	return int(math.Round(float64(words) / float64(wordsPerPage)))
}

// fallbackTitle uses the file name when the book carries no title of its own.
func fallbackTitle(title, filename string) string {
	if strings.TrimSpace(title) != "" {
		return title
	}

	base := filepath.Base(filename)

	return strings.TrimSuffix(base, filepath.Ext(base))
}

func setIfEmpty(record *core.Record, field, value string) {
	if value == "" {
		return
	}
	if strings.TrimSpace(record.GetString(field)) != "" {
		return
	}
	record.Set(field, value)
}

// setNumberIfZero fills a number the uploader left alone. Zero is the only
// value that can mean "not said" here — a series has no volume nought — so it
// is the one the file is allowed to overwrite.
func setNumberIfZero(record *core.Record, field string, value float64) {
	if value == 0 {
		return
	}
	if record.GetFloat(field) != 0 {
		return
	}
	record.Set(field, value)
}

// setJSONIfEmpty stores a value as JSON unless the uploader already supplied
// one.
func setJSONIfEmpty(record *core.Record, field string, value any) {
	if value == nil {
		return
	}
	if existing := strings.TrimSpace(record.GetString(field)); existing != "" && existing != "null" && existing != "[]" && existing != "{}" {
		return
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return
	}
	// An empty list is not worth writing down. A nil slice does not compare
	// equal to nil once it is inside an interface, so it arrives here and
	// encodes as "null" — which is the same nothing the column already holds,
	// spelled a second way for anything that later has to ask.
	switch string(encoded) {
	case "null", "[]", "{}":
		return
	}
	record.Set(field, string(encoded))
}

// readerAt adapts the uploaded file to the interface archive/zip needs.
//
// PocketBase hands over an io.ReadSeekCloser, and zip wants an io.ReaderAt.
// Seeking per read keeps the whole book out of memory, which matters when the
// field allows files far larger than the reference ones.
type readerAt struct {
	source io.ReadSeeker
}

func (r readerAt) ReadAt(buffer []byte, offset int64) (int, error) {
	if _, err := r.source.Seek(offset, io.SeekStart); err != nil {
		return 0, err
	}

	read, err := io.ReadFull(r.source, buffer)
	if errors.Is(err, io.ErrUnexpectedEOF) {
		// ReadAt reports a short final read as EOF, ReadFull does not.
		return read, io.EOF
	}

	return read, err
}
