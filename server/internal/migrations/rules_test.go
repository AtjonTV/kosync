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

	scenario.TestAppFactory = testutil.SeededApp

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

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "the analytics queue is invisible to users",
		Method:          http.MethodGet,
		URL:             "/api/collections/analytics_queue/records",
		ExpectedStatus:  http.StatusForbidden,
		ExpectedContent: []string{`"data":{}`},
	})
}
