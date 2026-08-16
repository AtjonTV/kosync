//
// File:        internal/migrations/rules_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package migrations_test

import (
	"net/http"
	"strings"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// asUser runs the scenario against a freshly seeded app, authenticated as the
// given fixture user. An empty userId runs the scenario as a guest.
//
// The token has to be minted after the app exists (its signing key is per
// instance), so it is injected from BeforeTestFunc into the header map that the
// scenario later reads.
func asUser(t *testing.T, userId string, scenario tests.ApiScenario) {
	t.Helper()

	headers := map[string]string{}
	for k, v := range scenario.Headers {
		headers[k] = v
	}
	scenario.Headers = headers

	if scenario.TestAppFactory == nil {
		scenario.TestAppFactory = testutil.SeededApp
	}

	if userId != "" {
		scenario.BeforeTestFunc = func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			user, err := app.FindRecordById(schema.CollectionUsers, userId)
			if err != nil {
				t.Fatalf("failed to load the fixture user %q: %v", userId, err)
			}
			headers["Authorization"] = testutil.UserToken(t, user)
		}
	}

	scenario.Test(t)
}

func TestDocumentsAreScopedToTheirOwner(t *testing.T) {
	asUser(t, "", tests.ApiScenario{
		Name:            "a guest lists no documents",
		Method:          http.MethodGet,
		URL:             "/api/collections/documents/records",
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"totalItems":0`},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "a user lists only their own documents",
		Method:          http.MethodGet,
		URL:             "/api/collections/documents/records",
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"totalItems":1`, `"id":"` + testutil.IdDocumentA + `"`},
		NotExpectedContent: []string{
			`"id":"` + testutil.IdDocumentB + `"`,
		},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "a user cannot view a foreign document",
		Method:          http.MethodGet,
		URL:             "/api/collections/documents/records/" + testutil.IdDocumentB,
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{`"data":{}`},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "a user cannot rename a foreign document",
		Method:          http.MethodPatch,
		URL:             "/api/collections/documents/records/" + testutil.IdDocumentB,
		Body:            strings.NewReader(`{"title":"stolen"}`),
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{`"data":{}`},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "a user cannot delete a foreign document",
		Method:          http.MethodDelete,
		URL:             "/api/collections/documents/records/" + testutil.IdDocumentB,
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{`"data":{}`},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "a user can rename their own document",
		Method:          http.MethodPatch,
		URL:             "/api/collections/documents/records/" + testutil.IdDocumentA,
		Body:            strings.NewReader(`{"title":"The Hitchhiker's Guide"}`),
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"title":"The Hitchhiker's Guide"`},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "documents cannot be created through the collection API",
		Method:          http.MethodPost,
		URL:             "/api/collections/documents/records",
		Body:            strings.NewReader(`{"owner":"` + testutil.IdUserA + `","document":"deadbeef","last_read_at":"2026-03-01 12:00:00.000Z"}`),
		ExpectedStatus:  http.StatusForbidden,
		ExpectedContent: []string{`"data":{}`},
	})
}

func TestDocumentHistoryIsScopedToItsOwner(t *testing.T) {
	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "a user lists only their own history",
		Method:          http.MethodGet,
		URL:             "/api/collections/document_history/records",
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"totalItems":1`, `"id":"` + testutil.IdHistoryA + `"`},
	})

	asUser(t, testutil.IdUserB, tests.ApiScenario{
		Name:            "a user cannot see a foreign history entry",
		Method:          http.MethodGet,
		URL:             "/api/collections/document_history/records/" + testutil.IdHistoryA,
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{`"data":{}`},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "history cannot be forged through the collection API",
		Method:          http.MethodPost,
		URL:             "/api/collections/document_history/records",
		Body:            strings.NewReader(`{"owner":"` + testutil.IdUserA + `","document_ref":"` + testutil.IdDocumentA + `","progress":1,"last_read_at":"2026-03-01 12:00:00.000Z"}`),
		ExpectedStatus:  http.StatusForbidden,
		ExpectedContent: []string{`"data":{}`},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "history cannot be rewritten",
		Method:          http.MethodPatch,
		URL:             "/api/collections/document_history/records/" + testutil.IdHistoryA,
		Body:            strings.NewReader(`{"progress":0.99}`),
		ExpectedStatus:  http.StatusForbidden,
		ExpectedContent: []string{`"data":{}`},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:           "a user can delete their own history entry",
		Method:         http.MethodDelete,
		URL:            "/api/collections/document_history/records/" + testutil.IdHistoryA,
		ExpectedStatus: http.StatusNoContent,
	})
}

