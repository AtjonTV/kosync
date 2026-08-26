//
// File:        internal/kosyncapi/storage_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosyncapi_test

import (
	"net/http"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/config"
	"git.obth.eu/atjontv/kosync/internal/kosyncapi"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

const storageURL = "/api/kosync/storage"

// storageApp mounts the API with a quota of the given number of megabytes.
func storageApp(t testing.TB, megabytes int) *tests.TestApp {
	t.Helper()

	app := testutil.SeededApp(t)
	conf := &config.Config{BooksQuotaMegabytes: megabytes}
	conf.Normalize()
	kosyncapi.Register(app, conf)

	return app
}

// storeSizedBook writes a book of a stated size, without the upload path.
func storeSizedBook(t testing.TB, app core.App, id, owner string, size int64) {
	t.Helper()

	collection, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		t.Fatalf("find books collection: %v", err)
	}

	file, err := filesystem.NewFileFromBytes([]byte("PK\x03\x04 stand-in"), "book.epub")
	if err != nil {
		t.Fatalf("build file: %v", err)
	}

	record := core.NewRecord(collection)
	record.Id = id
	record.Set(schema.FieldOwner, owner)
	record.Set(schema.FieldFile, file)
	record.Set(schema.FieldTitle, "Zeit des Sturms")
	record.Set(schema.FieldContentHash, id)
	record.Set(schema.FieldFileSize, size)

	if err := app.Save(record); err != nil {
		t.Fatalf("save book: %v", err)
	}
}

func TestStorageReportsTheLibraryAndTheLimit(t *testing.T) {
	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:   "an account is told how much of its own room it has used",
		Method: http.MethodGet,
		URL:    storageURL,
		TestAppFactory: func(t testing.TB) *tests.TestApp {
			app := storageApp(t, 1)
			storeSizedBook(t, app, testutil.PadId("bookq"), testutil.IdUserA, 4096)
			// Another account's library must not appear in this answer.
			storeSizedBook(t, app, testutil.PadId("bookr"), testutil.IdUserB, 999_999)

			return app
		},
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"used":4096`, `"quota":1048576`, `"books":1`},
	})
}

// Nothing configured is not "zero bytes available", and the interface has to be
// able to tell the difference.
func TestStorageReportsNoLimitAsZero(t *testing.T) {
	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:   "no quota configured",
		Method: http.MethodGet,
		URL:    storageURL,
		TestAppFactory: func(t testing.TB) *tests.TestApp {
			return storageApp(t, 0)
		},
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"quota":0`},
	})
}

func TestStorageNeedsAnAccount(t *testing.T) {
	asUser(t, "", tests.ApiScenario{
		Name:            "a signed out request is refused",
		Method:          http.MethodGet,
		URL:             storageURL,
		ExpectedStatus:  http.StatusUnauthorized,
		ExpectedContent: []string{`"status":401`},
	})
}
