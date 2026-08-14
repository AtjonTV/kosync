//
// File:        internal/koreader/koreader_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package koreader_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"git.obth.eu/atjontv/kosync/internal/config"
	"git.obth.eu/atjontv/kosync/internal/documents"
	"git.obth.eu/atjontv/kosync/internal/epub"
	"git.obth.eu/atjontv/kosync/internal/koreader"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// newApp returns a seeded app with the KOReader routes mounted.
func newApp(t testing.TB) *tests.TestApp {
	t.Helper()

	app := testutil.SeededApp(t)
	conf := &config.Config{KoreaderAuthCacheTtl: 300}
	conf.Normalize()
	koreader.Register(app, conf)

	return app
}

// deviceHeaders returns the two headers a KOReader device sends.
func deviceHeaders(username, password string) map[string]string {
	return map[string]string{
		koreader.HeaderAuthUser: username,
		koreader.HeaderAuthKey:  testutil.Md5Hex(password),
	}
}

func TestUsersAuth(t *testing.T) {
	scenarios := []tests.ApiScenario{
		{
			Name:            "valid credentials",
			Method:          http.MethodGet,
			URL:             "/koreader/users/auth",
			Headers:         deviceHeaders(testutil.KoUsernameA, testutil.KoPasswordA),
			ExpectedStatus:  http.StatusOK,
			ExpectedContent: []string{`"authorized":"OK"`},
		},
		{
			Name:            "wrong password",
			Method:          http.MethodGet,
			URL:             "/koreader/users/auth",
			Headers:         deviceHeaders(testutil.KoUsernameA, "not-the-password"),
			ExpectedStatus:  http.StatusUnauthorized,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:            "unknown user",
			Method:          http.MethodGet,
			URL:             "/koreader/users/auth",
			Headers:         deviceHeaders("nobody", testutil.KoPasswordA),
			ExpectedStatus:  http.StatusUnauthorized,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:            "missing headers",
			Method:          http.MethodGet,
			URL:             "/koreader/users/auth",
			ExpectedStatus:  http.StatusUnauthorized,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "the plain password is not accepted, only its digest",
			Method: http.MethodGet,
			URL:    "/koreader/users/auth",
			Headers: map[string]string{
				koreader.HeaderAuthUser: testutil.KoUsernameA,
				koreader.HeaderAuthKey:  testutil.KoPasswordA,
			},
			ExpectedStatus:  http.StatusUnauthorized,
			ExpectedContent: []string{`"data":{}`},
		},
	}

	for _, scenario := range scenarios {
		scenario.TestAppFactory = newApp
		scenario.Test(t)
	}
}

func TestPocketbaseTokensAreNotAcceptedByDeviceRoutes(t *testing.T) {
	headers := map[string]string{}

	scenario := tests.ApiScenario{
		Name:           "a WebUI session cannot push progress",
		Method:         http.MethodGet,
		URL:            "/koreader/users/auth",
		Headers:        headers,
		TestAppFactory: newApp,
		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			user, err := app.FindRecordById(schema.CollectionUsers, testutil.IdUserA)
			if err != nil {
				t.Fatalf("failed to load the fixture user: %v", err)
			}
			headers["Authorization"] = testutil.UserToken(t, user)
		},
		ExpectedStatus:  http.StatusUnauthorized,
		ExpectedContent: []string{`"data":{}`},
	}

	scenario.Test(t)
}

func TestDisabledCredentialsAreRejected(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:           "a disabled credential cannot authenticate",
		Method:         http.MethodGet,
		URL:            "/koreader/users/auth",
		Headers:        deviceHeaders(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory: newApp,
		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			account, err := app.FindFirstRecordByData(schema.CollectionKoreaderAccounts, schema.FieldUsername, testutil.KoUsernameA)
			if err != nil {
				t.Fatalf("failed to load the fixture credential: %v", err)
			}
			account.Set(schema.FieldDisabled, true)
			if err := app.Save(account); err != nil {
				t.Fatalf("failed to disable the fixture credential: %v", err)
			}
		},
		ExpectedStatus:  http.StatusUnauthorized,
		ExpectedContent: []string{`"data":{}`},
	}

	scenario.Test(t)
}

