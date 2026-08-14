//
// File:        internal/migrations/1786665600_books.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package migrations

import (
	"fmt"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

// MaxBookBytes caps a single uploaded EPUB. Illustrated editions run well past
// the PocketBase default of 5 MiB — the reference books are 2 to 7 MiB — while
// this still keeps a single upload bounded.
const MaxBookBytes int64 = 128 << 20

func init() {
	m.Register(upBooks, downBooks)
}

func upBooks(app core.App) error {
	users, err := app.FindCollectionByNameOrId(schema.CollectionUsers)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionUsers, err)
	}

	return createBooks(app, users.Id)
}

func downBooks(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		return nil
	}

	return app.Delete(collection)
}

// createBooks creates the library collection.
//
// Everything below the file itself is derived from it on upload, not supplied
// by the client: see internal/books.
func createBooks(app core.App, usersId string) error {
	collection := core.NewBaseCollection(schema.CollectionBooks)

	collection.ListRule = types.Pointer(schema.OwnerRule)
	collection.ViewRule = types.Pointer(schema.OwnerRule)
	collection.CreateRule = types.Pointer(schema.OwnerRule)
	collection.UpdateRule = types.Pointer(schema.OwnerRule)
	collection.DeleteRule = types.Pointer(schema.OwnerRule)

	collection.Fields.Add(&core.RelationField{
		Name:          schema.FieldOwner,
		Required:      true,
		MaxSelect:     1,
		CollectionId:  usersId,
		CascadeDelete: true,
	})
	collection.Fields.Add(&core.FileField{
		Name:      schema.FieldFile,
		Required:  true,
		MaxSelect: 1,
		MaxSize:   MaxBookBytes,
		// Readers and OPDS clients send the generic types too, so accepting
		// only application/epub+zip rejects perfectly good uploads.
		MimeTypes: []string{
			"application/epub+zip",
			"application/zip",
			"application/octet-stream",
		},
	})
	collection.Fields.Add(&core.FileField{
		Name:      schema.FieldCover,
		MaxSelect: 1,
		MaxSize:   16 << 20,
		MimeTypes: []string{"image/jpeg", "image/png", "image/gif", "image/webp"},
		Thumbs:    []string{"100x150", "200x300"},
	})
	collection.Fields.Add(&core.TextField{
		Name: schema.FieldTitle,
		Max:  500,
	})
	collection.Fields.Add(&core.JSONField{
		Name:    schema.FieldAuthors,
		MaxSize: 8000,
	})
	collection.Fields.Add(&core.TextField{
		Name: schema.FieldLanguage,
		Max:  35,
	})
	collection.Fields.Add(&core.JSONField{
		Name:    schema.FieldIdentifiers,
		MaxSize: 8000,
	})
	collection.Fields.Add(&core.NumberField{
		Name:    schema.FieldPageCount,
		OnlyInt: true,
		Min:     types.Pointer(0.0),
	})
	collection.Fields.Add(&core.NumberField{
		Name:    schema.FieldWordCount,
		OnlyInt: true,
		Min:     types.Pointer(0.0),
	})
	collection.Fields.Add(&core.TextField{
		Name: schema.FieldContentHash,
		Max:  64,
	})
	collection.Fields.Add(&core.TextField{
		Name: schema.FieldHashBinary,
		Max:  32,
	})
	collection.Fields.Add(&core.TextField{
		Name: schema.FieldHashFilename,
		Max:  32,
	})
	addTimestamps(collection)

	// The same file uploaded twice by one person is the same book. Across
	// owners it deliberately is not: shared storage would make deletion and
	// ownership ambiguous for a saving that does not matter at this scale.
	collection.AddIndex("idx_books_owner_content_hash", true, "owner,content_hash", "")

	// Phase 9 matches an incoming progress push by looking a book up on either
	// hash, so both are indexed per owner.
	collection.AddIndex("idx_books_owner_hash_binary", false, "owner,hash_binary", "")
	collection.AddIndex("idx_books_owner_hash_filename", false, "owner,hash_filename", "")

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("create %q: %w", schema.CollectionBooks, err)
	}

	return nil
}