func TestKoreaderAccountsAreScopedAndCannotAuthenticate(t *testing.T) {
	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "a user lists only their own credentials",
		Method:          http.MethodGet,
		URL:             "/api/collections/koreader_accounts/records",
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"totalItems":1`, `"username":"` + testutil.KoUsernameA + `"`},
		NotExpectedContent: []string{
			testutil.KoUsernameB,
			`"password"`, // the digest must never leave the server
		},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "credentials cannot be created through the collection API",
		Method:          http.MethodPost,
		URL:             "/api/collections/koreader_accounts/records",
		Body:            strings.NewReader(`{"username":"sneaky","owner":"` + testutil.IdUserA + `","password":"0123456789abcdef","passwordConfirm":"0123456789abcdef"}`),
		ExpectedStatus:  http.StatusForbidden,
		ExpectedContent: []string{`"data":{}`},
	})

	// A device credential must never be exchangeable for an API session.
	asUser(t, "", tests.ApiScenario{
		Name:   "a device credential cannot sign in to the API",
		Method: http.MethodPost,
		URL:    "/api/collections/koreader_accounts/auth-with-password",
		Body: strings.NewReader(`{"identity":"` + testutil.KoUsernameA +
			`","password":"` + testutil.Md5Hex(testutil.KoPasswordA) + `"}`),
		ExpectedStatus:  http.StatusForbidden,
		ExpectedContent: []string{`"data":{}`},
	})
}

func TestAnalyticsCollectionsAreReadOnlyAndScoped(t *testing.T) {
	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "reading days cannot be written by a user",
		Method:          http.MethodPost,
		URL:             "/api/collections/reading_days/records",
		Body:            strings.NewReader(`{"owner":"` + testutil.IdUserA + `","date":"2026-03-01","update_count":9000}`),
		ExpectedStatus:  http.StatusForbidden,
		ExpectedContent: []string{`"data":{}`},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "reading days are listable",
		Method:          http.MethodGet,
		URL:             "/api/collections/reading_days/records",
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"totalItems":0`},
	})

	// The per-book rows are derived exactly like the day rows, so they are read
	// only for the same reason: a page count nobody measured is worse than none.
	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "per-book days cannot be written by a user",
		Method:          http.MethodPost,
		URL:             "/api/collections/reading_book_days/records",
		Body:            strings.NewReader(`{"owner":"` + testutil.IdUserA + `","date":"2026-03-01","pages_read":9000}`),
		ExpectedStatus:  http.StatusForbidden,
		ExpectedContent: []string{`"data":{}`},
	})

	asUser(t, "", tests.ApiScenario{
		Name:            "per-book days are invisible to guests",
		Method:          http.MethodGet,
		URL:             "/api/collections/reading_book_days/records",
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"totalItems":0`},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "the analytics queue is invisible to users",
		Method:          http.MethodGet,
		URL:             "/api/collections/analytics_queue/records",
		ExpectedStatus:  http.StatusForbidden,
		ExpectedContent: []string{`"data":{}`},
	})
}

// idCollectionOfB is a shelf belonging to the other account, so that the tests
// have something they are not allowed to touch.
var idCollectionOfB = testutil.PadId("collb")

// withACollectionOfUserB seeds the two user world and gives the second one a
// shelf of its own.
func withACollectionOfUserB(t testing.TB) *tests.TestApp {
	t.Helper()

	app := testutil.SeededApp(t)

	userB, err := app.FindRecordById(schema.CollectionUsers, testutil.IdUserB)
	if err != nil {
		t.Fatalf("failed to load the fixture user: %v", err)
	}
	testutil.CreateBookCollection(t, app, userB, idCollectionOfB, "Bob's shelf")

	return app
}