func TestUsersCreateIsRefused(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:            "registration points at the web interface",
		Method:          http.MethodPost,
		URL:             "/koreader/users/create",
		Body:            strings.NewReader(`{"username":"newcomer","password":"5f4dcc3b5aa765d61d8327deb882cf99"}`),
		TestAppFactory:  newApp,
		ExpectedStatus:  http.StatusPaymentRequired,
		ExpectedContent: []string{"web interface"},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			accounts, err := app.FindAllRecords(schema.CollectionKoreaderAccounts)
			if err != nil {
				t.Fatalf("failed to list the credentials: %v", err)
			}
			if len(accounts) != 2 {
				t.Errorf("expected the two seeded credentials to be untouched, got %d", len(accounts))
			}
		},
	}

	scenario.Test(t)
}

func TestGetProgress(t *testing.T) {
	scenarios := []tests.ApiScenario{
		{
			Name:           "returns the stored state",
			Method:         http.MethodGet,
			URL:            "/koreader/syncs/progress/" + testutil.DocumentHashA,
			Headers:        deviceHeaders(testutil.KoUsernameA, testutil.KoPasswordA),
			ExpectedStatus: http.StatusOK,
			ExpectedContent: []string{
				`"document":"` + testutil.DocumentHashA + `"`,
				`"percentage":0.25`,
				// 2026-03-01T12:00:00Z in Unix seconds, not the legacy 1/10000s unit.
				`"timestamp":1772366400`,
			},
		},
		{
			Name:            "unknown document",
			Method:          http.MethodGet,
			URL:             "/koreader/syncs/progress/does-not-exist",
			Headers:         deviceHeaders(testutil.KoUsernameA, testutil.KoPasswordA),
			ExpectedStatus:  http.StatusNotFound,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:            "a device cannot read another user's document",
			Method:          http.MethodGet,
			URL:             "/koreader/syncs/progress/" + testutil.DocumentHashA,
			Headers:         deviceHeaders(testutil.KoUsernameB, testutil.KoPasswordB),
			ExpectedStatus:  http.StatusNotFound,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:            "unauthenticated",
			Method:          http.MethodGet,
			URL:             "/koreader/syncs/progress/" + testutil.DocumentHashA,
			ExpectedStatus:  http.StatusUnauthorized,
			ExpectedContent: []string{`"data":{}`},
		},
	}

	for _, scenario := range scenarios {
		scenario.TestAppFactory = newApp
		scenario.Test(t)
	}
}

func TestPutProgressCreatesADocument(t *testing.T) {
	const newHash = "9f9f9f9f9f9f9f9f9f9f9f9f9f9f9f9f"

	scenario := tests.ApiScenario{
		Name:   "a first push creates the document without history",
		Method: http.MethodPut,
		URL:    "/koreader/syncs/progress",
		Body: strings.NewReader(`{"document":"` + newHash + `","progress":"/body/DocFragment[3]",` +
			`"percentage":0.42,"device":"Kobo Clara","device_id":"ABCDEF"}`),
		Headers:         deviceHeaders(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:  newApp,
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"document":"` + newHash + `"`, `"timestamp":`},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			document, err := app.FindFirstRecordByFilter(
				schema.CollectionDocuments,
				"owner = {:owner} && document = {:document}",
				dbx.Params{"owner": testutil.IdUserA, "document": newHash},
			)
			if err != nil {
				t.Fatalf("expected the pushed document to be stored: %v", err)
			}

			if got := document.GetFloat(schema.FieldProgress); got != 0.42 {
				t.Errorf("expected progress 0.42, got %v", got)
			}
			if got := document.GetString(schema.FieldCurrentLocation); got != "/body/DocFragment[3]" {
				t.Errorf("unexpected current_location %q", got)
			}
			if got := document.GetString(schema.FieldLastDevice); got != "Kobo Clara" {
				t.Errorf("unexpected last_device %q", got)
			}
			if got := document.GetString(schema.FieldSourceAccount); got == "" {
				t.Errorf("expected the pushing credential to be recorded")
			}

			history, err := app.FindAllRecords(schema.CollectionDocumentHistory, dbx.HashExp{"document_ref": document.Id})
			if err != nil {
				t.Fatalf("failed to load the document history: %v", err)
			}
			if len(history) != 0 {
				t.Errorf("expected a first push to leave no history, got %d entries", len(history))
			}
		},
	}

	scenario.Test(t)
}

func TestPutProgressArchivesThePreviousState(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:   "a second push archives what it replaced",
		Method: http.MethodPut,
		URL:    "/koreader/syncs/progress",
		Body: strings.NewReader(`{"document":"` + testutil.DocumentHashA + `","progress":"/body/DocFragment[9]",` +
			`"percentage":0.75,"device":"Kobo Clara","device_id":"ABCDEF"}`),
		Headers:         deviceHeaders(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:  newApp,
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"document":"` + testutil.DocumentHashA + `"`},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			document, err := app.FindRecordById(schema.CollectionDocuments, testutil.IdDocumentA)
			if err != nil {
				t.Fatalf("failed to reload the document: %v", err)
			}
			if got := document.GetFloat(schema.FieldProgress); got != 0.75 {
				t.Errorf("expected the document to hold the new progress, got %v", got)
			}

			history, err := app.FindAllRecords(schema.CollectionDocumentHistory, dbx.HashExp{"document_ref": document.Id})
			if err != nil {
				t.Fatalf("failed to load the document history: %v", err)
			}
			// One seeded entry plus the state this push replaced.
			if len(history) != 2 {
				t.Fatalf("expected 2 history entries, got %d", len(history))
			}

			var archived bool
			for _, entry := range history {
				if entry.GetFloat(schema.FieldProgress) == 0.25 {
					archived = true
				}
				if entry.GetFloat(schema.FieldProgress) == 0.75 {
					t.Errorf("the new state must not be archived, only the state it replaced")
				}
			}
			if !archived {
				t.Errorf("expected the replaced progress of 0.25 to be archived")
			}
		},
	}

	scenario.Test(t)
}

