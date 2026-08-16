//
// File:        internal/books/quota_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package books_test

import (
	"net/http"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/books"
	"git.obth.eu/atjontv/kosync/internal/config"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// megabyte is the unit the quota is configured in.
const megabyte = 1024 * 1024

// quotaApp returns a seeded app whose accounts are limited to the given number
// of megabytes.
func quotaApp(t testing.TB, megabytes int) *tests.TestApp {
	t.Helper()

	app := testutil.SeededApp(t)
	conf := &config.Config{BooksQuotaMegabytes: megabytes}
	conf.Normalize()
	books.Register(app, conf)

	return app
}

// fill stores a book of the given size without going through the upload path,
// which is how an account that is already nearly full is described without
// having to produce a megabyte of EPUB to get there.
func fill(t testing.TB, app core.App, owner string, size int64) {
	t.Helper()

	book := createBook(t, app, testutil.PadId("filler"), owner,
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	book.Set(schema.FieldFileSize, size)

	if err := app.Save(book); err != nil {
		t.Fatalf("store the filler book: %v", err)
	}
}

func TestUploadRecordsHowMuchRoomItTakes(t *testing.T) {
	content := epubBytes(t, 200)
	body, contentType := upload(t, testutil.IdUserA, "measured.epub", content, nil)

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "an upload is weighed as it arrives",
		Method:          http.MethodPost,
		URL:             booksURL,
		Body:            body,
		Headers:         map[string]string{"Content-Type": contentType},
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"file_size":`},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			book, err := app.FindFirstRecordByData(schema.CollectionBooks, schema.FieldOwner, testutil.IdUserA)
			if err != nil {
				t.Fatalf("expected the book to be stored: %v", err)
			}
			if got := book.GetInt(schema.FieldFileSize); got != len(content) {
				t.Errorf("stored size is %d, want %d", got, len(content))
			}
		},
	})
}

func TestUploadIsRefusedWhenThereIsNoRoomLeft(t *testing.T) {
	body, contentType := upload(t, testutil.IdUserA, "one-too-many.epub", epubBytes(t, 200), nil)

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:   "a library that is already full takes nothing more",
		Method: http.MethodPost,
		URL:    booksURL,
		Body:   body,
		TestAppFactory: func(t testing.TB) *tests.TestApp {
			app := quotaApp(t, 1)
			fill(t, app, testutil.IdUserA, megabyte-10)

			return app
		},
		Headers:        map[string]string{"Content-Type": contentType},
		ExpectedStatus: http.StatusBadRequest,
		// The refusal says what is needed and what is free, because "quota
		// exceeded" leaves the reader with nothing to do about it.
		ExpectedContent:    []string{"not enough room", "free"},
		NotExpectedContent: []string{`"title"`},
	})
}

func TestUploadIsAcceptedWhileThereIsRoom(t *testing.T) {
	body, contentType := upload(t, testutil.IdUserA, "plenty-of-room.epub", epubBytes(t, 200), nil)

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:   "a small book into an empty library",
		Method: http.MethodPost,
		URL:    booksURL,
		Body:   body,
		TestAppFactory: func(t testing.TB) *tests.TestApp {
			return quotaApp(t, 1)
		},
		Headers:         map[string]string{"Content-Type": contentType},
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"title"`},
	})
}

// The default. An instance nobody set a limit on does not have one, and the
// hook that would enforce it is never bound.
func TestWithoutAQuotaNothingIsRefused(t *testing.T) {
	body, contentType := upload(t, testutil.IdUserA, "unlimited.epub", epubBytes(t, 200), nil)

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:   "no limit configured",
		Method: http.MethodPost,
		URL:    booksURL,
		Body:   body,
		TestAppFactory: func(t testing.TB) *tests.TestApp {
			app := quotaApp(t, 0)
			fill(t, app, testutil.IdUserA, 500*megabyte)

			return app
		},
		Headers:         map[string]string{"Content-Type": contentType},
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"title"`},
	})
}

func TestUsageIsCountedPerAccount(t *testing.T) {
	app := quotaApp(t, 1)
	fill(t, app, testutil.IdUserA, 4096)

	usage, err := books.UsageOf(app, megabyte, testutil.IdUserA)
	if err != nil {
		t.Fatalf("measure: %v", err)
	}
	if usage.Used != 4096 {
		t.Errorf("used is %d, want 4096", usage.Used)
	}
	if usage.Free() != megabyte-4096 {
		t.Errorf("free is %d, want %d", usage.Free(), megabyte-4096)
	}

	// The other account's library is its own, and the seeded books it may have
	// were never weighed.
	other, err := books.UsageOf(app, megabyte, testutil.IdUserB)
	if err != nil {
		t.Fatalf("measure the other account: %v", err)
	}
	if other.Used != 0 {
		t.Errorf("the other account is charged %d bytes", other.Used)
	}
}

// Without a quota there is nothing to be free of, and -1 says so rather than
// pretending an empty library has room for a specific number of bytes.
func TestUnlimitedHasNoFreeSpace(t *testing.T) {
	usage := books.Usage{Used: 12345, Quota: 0}
	if usage.Free() != -1 {
		t.Errorf("free is %d, want -1", usage.Free())
	}
}

func TestSizesAreWrittenTheWayPeopleReadThem(t *testing.T) {
	cases := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{20 * 1024, "20 KB"},
		{megabyte, "1.0 MB"},
		{5 * megabyte, "5.0 MB"},
		{700 * megabyte, "700 MB"},
		{2 * 1024 * megabyte, "2.0 GB"},
		{-1, "0 B"},
	}

	for _, one := range cases {
		if got := books.FormatBytes(one.bytes); got != one.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", one.bytes, got, one.want)
		}
	}
}