// A collection is the one thing here that is entirely its owner's own opinion,
// so unlike everything else it is created and deleted through the ordinary
// collection API rather than by the server. That makes the rules the whole of
// the protection.
func TestBookCollectionsAreScopedToTheirOwner(t *testing.T) {
	const url = "/api/collections/" + schema.CollectionBookCollections + "/records"

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "a user makes a shelf of their own",
		Method:          http.MethodPost,
		URL:             url,
		Body:            strings.NewReader(`{"owner":"` + testutil.IdUserA + `","name":"To read"}`),
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"name":"To read"`},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "a user cannot make a shelf for somebody else",
		Method:          http.MethodPost,
		URL:             url,
		Body:            strings.NewReader(`{"owner":"` + testutil.IdUserB + `","name":"Not mine"}`),
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{`"data":{}`},
	})

	asUser(t, "", tests.ApiScenario{
		Name:            "a guest makes nothing",
		Method:          http.MethodPost,
		URL:             url,
		Body:            strings.NewReader(`{"owner":"` + testutil.IdUserA + `","name":"Anonymous"}`),
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{`"data":{}`},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:               "a user lists only their own shelves",
		Method:             http.MethodGet,
		URL:                url,
		TestAppFactory:     withACollectionOfUserB,
		ExpectedStatus:     http.StatusOK,
		ExpectedContent:    []string{`"totalItems":0`},
		NotExpectedContent: []string{`"id":"` + idCollectionOfB + `"`},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "a user cannot read a foreign shelf",
		Method:          http.MethodGet,
		URL:             url + "/" + idCollectionOfB,
		TestAppFactory:  withACollectionOfUserB,
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{`"data":{}`},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "a user cannot rename a foreign shelf",
		Method:          http.MethodPatch,
		URL:             url + "/" + idCollectionOfB,
		Body:            strings.NewReader(`{"name":"stolen"}`),
		TestAppFactory:  withACollectionOfUserB,
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{`"data":{}`},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "a user cannot throw away a foreign shelf",
		Method:          http.MethodDelete,
		URL:             url + "/" + idCollectionOfB,
		TestAppFactory:  withACollectionOfUserB,
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{`"data":{}`},
	})
}

// Two shelves of one name are two answers to the same question, and the index
// that refuses the second one is what lets the browser say so on the field.
func TestOneAccountCannotHaveTwoShelvesOfOneName(t *testing.T) {
	const url = "/api/collections/" + schema.CollectionBookCollections + "/records"

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:           "the same name twice",
		Method:         http.MethodPost,
		URL:            url,
		Body:           strings.NewReader(`{"owner":"` + testutil.IdUserA + `","name":"To read"}`),
		TestAppFactory: withAShelfOfUserA,
		ExpectedStatus: http.StatusBadRequest,
		ExpectedContent: []string{
			`"name"`,
			`validation_not_unique`,
		},
	})

	// The same name on another account is another account's business.
	asUser(t, testutil.IdUserB, tests.ApiScenario{
		Name:            "the same name on another account",
		Method:          http.MethodPost,
		URL:             url,
		Body:            strings.NewReader(`{"owner":"` + testutil.IdUserB + `","name":"To read"}`),
		TestAppFactory:  withAShelfOfUserA,
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"name":"To read"`},
	})
}

// withAShelfOfUserA seeds the two user world with a shelf named "To read"
// belonging to the first account.
func withAShelfOfUserA(t testing.TB) *tests.TestApp {
	t.Helper()

	app := testutil.SeededApp(t)

	userA, err := app.FindRecordById(schema.CollectionUsers, testutil.IdUserA)
	if err != nil {
		t.Fatalf("failed to load the fixture user: %v", err)
	}
	testutil.CreateBookCollection(t, app, userA, "", "To read")

	return app
}