func TestPutProgressIsScopedToTheOwner(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:   "the same document hash for another user is a separate document",
		Method: http.MethodPut,
		URL:    "/koreader/syncs/progress",
		Body: strings.NewReader(`{"document":"` + testutil.DocumentHashA + `","progress":"/body/DocFragment[1]",` +
			`"percentage":0.9,"device":"Kindle","device_id":"FEDCBA"}`),
		Headers:         deviceHeaders(testutil.KoUsernameB, testutil.KoPasswordB),
		TestAppFactory:  newApp,
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"document":"` + testutil.DocumentHashA + `"`},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			untouched, err := app.FindRecordById(schema.CollectionDocuments, testutil.IdDocumentA)
			if err != nil {
				t.Fatalf("failed to reload user A's document: %v", err)
			}
			if got := untouched.GetFloat(schema.FieldProgress); got != 0.25 {
				t.Errorf("user B's push changed user A's document, progress is now %v", got)
			}

			pushed, err := app.FindFirstRecordByFilter(
				schema.CollectionDocuments,
				"owner = {:owner} && document = {:document}",
				dbx.Params{"owner": testutil.IdUserB, "document": testutil.DocumentHashA},
			)
			if err != nil {
				t.Fatalf("expected user B to get their own document record: %v", err)
			}
			if got := pushed.GetFloat(schema.FieldProgress); got != 0.9 {
				t.Errorf("expected user B's progress 0.9, got %v", got)
			}
		},
	}

	scenario.Test(t)
}

func TestPutProgressRejectsAnEmptyDocument(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:            "the document hash is required",
		Method:          http.MethodPut,
		URL:             "/koreader/syncs/progress",
		Body:            strings.NewReader(`{"progress":"/body","percentage":0.1,"device":"Kobo"}`),
		Headers:         deviceHeaders(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:  newApp,
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{`"data":{}`},
	}

	scenario.Test(t)
}

func TestPutProgressClampsAnOutOfRangePercentage(t *testing.T) {
	const newHash = "cccccccccccccccccccccccccccccccc"

	scenario := tests.ApiScenario{
		Name:            "a percentage above one is clamped instead of refused",
		Method:          http.MethodPut,
		URL:             "/koreader/syncs/progress",
		Body:            strings.NewReader(`{"document":"` + newHash + `","progress":"/body","percentage":1.0000001,"device":"Kobo"}`),
		Headers:         deviceHeaders(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:  newApp,
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"document":"` + newHash + `"`},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			document, err := app.FindFirstRecordByFilter(
				schema.CollectionDocuments,
				"owner = {:owner} && document = {:document}",
				dbx.Params{"owner": testutil.IdUserA, "document": newHash},
			)
			if err != nil {
				t.Fatalf("expected the pushed document to be stored: %v", err)
			}
			if got := document.GetFloat(schema.FieldProgress); got != 1 {
				t.Errorf("expected the progress to be clamped to 1, got %v", got)
			}
		},
	}

	scenario.Test(t)
}

