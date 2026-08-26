//
// File:        internal/migrations/1788307200_protect_book_files_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package migrations_test

import (
	"net/http"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestBookFilesAreProtected(t *testing.T) {
	app := testutil.NewApp(t)

	collection, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		t.Fatalf("failed to find the %q collection: %v", schema.CollectionBooks, err)
	}

	for _, name := range []string{schema.FieldFile, schema.FieldCover} {
		field, ok := collection.Fields.GetByName(name).(*core.FileField)
		if !ok {
			t.Errorf("expected %q to be a file field", name)
			continue
		}
		if !field.Protected {
			t.Errorf("expected %q to be protected", name)
		}
	}
}

func TestTheFileTokenOutlastsPocketbasesDefault(t *testing.T) {
	app := testutil.NewApp(t)

	users, err := app.FindCollectionByNameOrId(schema.CollectionUsers)
	if err != nil {
		t.Fatalf("failed to find the %q collection: %v", schema.CollectionUsers, err)
	}

	// The default is three minutes, which would re-fetch a page full of covers
	// every three minutes. See the migration for the reasoning.
	if users.FileToken.Duration <= 180 {
		t.Errorf("the file token lasts %d seconds, which is PocketBase's own default or less",
			users.FileToken.Duration)
	}
}

// The two fields go through the same PocketBase code path, so the file stands
// in for the cover here: what is being tested is that the collection's view
// rule now reaches the stored file at all.
func TestAStoredBookNeedsATokenToBeRead(t *testing.T) {
	noToken := func(testing.TB, *tests.TestApp) string { return "" }
	madeUp := func(testing.TB, *tests.TestApp) string { return "not-a-token" }

	download(t, tests.ApiScenario{
		Name:            "a guest cannot download a stored book",
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{`"data":{}`},
	}, noToken)

	download(t, tests.ApiScenario{
		Name:            "a made up token cannot download a stored book",
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{`"data":{}`},
	}, madeUp)

	download(t, tests.ApiScenario{
		Name:            "another account cannot download a stored book",
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{`"data":{}`},
	}, tokenOf(testutil.IdUserB))

	download(t, tests.ApiScenario{
		Name:            "the owner downloads their own book",
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{"stand-in"},
	}, tokenOf(testutil.IdUserA))
}

// download runs one request for the stored file of a book owned by user A.
//
// The address is filled in from inside the test rather than written into the
// scenario, because neither half of it can be known before the app exists: the
// stored name of a file carries a random suffix, and a file token is signed
// with a secret that is generated per instance. PocketBase builds the request
// after the before-func has run, which is what makes this work.
func download(t *testing.T, scenario tests.ApiScenario, token func(testing.TB, *tests.TestApp) string) {
	t.Helper()

	scenario.Method = http.MethodGet
	scenario.TestAppFactory = testutil.SeededApp
	scenario.BeforeTestFunc = func(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
		alice, err := app.FindRecordById(schema.CollectionUsers, testutil.IdUserA)
		if err != nil {
			t.Fatalf("failed to load the fixture user: %v", err)
		}

		book := testutil.CreateBook(t, app, alice, testutil.PadId("bookf"), "Zeit des Sturms", "aa", "bb")

		scenario.URL = "/api/files/" + schema.CollectionBooks + "/" + book.Id + "/" +
			book.GetString(schema.FieldFile)
		if value := token(t, app); value != "" {
			scenario.URL += "?token=" + value
		}
	}

	scenario.Test(t)
}

// tokenOf mints the short lived token that opens a protected file, for one of
// the fixture accounts.
func tokenOf(userId string) func(testing.TB, *tests.TestApp) string {
	return func(t testing.TB, app *tests.TestApp) string {
		user, err := app.FindRecordById(schema.CollectionUsers, userId)
		if err != nil {
			t.Fatalf("failed to load the fixture user %q: %v", userId, err)
		}

		token, err := user.NewFileToken()
		if err != nil {
			t.Fatalf("failed to mint a file token: %v", err)
		}

		return token
	}
}