func TestPushUpdatesTheCredentialLastUsed(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:            "authenticating records that the device was seen",
		Method:          http.MethodGet,
		URL:             "/koreader/users/auth",
		Headers:         deviceHeaders(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:  newApp,
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"authorized":"OK"`},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			account, err := app.FindFirstRecordByData(schema.CollectionKoreaderAccounts, schema.FieldUsername, testutil.KoUsernameA)
			if err != nil {
				t.Fatalf("failed to reload the credential: %v", err)
			}
			if account.GetDateTime(schema.FieldLastUsed).IsZero() {
				t.Errorf("expected last_used to be set after a successful authentication")
			}
		},
	}

	scenario.Test(t)
}

// mergedAwayHash is the hash of a document that was folded into another one, the
// way a second copy of the same book ends up after a merge.
const mergedAwayHash = "7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e"

// withMergedDocument seeds a second document of user A and merges it into the
// fixture's document, leaving mergedAwayHash as an alias.
func withMergedDocument(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
	user, err := app.FindRecordById(schema.CollectionUsers, testutil.IdUserA)
	if err != nil {
		t.Fatalf("failed to load the fixture user: %v", err)
	}

	second := testutil.CreateDocument(t, app, user, "", mergedAwayHash, 0.05,
		time.Date(2026, 2, 20, 8, 0, 0, 0, time.UTC))

	if _, err := documents.Merge(app, user.Id, testutil.IdDocumentA, []string{second.Id}); err != nil {
		t.Fatalf("failed to merge the fixture documents: %v", err)
	}
}

// After a merge the device that reported the folded hash carries on sending it.
// If that made a document of its own again the merge would come apart on the
// next sync, so the push has to land on the document it was merged into.
func TestPutProgressFollowsAMerge(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:   "a push under a merged away hash updates the surviving document",
		Method: http.MethodPut,
		URL:    "/koreader/syncs/progress",
		Body: strings.NewReader(`{"document":"` + mergedAwayHash + `","progress":"/body/DocFragment[9]",` +
			`"percentage":0.8,"device":"Kobo Clara","device_id":"ABCDEF"}`),
		Headers:         deviceHeaders(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:  newApp,
		BeforeTestFunc:  withMergedDocument,
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"document":"` + mergedAwayHash + `"`},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			survivor, err := app.FindRecordById(schema.CollectionDocuments, testutil.IdDocumentA)
			if err != nil {
				t.Fatalf("failed to reload the surviving document: %v", err)
			}
			if got := survivor.GetFloat(schema.FieldProgress); got != 0.8 {
				t.Errorf("expected the push to land on the survivor at 0.8, got %v", got)
			}

			all, err := app.FindAllRecords(schema.CollectionDocuments,
				dbx.HashExp{schema.FieldOwner: testutil.IdUserA})
			if err != nil {
				t.Fatalf("failed to list the documents: %v", err)
			}
			if len(all) != 1 {
				t.Errorf("expected the merge to hold, user A now has %d documents", len(all))
			}
		},
	}

	scenario.Test(t)
}

// The other half of the same thing: the device asks about its own file and gets
// the joined position back, which is what makes two devices sync with each other
// once their documents have been merged.
func TestGetProgressFollowsAMerge(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:           "a pull under a merged away hash reads the surviving document",
		Method:         http.MethodGet,
		URL:            "/koreader/syncs/progress/" + mergedAwayHash,
		Headers:        deviceHeaders(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory: newApp,
		BeforeTestFunc: withMergedDocument,
		ExpectedStatus: http.StatusOK,
		ExpectedContent: []string{
			// Answered about the file it asked about, at the position the reading
			// as a whole has reached.
			`"document":"` + mergedAwayHash + `"`,
			`"percentage":0.25`,
		},
	}

	scenario.Test(t)
}

// pushWithMetadata is a progress push from a device that has "send document
// metadata" turned on.
func pushWithMetadata(hash, filename, title, authors string) string {
	return `{"document":"` + hash + `","progress":"/body/DocFragment[1]","percentage":0.3,` +
		`"device":"Kobo Clara","device_id":"ABCDEF","metadata":{` +
		`"filename":"` + filename + `","title":"` + title + `","authors":"` + authors + `"}}`
}

func TestPutProgressRecordsTheDocumentMetadata(t *testing.T) {
	const newHash = "5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c"

	scenario := tests.ApiScenario{
		Name:            "a push with metadata names the document",
		Method:          http.MethodPut,
		URL:             "/koreader/syncs/progress",
		Body:            strings.NewReader(pushWithMetadata(newHash, "Metro 2033.epub", "Metro 2033", "Dmitry Glukhovsky")),
		Headers:         deviceHeaders(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:  newApp,
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"document":"` + newHash + `"`},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			document, err := app.FindFirstRecordByFilter(schema.CollectionDocuments,
				"document = {:hash}", dbx.Params{"hash": newHash})
			if err != nil {
				t.Fatalf("failed to load the pushed document: %v", err)
			}

			if got := document.GetString(schema.FieldTitle); got != "Metro 2033" {
				t.Errorf("expected the reported title, got %q", got)
			}
			if got := document.GetString(schema.FieldFilename); got != "Metro 2033.epub" {
				t.Errorf("expected the reported filename, got %q", got)
			}
			if got := document.GetString(schema.FieldDocumentAuthors); got != "Dmitry Glukhovsky" {
				t.Errorf("expected the reported authors, got %q", got)
			}
			// The hash of the name, so a book stored under it can be found by an
			// indexed comparison.
			if got := document.GetString(schema.FieldFilenameHash); got != epub.FilenameMD5("Metro 2033.epub") {
				t.Errorf("expected the filename hash, got %q", got)
			}
		},
	}

	scenario.Test(t)
}

// A push without the setting turned on is the ordinary case and must stay
// exactly as it was.
func TestPutProgressWithoutMetadataStoresNothingExtra(t *testing.T) {
	const newHash = "6d6d6d6d6d6d6d6d6d6d6d6d6d6d6d6d"

	scenario := tests.ApiScenario{
		Name:   "a push with no metadata is unchanged",
		Method: http.MethodPut,
		URL:    "/koreader/syncs/progress",
		Body: strings.NewReader(`{"document":"` + newHash + `","progress":"/body","percentage":0.1,` +
			`"device":"Kobo","device_id":"ABC"}`),
		Headers:         deviceHeaders(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:  newApp,
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"document":"` + newHash + `"`},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			document, err := app.FindFirstRecordByFilter(schema.CollectionDocuments,
				"document = {:hash}", dbx.Params{"hash": newHash})
			if err != nil {
				t.Fatalf("failed to load the pushed document: %v", err)
			}
			for _, field := range []string{schema.FieldFilename, schema.FieldFilenameHash, schema.FieldDocumentAuthors} {
				if got := document.GetString(field); got != "" {
					t.Errorf("expected %q to stay empty, got %q", field, got)
				}
			}
		},
	}

	scenario.Test(t)
}

// The title is the one thing on a document a person can edit, so a device that
// keeps sending the publisher's title must not undo a rename.
func TestPutProgressDoesNotOverwriteAChosenTitle(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:           "a reported title does not replace one that was set",
		Method:         http.MethodPut,
		URL:            "/koreader/syncs/progress",
		Body:           strings.NewReader(pushWithMetadata(testutil.DocumentHashA, "whatever.epub", "The Publisher's Title", "")),
		Headers:        deviceHeaders(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory: newApp,
		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
			document, err := app.FindRecordById(schema.CollectionDocuments, testutil.IdDocumentA)
			if err != nil {
				t.Fatalf("failed to load the fixture document: %v", err)
			}
			document.Set(schema.FieldTitle, "What I Called It")
			if err := app.Save(document); err != nil {
				t.Fatalf("failed to name the document: %v", err)
			}
		},
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"document":"` + testutil.DocumentHashA + `"`},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			document, err := app.FindRecordById(schema.CollectionDocuments, testutil.IdDocumentA)
			if err != nil {
				t.Fatalf("failed to reload the document: %v", err)
			}
			if got := document.GetString(schema.FieldTitle); got != "What I Called It" {
				t.Errorf("the chosen title was overwritten, it is now %q", got)
			}
			// The filename is not editable, so it is kept up to date.
			if got := document.GetString(schema.FieldFilename); got != "whatever.epub" {
				t.Errorf("expected the filename to be recorded, got %q", got)
			}
		},
	}

	scenario.Test(t)
}
